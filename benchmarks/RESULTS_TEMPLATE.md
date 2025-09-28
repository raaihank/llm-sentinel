# Benchmark Results Template

Use this template to update benchmark results in README.md

## Table Format

```markdown
| Benchmark | Samples | Threshold | Balanced Accuracy | Precision | Recall | Mean Latency | P95 Latency | Notes |
|-----------|---------|-----------|-------------------|-----------|--------|--------------|-------------|-------|
| **Gandalf (English)** | 111 injections<br/>111 benign | 0.70 | **73.9%** | **100.0%** | 47.7% | 14.6ms | 19.3ms | Zero false positives<br/>Latency: blocked only |
```

## How to Update

1. **Run benchmark**: `python benchmarks/prompt_injection_gandalf.py`
2. **Extract metrics** from output:
   - Balanced Accuracy (PINT Score)
   - Precision (True Negatives / (True Negatives + False Positives))
   - Recall (True Positives / (True Positives + False Negatives))
   - Mean Latency (blocked requests only)
   - P95 Latency (blocked requests only)
3. **Update table** in README.md
4. **Add notes** about key characteristics

## Benchmark Types

### Gandalf Dataset
- **Source**: `lakera/gandalf_ignore_instructions` (HuggingFace)
- **Type**: Real prompt injections from Gandalf game
- **Language**: English-only (filtered)
- **Command**: `python benchmarks/prompt_injection_gandalf.py`

### PINT Official
- **Source**: Lakera PINT benchmark dataset (private)
- **Type**: Official prompt injection test suite
- **Command**: `python benchmarks/prompt_injection_pint.py --dataset pint-dataset.yaml`

### Custom Dataset
- **Source**: Your 50k balanced security dataset
- **Type**: Curated prompt injection + benign samples
- **Command**: `python benchmarks/prompt_injection_custom.py` (future)

## Threshold Testing

Test different thresholds and update table:

```bash
# Test threshold 0.60 (higher recall)
# Update config: block_threshold: 0.60
# Run benchmark and add new row

# Test threshold 0.80 (higher precision)  
# Update config: block_threshold: 0.80
# Run benchmark and add new row
```

## Example Updates

```markdown
| **Gandalf (0.60)** | 111 injections<br/>111 benign | 0.60 | **78.2%** | **95.5%** | 61.3% | 15.1ms | 20.8ms | Higher recall<br/>Latency: blocked only |
| **Gandalf (0.80)** | 111 injections<br/>111 benign | 0.80 | **71.1%** | **100.0%** | 42.3% | 14.2ms | 18.9ms | Maximum precision<br/>Latency: blocked only |
```
