package privacy

import "regexp"

// DetectionRule represents a single PII detection rule
type DetectionRule struct {
	Name        string
	Pattern     *regexp.Regexp
	Replacement string
	Enabled     bool
}

// Finding represents a detection result
type Finding struct {
	EntityType string `json:"entityType"`
	Masked     string `json:"masked"`
	Count      int    `json:"count"`
	Positions  []int  `json:"positions,omitempty"`
}

// ProcessResult contains the result of processing text through the detector
type ProcessResult struct {
	MaskedText string    `json:"maskedText"`
	Findings   []Finding `json:"findings"`
	Original   string    `json:"-"` // Never serialize original text
}
