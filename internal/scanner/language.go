package scanner

import (
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/go-enry/go-enry/v2"
	"github.com/petrarca/tech-stack-analyzer/internal/config"
)

// LanguageResult holds the detected language and optional type override from reclassification.
type LanguageResult struct {
	Language     string // Detected or overridden language label
	TypeOverride string // If non-empty, overrides enry.GetLanguageType() — one of: programming, data, markup, prose
}

// LanguageDetector handles language detection using go-enry (GitHub Linguist)
// with optional reclassification rules that override detection for specific file patterns.
type LanguageDetector struct {
	rules    []config.ReclassifyRule
	scanRoot string // scan root for resolving relative paths in glob matching
}

// NewLanguageDetector creates a language detector.
// Pass non-nil rules and scanRoot to enable reclassification; both default gracefully to zero values.
func NewLanguageDetector(rules []config.ReclassifyRule, scanRoot string) *LanguageDetector {
	return &LanguageDetector{
		rules:    rules,
		scanRoot: scanRoot,
	}
}

// DetectLanguage detects the programming language from a filename and optional content.
// If a reclassify rule matches, its language override is applied instead of go-enry detection.
// Returns the detected language or empty string if not detected.
func (d *LanguageDetector) DetectLanguage(filename string, content []byte) string {
	result := d.DetectLanguageWithType(filename, content)
	return result.Language
}

// DetectLanguageWithType detects the language and returns both the language label
// and an optional type override from reclassification rules.
func (d *LanguageDetector) DetectLanguageWithType(filename string, content []byte) LanguageResult {
	// Check reclassify rules first (first match wins)
	if len(d.rules) > 0 {
		relPath := d.relativePath(filename)
		for _, rule := range d.rules {
			if matched, _ := doublestar.Match(rule.Match, relPath); matched {
				// Type-only override: return empty language so it does not pollute
				// the languages map or codeByLanguage stats. TypeOverride still
				// reaches ProcessFile to place the file in the correct type bucket.
				return LanguageResult{
					Language:     rule.Language, // empty string when type-only
					TypeOverride: rule.Type,
				}
			}
		}
	}

	// No reclassify match — fall through to go-enry
	return LanguageResult{
		Language: d.detectWithEnry(filename, content),
	}
}

// detectWithEnry runs the standard go-enry detection pipeline.
func (d *LanguageDetector) detectWithEnry(filename string, content []byte) string {
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

// relativePath converts an absolute file path to a path relative to the scan root,
// using forward slashes for consistent glob matching.
func (d *LanguageDetector) relativePath(filename string) string {
	if d.scanRoot == "" {
		return filepath.ToSlash(filename)
	}
	rel, err := filepath.Rel(d.scanRoot, filename)
	if err != nil {
		return filepath.ToSlash(filename)
	}
	return filepath.ToSlash(strings.TrimPrefix(rel, "./"))
}

// IsLanguageFile checks if a file should be considered for language detection.
// Excludes binary files, images, and other non-source files.
func (d *LanguageDetector) IsLanguageFile(filename string) bool {
	// Use go-enry's built-in checks
	return !enry.IsVendor(filename) &&
		!enry.IsBinary([]byte(filename)) &&
		!enry.IsDocumentation(filename) &&
		!enry.IsConfiguration(filename)
}

// GetLanguageType returns the type of language (programming, markup, data, prose).
func (d *LanguageDetector) GetLanguageType(language string) enry.Type {
	return enry.GetLanguageType(language)
}
