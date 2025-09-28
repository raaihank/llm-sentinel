//go:build onnx
// +build onnx

package embeddings

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
	"go.uber.org/zap"
)

// OnnxBackend implements TransformerBackend using ONNX Runtime (via yalue/onnxruntime_go).
type OnnxBackend struct {
	session    *ort.DynamicAdvancedSession
	inputNames []string
	outputName string
	logger     *zap.Logger
	ready      bool
	mu         sync.RWMutex
}

// NewTransformerBackend initializes the ONNX Runtime backend. Requires build tag 'onnx'.
func NewTransformerBackend(logger *zap.Logger, modelPath string, maxLength int) TransformerBackend {
	// Allow user to provide shared library path via environment variable.
	if shlib := os.Getenv("ONNXRUNTIME_SHARED_LIB"); shlib != "" {
		ort.SetSharedLibraryPath(shlib)
	} else if shlib := os.Getenv("ORT_SHLIB"); shlib != "" {
		ort.SetSharedLibraryPath(shlib)
	}

	if err := ort.InitializeEnvironment(); err != nil {
		logger.Error("ONNX Runtime environment init failed", zap.Error(err))
		return nil
	}

	// Inspect model IO to determine names
	inputsInfo, outputsInfo, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		logger.Error("Failed to inspect ONNX model IO", zap.Error(err), zap.String("model", modelPath))
		return nil
	}

	// Prefer common transformer inputs order
	preferredInputs := []string{"input_ids", "attention_mask", "token_type_ids"}
	available := map[string]bool{}
	for _, ii := range inputsInfo {
		available[strings.ToLower(ii.Name)] = true
	}
	var inputNames []string
	for _, name := range preferredInputs {
		if available[name] {
			inputNames = append(inputNames, name)
		}
	}
	// If no preferred names matched, fall back to model-declared order
	if len(inputNames) == 0 && len(inputsInfo) > 0 {
		// Keep stable order by name for determinism
		sorted := make([]string, 0, len(inputsInfo))
		for _, ii := range inputsInfo {
			sorted = append(sorted, ii.Name)
		}
		sort.Strings(sorted)
		inputNames = sorted
	}

	// Choose the first output by default
	if len(outputsInfo) == 0 {
		logger.Error("ONNX model reports no outputs", zap.String("model", modelPath))
		return nil
	}
	outputName := outputsInfo[0].Name

	sess, err := ort.NewDynamicAdvancedSession(modelPath, inputNames, []string{outputName}, nil)
	if err != nil {
		logger.Error("ONNX Runtime session creation failed", zap.Error(err), zap.String("model", modelPath))
		return nil
	}

	logger.Info("ONNX Runtime backend ready", zap.String("model", modelPath), zap.Strings("inputs", inputNames), zap.String("output", outputName))
	return &OnnxBackend{session: sess, inputNames: inputNames, outputName: outputName, logger: logger, ready: true}
}

// IsReady reports whether the backend is initialized.
func (b *OnnxBackend) IsReady() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.ready && b.session != nil
}

// Close releases session and environment resources.
func (b *OnnxBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.session != nil {
		b.session.Destroy()
		b.session = nil
	}
	ort.DestroyEnvironment()
	b.ready = false
	return nil
}

