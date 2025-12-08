package util

import (
	"fmt"
	"strings"
)

// ValidOutputFormats defines the supported output formats
var ValidOutputFormats = map[string]bool{
	"text": true,
	"json": true,
	"yaml": true,
}

// ValidateOutputFormat checks if the given format is valid
func ValidateOutputFormat(format string) error {
	if !ValidOutputFormats[strings.ToLower(format)] {
		return fmt.Errorf("invalid format: %s. Valid formats are: text, json, yaml", format)
	}
	return nil
}

// GetValidFormats returns a list of valid output formats
func GetValidFormats() []string {
	formats := make([]string, 0, len(ValidOutputFormats))
	for format := range ValidOutputFormats {
		formats = append(formats, format)
	}
	return formats
}

// NormalizeFormat normalizes the format string to lowercase
func NormalizeFormat(format string) string {
	return strings.ToLower(format)
}
