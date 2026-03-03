package license

import (
	"fmt"
	"math"

	"github.com/go-enry/go-license-detector/v4/licensedb"
	"github.com/go-enry/go-license-detector/v4/licensedb/filer"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// LicenseDetector handles file-based license detection
type LicenseDetector struct{}

// LicenseMatch represents a detected license with metadata
type LicenseMatch struct {
	License    string
	Confidence float32
	File       string
}

// NewLicenseDetector creates a new license detector
func NewLicenseDetector() *LicenseDetector {
	return &LicenseDetector{}
}

// DetectLicensesInDirectory detects licenses from LICENSE files in a directory
// Returns a list of detected licenses with metadata (confidence > 0.9)
func (d *LicenseDetector) DetectLicensesInDirectory(dirPath string) []LicenseMatch {
	// Create a filer for the directory
	fs, err := filer.FromDirectory(dirPath)
	if err != nil {
		return nil
	}

	// Detect licenses
	matches, err := licensedb.Detect(fs)
	if err != nil {
		return nil
	}

	// Extract license matches with high confidence (> 0.9)
	var licenses []LicenseMatch
	for licenseID, match := range matches {
		if match.Confidence > 0.9 {
			licenses = append(licenses, LicenseMatch{
				License:    licenseID,
				Confidence: match.Confidence,
				File:       match.File,
			})
		}
	}

	return licenses
}

// AddLicensesToPayload detects and adds licenses from the current directory to the payload
func (d *LicenseDetector) AddLicensesToPayload(payload *types.Payload, dirPath string) {
	licenseMatches := d.DetectLicensesInDirectory(dirPath)

	// Add detected licenses to payload (avoid duplicates)
	for _, match := range licenseMatches {
		// Check if license already exists
		exists := false
		for _, existing := range payload.Licenses {
			if existing.LicenseName == match.License {
				exists = true
				break
			}
		}

		if !exists {
			// Create structured License object
			license := types.License{
				LicenseName:   match.License,
				DetectionType: "file_based",
				SourceFile:    match.File,
				Confidence:    math.Round(float64(match.Confidence)*100) / 100,
			}

			payload.Licenses = append(payload.Licenses, license)
			// Add reason to _license category
			payload.AddLicenseReason(fmt.Sprintf("license detected: %s (confidence: %.2f, file: %s)",
				match.License, match.Confidence, match.File))
		}
	}
}