// EmbedBatch runs inference for the batch and returns 384-d embeddings.
func (b *OnnxBackend) EmbedBatch(ctx context.Context, tokensBatch []*TokenizedInput) ([][]float32, error) {
	if !b.IsReady() {
		return nil, fmt.Errorf("onnx backend not ready")
	}

	batch := len(tokensBatch)
	if batch == 0 {
		return [][]float32{}, nil
	}
	seqLen := len(tokensBatch[0].InputIDs)

	// Prepare inputs as int64 (common for BERT-like models)
	inputIDs := make([]int64, 0, batch*seqLen)
	attention := make([]int64, 0, batch*seqLen)
	tokenTypes := make([]int64, 0, batch*seqLen)
	for _, t := range tokensBatch {
		// Respect context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		for i := 0; i < seqLen; i++ {
			inputIDs = append(inputIDs, int64(t.InputIDs[i]))
			attention = append(attention, int64(t.AttentionMask[i]))
			tokenTypes = append(tokenTypes, int64(t.TokenTypeIDs[i]))
		}
	}

	// Create tensors
	shape := ort.NewShape(int64(batch), int64(seqLen))
	idsTensor, err := ort.NewTensor[int64](shape, inputIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to create input_ids tensor: %w", err)
	}
	defer idsTensor.Destroy()
	maskTensor, err := ort.NewTensor[int64](shape, attention)
	if err != nil {
		return nil, fmt.Errorf("failed to create attention_mask tensor: %w", err)
	}
	defer maskTensor.Destroy()
	var typeTensor *ort.Tensor[int64]
	if len(tokenTypes) > 0 {
		var terr error
		typeTensor, terr = ort.NewTensor[int64](shape, tokenTypes)
		if terr != nil {
			return nil, fmt.Errorf("failed to create token_type_ids tensor: %w", terr)
		}
		defer typeTensor.Destroy()
	}

	// Prepare inputs in the order declared with robust name heuristics
	inputs := make([]ort.Value, 0, len(b.inputNames))
	idUsed, maskUsed, typeUsed := false, false, false
	for _, rawName := range b.inputNames {
		name := strings.ToLower(rawName)
		switch {
		case name == "input_ids" || name == "input" || strings.Contains(name, "input_ids") || strings.Contains(name, "ids"):
			inputs = append(inputs, idsTensor)
			idUsed = true
		case name == "attention_mask" || strings.Contains(name, "attention") || strings.Contains(name, "mask"):
			inputs = append(inputs, maskTensor)
			maskUsed = true
		case name == "token_type_ids" || strings.Contains(name, "token_type") || strings.Contains(name, "segment"):
			if typeTensor != nil {
				inputs = append(inputs, typeTensor)
			} else {
				empty, terr := ort.NewTensor[int64](shape, make([]int64, batch*seqLen))
				if terr != nil {
					return nil, fmt.Errorf("failed to create placeholder token_type_ids: %w", terr)
				}
				defer empty.Destroy()
				inputs = append(inputs, empty)
			}
			typeUsed = true
		default:
			// Fallback by position: first unseen -> ids, then mask, then type
			if !idUsed {
				inputs = append(inputs, idsTensor)
				idUsed = true
				continue
			}
			if !maskUsed {
				inputs = append(inputs, maskTensor)
				maskUsed = true
				continue
			}
			if !typeUsed {
				if typeTensor != nil {
					inputs = append(inputs, typeTensor)
				} else {
					empty, terr := ort.NewTensor[int64](shape, make([]int64, batch*seqLen))
					if terr != nil {
						return nil, fmt.Errorf("failed to create placeholder token_type_ids: %w", terr)
					}
					defer empty.Destroy()
					inputs = append(inputs, empty)
				}
				typeUsed = true
				continue
			}
			// If all used, default to ids
			inputs = append(inputs, idsTensor)
		}
	}

	// One output; let ORT allocate it
	outputs := make([]ort.Value, 1)
	if err := b.session.Run(inputs, outputs); err != nil {
		return nil, fmt.Errorf("onnx run failed: %w", err)
	}
	if len(outputs) == 0 || outputs[0] == nil {
		return nil, fmt.Errorf("onnx returned no outputs")
	}
	defer func() {
		if outputs[0] != nil {
			_ = outputs[0].Destroy()
		}
	}()

	// Expect first output as pooled embedding or last_hidden_state
	outTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected output type (want float32 tensor)")
	}
	data := outTensor.GetData()
	outShape := outTensor.GetShape()
	res := make([][]float32, batch)
	if len(outShape) == 2 {
		// [batch, dims]
		dims := int(outShape[1])
		if dims != EmbeddingDimensions {
			return nil, fmt.Errorf("unexpected output dims %d (want %d)", dims, EmbeddingDimensions)
		}
		if len(data) != batch*dims {
			return nil, fmt.Errorf("unexpected flat data length %d for shape %v", len(data), outShape)
		}
		for i := 0; i < batch; i++ {
			start := i * dims
			end := start + dims
			res[i] = make([]float32, EmbeddingDimensions)
			copy(res[i], data[start:end])
		}
	} else if len(outShape) == 3 {
		// [batch, seq, dims] -> mean pool over seq
		seq := int(outShape[1])
		dims := int(outShape[2])
		if dims != EmbeddingDimensions {
			return nil, fmt.Errorf("unexpected hidden dims %d (want %d)", dims, EmbeddingDimensions)
		}
		if len(data) != batch*seq*dims {
			return nil, fmt.Errorf("unexpected flat data length %d for shape %v", len(data), outShape)
		}
		for b := 0; b < batch; b++ {
			pooled := make([]float32, EmbeddingDimensions)
			for s := 0; s < seq; s++ {
				offset := (b*seq + s) * dims
				for d := 0; d < dims; d++ {
					pooled[d] += data[offset+d]
				}
			}
			inv := 1.0 / float32(seq)
			for d := 0; d < dims; d++ {
				pooled[d] *= inv
			}
			res[b] = pooled
		}
	} else {
		return nil, fmt.Errorf("unsupported output shape %v", outShape)
	}

	return res, nil
}
