package privacy

import (
	"fmt"
	"strings"

	"github.com/yourusername/llm-sentinel/internal/config"
	"github.com/yourusername/llm-sentinel/internal/logger"
	"go.uber.org/zap"
)

// Detector handles PII detection and masking
type Detector struct {
	rules   []DetectionRule
	enabled map[string]bool
	logger  *logger.Logger
	config  config.PrivacyConfig
}

// New creates a new PII detector instance
func New(cfg config.PrivacyConfig, log *logger.Logger) (*Detector, error) {
	detector := &Detector{
		rules:   GetDefaultRules(),
		enabled: make(map[string]bool),
		logger:  log,
		config:  cfg,
	}

	// Configure enabled detectors
	if err := detector.configureDetectors(cfg.Detectors); err != nil {
		return nil, fmt.Errorf("failed to configure detectors: %w", err)
	}

	log.Info("Privacy detector initialized",
		zap.Int("total_rules", len(detector.rules)),
		zap.Int("enabled_rules", detector.countEnabledRules()),
	)

	return detector, nil
}

// configureDetectors enables/disables detectors based on configuration
func (d *Detector) configureDetectors(detectors []string) error {
	// Disable all rules by default
	for _, rule := range d.rules {
		d.enabled[rule.Name] = false
	}

	// Enable specified detectors
	for _, detector := range detectors {
		if detector == "all" {
			// Enable all detectors
			for _, rule := range d.rules {
				d.enabled[rule.Name] = true
			}
			continue
		}

		// Enable specific detector
		found := false
		for _, rule := range d.rules {
			if rule.Name == detector {
				d.enabled[rule.Name] = true
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("unknown detector: %s", detector)
		}
	}

	return nil
}

// ProcessText processes text through all enabled PII detectors
func (d *Detector) ProcessText(text string) ProcessResult {
	if !d.config.Enabled {
		return ProcessResult{
			MaskedText: text,
			Findings:   []Finding{},
			Original:   text,
		}
	}

	maskedText := text
	findings := make([]Finding, 0)

	for _, rule := range d.rules {
		if !d.enabled[rule.Name] {
			continue
		}

		matches := rule.Pattern.FindAllStringSubmatch(maskedText, -1)
		if len(matches) > 0 {
			// Create finding
			finding := Finding{
				EntityType: rule.Name,
				Masked:     rule.Replacement,
				Count:      len(matches),
			}
			findings = append(findings, finding)

			// Apply masking
			maskedText = rule.Pattern.ReplaceAllString(maskedText, rule.Replacement)

			d.logger.Debug("PII detected and masked",
				zap.String("entity_type", rule.Name),
				zap.Int("count", len(matches)),
				zap.String("replacement", rule.Replacement),
			)
		}
	}

	return ProcessResult{
		MaskedText: maskedText,
		Findings:   findings,
		Original:   text,
	}
}

// ProcessHeaders processes HTTP headers for sensitive data
func (d *Detector) ProcessHeaders(headers map[string][]string) map[string][]string {
	return d.ProcessHeadersForContext(headers, false)
}

// ProcessHeadersForContext processes HTTP headers with context about usage
func (d *Detector) ProcessHeadersForContext(headers map[string][]string, forUpstream bool) map[string][]string {
	if !d.config.Enabled || !d.config.HeaderScrubbing.Enabled {
		return headers
	}

	processedHeaders := make(map[string][]string)

	for key, values := range headers {
		if d.isSensitiveHeader(key) {
			// Check if we should preserve upstream auth headers
			if forUpstream && d.config.HeaderScrubbing.PreserveUpstreamAuth && d.isAuthHeader(key) {
				processedHeaders[key] = values
				d.logger.Debug("Auth header preserved for upstream", zap.String("header", key))
			} else {
				processedHeaders[key] = []string{"[REDACTED]"}
				d.logger.Debug("Header scrubbed", zap.String("header", key))
			}
		} else {
			processedHeaders[key] = values
		}
	}

	return processedHeaders
}

// isSensitiveHeader checks if a header should be scrubbed
func (d *Detector) isSensitiveHeader(header string) bool {
	headerLower := strings.ToLower(header)

	for _, sensitiveHeader := range d.config.HeaderScrubbing.Headers {
		if strings.Contains(headerLower, strings.ToLower(sensitiveHeader)) {
			return true
		}
	}

	return false
}

// isAuthHeader checks if a header is used for authentication
func (d *Detector) isAuthHeader(header string) bool {
	headerLower := strings.ToLower(header)
	authHeaders := []string{"authorization", "x-api-key", "x-auth-token", "bearer"}
	
	for _, authHeader := range authHeaders {
		if strings.Contains(headerLower, authHeader) {
			return true
		}
	}
	
	return false
}

// IsAuthHeaderPublic is a public wrapper for isAuthHeader
func (d *Detector) IsAuthHeaderPublic(header string) bool {
	return d.isAuthHeader(header)
}

// countEnabledRules returns the number of enabled detection rules
func (d *Detector) countEnabledRules() int {
	count := 0
	for _, enabled := range d.enabled {
		if enabled {
			count++
		}
	}
	return count
}

// GetEnabledRules returns a list of enabled rule names
func (d *Detector) GetEnabledRules() []string {
	var enabled []string
	for ruleName, isEnabled := range d.enabled {
		if isEnabled {
			enabled = append(enabled, ruleName)
		}
	}
	return enabled
}

// EnableRule enables a specific detection rule
func (d *Detector) EnableRule(ruleName string) error {
	for _, rule := range d.rules {
		if rule.Name == ruleName {
			d.enabled[ruleName] = true
			d.logger.Info("Detection rule enabled", zap.String("rule", ruleName))
			return nil
		}
	}
	return fmt.Errorf("unknown rule: %s", ruleName)
}

// DisableRule disables a specific detection rule
func (d *Detector) DisableRule(ruleName string) error {
	if _, exists := d.enabled[ruleName]; !exists {
		return fmt.Errorf("unknown rule: %s", ruleName)
	}

	d.enabled[ruleName] = false
	d.logger.Info("Detection rule disabled", zap.String("rule", ruleName))
	return nil
}
