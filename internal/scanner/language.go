package scanner

import (
	"path/filepath"

	"github.com/go-enry/go-enry/v2"
)

// LanguageDetector handles language detection using go-enry (GitHub Linguist)
type LanguageDetector struct{}

// NewLanguageDetector creates a new language detector
func NewLanguageDetector() *LanguageDetector {
	return &LanguageDetector{}
}

// DetectLanguage detects the programming language from a filename and optional content
// Uses content analysis for ambiguous extensions (e.g., .md could be Markdown or GCC Machine Description)
// Returns the detected language or empty string if not detected
func (d *LanguageDetector) DetectLanguage(filename string, content []byte) string {
	// Try detection by extension first (fast path)
	lang, safe := enry.GetLanguageByExtension(filename)

	// If not safe (ambiguous extension), use content analysis for better accuracy
	if !safe && lang != "" && len(content) > 0 {
		lang = enry.GetLanguage(filepath.Base(filename), content)
	}

	// If no result from extension, try by filename (handles special files like Makefile, Dockerfile)
	if lang == "" {
		lang, _ = enry.GetLanguageByFilename(filename)
	}

	return lang
}

// IsLanguageFile checks if a file should be considered for language detection
// Excludes binary files, images, and other non-source files
func (d *LanguageDetector) IsLanguageFile(filename string) bool {
	// Use go-enry's built-in checks
	return !enry.IsVendor(filename) &&
		!enry.IsBinary([]byte(filename)) &&
		!enry.IsDocumentation(filename) &&
		!enry.IsConfiguration(filename)
}

// GetLanguageType returns the type of language (programming, markup, data, prose)
func (d *LanguageDetector) GetLanguageType(language string) enry.Type {
	return enry.GetLanguageType(language)
}
