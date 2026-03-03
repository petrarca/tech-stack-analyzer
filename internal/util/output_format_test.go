package util

import (
	"testing"
)

func TestValidateOutputFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"valid text", "text", false},
		{"valid json", "json", false},
		{"valid yaml", "yaml", false},
		{"valid uppercase", "JSON", false},
		{"valid mixed case", "Yaml", false},
		{"invalid format", "xml", true},
		{"empty format", "", true},
		{"invalid format", "csv", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOutputFormat(tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOutputFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeFormat(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		expected string
	}{
		{"lowercase", "text", "text"},
		{"uppercase", "JSON", "json"},
		{"mixed case", "Yaml", "yaml"},
		{"already lowercase", "yaml", "yaml"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeFormat(tt.format)
			if result != tt.expected {
				t.Errorf("NormalizeFormat() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetValidFormats(t *testing.T) {
	formats := GetValidFormats()
	expected := []string{"text", "json", "yaml"}

	if len(formats) != len(expected) {
		t.Errorf("GetValidFormats() returned %d formats, expected %d", len(formats), len(expected))
	}

	// Check that all expected formats are present
	formatMap := make(map[string]bool)
	for _, format := range formats {
		formatMap[format] = true
	}

	for _, expectedFormat := range expected {
		if !formatMap[expectedFormat] {
			t.Errorf("GetValidFormats() missing expected format: %s", expectedFormat)
		}
	}
}
