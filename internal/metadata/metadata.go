package metadata

import (
	"path/filepath"
	"time"
)

// ScanMetadata contains information about the scan execution
type ScanMetadata struct {
	Format         string                 `json:"format"` // Output format: "full" or "aggregated"
	Timestamp      string                 `json:"timestamp"`
	ScanPath       string                 `json:"scan_path"`
	SpecVersion    string                 `json:"specVersion"` // Output format specification version
	DurationMs     int64                  `json:"duration_ms,omitempty"`
	FileCount      int                    `json:"file_count,omitempty"`
	ComponentCount int                    `json:"component_count,omitempty"`
	LanguageCount  int                    `json:"language_count,omitempty"` // Number of distinct programming languages
	TechCount      int                    `json:"tech_count,omitempty"`     // Number of primary technologies
	TechsCount     int                    `json:"techs_count,omitempty"`    // Number of all detected technologies
	Properties     map[string]interface{} `json:"properties,omitempty"`
}

// NewScanMetadata creates a new scan metadata instance
func NewScanMetadata(scanPath string, version string) *ScanMetadata {
	absPath, _ := filepath.Abs(scanPath)

	return &ScanMetadata{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		ScanPath:    absPath,
		SpecVersion: version,
	}
}

// SetDuration sets the scan duration in milliseconds
func (m *ScanMetadata) SetDuration(duration time.Duration) {
	m.DurationMs = duration.Milliseconds()
}

// SetFileCounts sets the file and component counts
func (m *ScanMetadata) SetFileCounts(fileCount, componentCount int) {
	m.FileCount = fileCount
	m.ComponentCount = componentCount
}

// SetLanguageCount sets the number of distinct programming languages
func (m *ScanMetadata) SetLanguageCount(languageCount int) {
	m.LanguageCount = languageCount
}

// SetTechCounts sets the primary and total technology counts
func (m *ScanMetadata) SetTechCounts(techCount, techsCount int) {
	m.TechCount = techCount
	m.TechsCount = techsCount
}

// SetProperties sets custom properties from configuration
func (m *ScanMetadata) SetProperties(properties map[string]interface{}) {
	if len(properties) > 0 {
		m.Properties = properties
	}
}

// SetFormat sets the output format type
func (m *ScanMetadata) SetFormat(format string) {
	m.Format = format
}
