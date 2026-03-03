package license

import (
	"encoding/json"
	"strings"
)

// Normalizer handles SPDX-compliant license normalization
type Normalizer struct {
	mappings map[string]string
}

// NewNormalizer creates a new license normalizer with comprehensive SPDX mappings
func NewNormalizer() *Normalizer {
	return &Normalizer{
		mappings: map[string]string{
			// MIT variations
			"mit":           "MIT",
			"MIT":           "MIT",
			"mit license":   "MIT",
			"expat":         "MIT",
			"expat license": "MIT",

			// Apache variations
			"apache":             "Apache-2.0",
			"apache-2.0":         "Apache-2.0",
			"apache 2.0":         "Apache-2.0",
			"apache2":            "Apache-2.0",
			"apache-2":           "Apache-2.0",
			"apache license 2.0": "Apache-2.0",
			"Apache-2.0":         "Apache-2.0",

			// GPL variations
			"gpl":     "GPL-3.0",
			"gpl-3.0": "GPL-3.0",
			"gplv3":   "GPL-3.0",
			"gpl v3":  "GPL-3.0",
			"GPL-3.0": "GPL-3.0",
			"gpl-2.0": "GPL-2.0",
			"gplv2":   "GPL-2.0",
			"gpl v2":  "GPL-2.0",
			"GPL-2.0": "GPL-2.0",

			// LGPL variations
			"lgpl":      "LGPL-3.0",
			"lgpl-3.0":  "LGPL-3.0",
			"lgplv3":    "LGPL-3.0",
			"lgpl v3":   "LGPL-3.0",
			"LGPL-3.0":  "LGPL-3.0",
			"lgpl-2.1":  "LGPL-2.1",
			"lgplv2.1":  "LGPL-2.1",
			"lgpl v2.1": "LGPL-2.1",
			"LGPL-2.1":  "LGPL-2.1",

			// BSD variations
			"bsd":          "BSD-3-Clause",
			"bsd-3-clause": "BSD-3-Clause",
			"bsd 3 clause": "BSD-3-Clause",
			"bsd-2-clause": "BSD-2-Clause",
			"bsd 2 clause": "BSD-2-Clause",
			"bsd-4-clause": "BSD-4-Clause",
			"bsd 4 clause": "BSD-4-Clause",
			"BSD-3-Clause": "BSD-3-Clause",
			"BSD-2-Clause": "BSD-2-Clause",
			"BSD-4-Clause": "BSD-4-Clause",

			// AGPL variations
			"agpl":                                   "AGPL-3.0-only",
			"agpl-3.0":                               "AGPL-3.0-only",
			"agplv3":                                 "AGPL-3.0-only",
			"agpl v3":                                "AGPL-3.0-only",
			"AGPL-3.0":                               "AGPL-3.0-only",
			"AGPL-3.0-only":                          "AGPL-3.0-only",
			"AGPL-3.0-or-later":                      "AGPL-3.0-or-later",
			"agpl-3.0-only":                          "AGPL-3.0-only",
			"agpl-3.0-or-later":                      "AGPL-3.0-or-later",
			"GNU Affero General Public License v3.0": "AGPL-3.0-only",

			// GPL -only/-or-later variants (SPDX 3.x preferred forms)
			"GPL-2.0-only":     "GPL-2.0-only",
			"GPL-2.0-or-later": "GPL-2.0-or-later",
			"GPL-3.0-only":     "GPL-3.0-only",
			"GPL-3.0-or-later": "GPL-3.0-or-later",
			"gpl-2.0-only":     "GPL-2.0-only",
			"gpl-2.0-or-later": "GPL-2.0-or-later",
			"gpl-3.0-only":     "GPL-3.0-only",
			"gpl-3.0-or-later": "GPL-3.0-or-later",

			// LGPL -only/-or-later variants
			"LGPL-2.1-only":     "LGPL-2.1-only",
			"LGPL-2.1-or-later": "LGPL-2.1-or-later",
			"LGPL-3.0-only":     "LGPL-3.0-only",
			"LGPL-3.0-or-later": "LGPL-3.0-or-later",
			"lgpl-2.1-only":     "LGPL-2.1-only",
			"lgpl-2.1-or-later": "LGPL-2.1-or-later",
			"lgpl-3.0-only":     "LGPL-3.0-only",
			"lgpl-3.0-or-later": "LGPL-3.0-or-later",

			// Other common licenses
			"isc":                    "ISC",
			"ISC":                    "ISC",
			"unlicense":              "Unlicense",
			"Unlicense":              "Unlicense",
			"cc0":                    "CC0-1.0",
			"cc0-1.0":                "CC0-1.0",
			"CC0-1.0":                "CC0-1.0",
			"mpl":                    "MPL-2.0",
			"mpl-2.0":                "MPL-2.0",
			"mozilla public license": "MPL-2.0",
			"MPL-2.0":                "MPL-2.0",

			// Eclipse Public License
			"epl":                        "EPL-2.0",
			"epl-1.0":                    "EPL-1.0",
			"epl-2.0":                    "EPL-2.0",
			"EPL-1.0":                    "EPL-1.0",
			"EPL-2.0":                    "EPL-2.0",
			"eclipse public license":     "EPL-2.0",
			"eclipse public license 1.0": "EPL-1.0",
			"eclipse public license 2.0": "EPL-2.0",

			// CDDL
			"cddl":     "CDDL-1.0",
			"cddl-1.0": "CDDL-1.0",
			"cddl-1.1": "CDDL-1.1",
			"CDDL-1.0": "CDDL-1.0",
			"CDDL-1.1": "CDDL-1.1",

			// Artistic License
			"artistic":     "Artistic-2.0",
			"artistic-2.0": "Artistic-2.0",
			"Artistic-2.0": "Artistic-2.0",

			// Zlib
			"zlib":        "Zlib",
			"Zlib":        "Zlib",
			"zlib/libpng": "Zlib",

			// 0BSD (Zero-Clause BSD)
			"0bsd": "0BSD",
			"0BSD": "0BSD",

			// WTFPL
			"wtfpl": "WTFPL",
			"WTFPL": "WTFPL",

			// Boost Software License
			"bsl":                    "BSL-1.0",
			"bsl-1.0":                "BSL-1.0",
			"BSL-1.0":                "BSL-1.0",
			"boost":                  "BSL-1.0",
			"boost software license": "BSL-1.0",

			// Academic Free License
			"afl":     "AFL-3.0",
			"afl-3.0": "AFL-3.0",
			"AFL-3.0": "AFL-3.0",

			// European Union Public License
			"eupl":     "EUPL-1.2",
			"eupl-1.2": "EUPL-1.2",
			"EUPL-1.2": "EUPL-1.2",

			// PostgreSQL License
			"postgresql": "PostgreSQL",
			"PostgreSQL": "PostgreSQL",

			// Commercial/Proprietary
			"proprietary":   "Proprietary",
			"commercial":    "Proprietary",
			"closed source": "Proprietary",
		},
	}
}

// Normalize normalizes a license string to SPDX standard format
func (n *Normalizer) Normalize(license string) string {
	if license == "" {
		return ""
	}

	// Clean up the license string
	license = strings.TrimSpace(license)
	license = strings.Trim(license, `"'`)

	// Convert to lowercase for matching
	lowerLicense := strings.ToLower(license)

	// Check exact match first
	if spdx, exists := n.mappings[license]; exists {
		return spdx
	}

	// Check lowercase match
	if spdx, exists := n.mappings[lowerLicense]; exists {
		return spdx
	}

	// Return as-is if no mapping found (might already be SPDX)
	return license
}

// ParseTOMLLicense parses TOML license field and extracts the license text
// Handles formats like: license = "MIT", license = {text = "MIT"}
func (n *Normalizer) ParseTOMLLicense(licenseStr string) string {
	if licenseStr == "" {
		return ""
	}

	licenseStr = strings.TrimSpace(licenseStr)

	// Handle simple string format: license = "MIT"
	if strings.HasPrefix(licenseStr, `"`) && strings.HasSuffix(licenseStr, `"`) {
		license := strings.Trim(licenseStr, `"`)
		return n.Normalize(license)
	}

	// Handle single quotes: license = 'MIT'
	if strings.HasPrefix(licenseStr, `'`) && strings.HasSuffix(licenseStr, `'`) {
		license := strings.Trim(licenseStr, `'`)
		return n.Normalize(license)
	}

	// Handle TOML object format: license = {text = "MIT"}
	if strings.Contains(licenseStr, "{") && strings.Contains(licenseStr, "}") {
		// Try to parse as JSON-like structure
		var licenseObj map[string]interface{}
		if err := json.Unmarshal([]byte(strings.ReplaceAll(licenseStr, "=", ":")), &licenseObj); err == nil {
			if text, exists := licenseObj["text"]; exists {
				if textStr, ok := text.(string); ok {
					return n.Normalize(textStr)
				}
			}
		}

		// Fallback: extract text from pattern
		if strings.Contains(licenseStr, "text") {
			parts := strings.Split(licenseStr, "text")
			if len(parts) > 1 {
				textPart := strings.Split(parts[1], "=")
				if len(textPart) > 1 {
					license := strings.TrimSpace(textPart[1])
					license = strings.Trim(license, `"',}`)
					return n.Normalize(license)
				}
			}
		}
	}

	// Handle any remaining quotes and normalize
	license := strings.Trim(licenseStr, `"'`)
	return n.Normalize(license)
}

// ParseLicenseExpression parses license expressions like "MIT OR Apache-2.0"
// Returns individual licenses as a slice
func (n *Normalizer) ParseLicenseExpression(expr string) []string {
	if expr == "" {
		return nil
	}

	expr = strings.TrimSpace(expr)

	// Split by common operators
	operators := []string{" OR ", " AND ", " or ", " and ", "||", "&&"}

	var licenses []string
	current := expr

	// Try each operator
	for _, op := range operators {
		if strings.Contains(current, op) {
			parts := strings.Split(current, op)
			for _, part := range parts {
				normalized := n.Normalize(strings.TrimSpace(part))
				if normalized != "" {
					licenses = append(licenses, normalized)
				}
			}
			return licenses
		}
	}

	// Check if it's just an operator without any license
	isOperator := false
	operatorTokens := []string{"OR", "AND", "||", "&&"}
	for _, token := range operatorTokens {
		if strings.ToUpper(expr) == token {
			isOperator = true
			break
		}
	}
	if isOperator {
		return nil
	}

	// Single license
	normalized := n.Normalize(expr)
	if normalized != "" {
		licenses = append(licenses, normalized)
	}

	return licenses
}

// NormalizeMultiple normalizes multiple licenses and removes duplicates
func (n *Normalizer) NormalizeMultiple(licenses []string) []string {
	if len(licenses) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var normalized []string

	for _, license := range licenses {
		// Try parsing as TOML license first
		normalizedLicense := n.ParseTOMLLicense(license)
		if normalizedLicense == "" {
			// Fallback to regular normalization
			normalizedLicense = n.Normalize(license)
		}

		if normalizedLicense != "" && !seen[normalizedLicense] {
			seen[normalizedLicense] = true
			normalized = append(normalized, normalizedLicense)
		}
	}

	return normalized
}

// IsSPDXValid checks if a license string appears to be a valid SPDX identifier
func (n *Normalizer) IsSPDXValid(license string) bool {
	if license == "" {
		return false
	}

	normalized := n.Normalize(license)

	// Check if it's in our mappings (SPDX compatible)
	for _, spdx := range n.mappings {
		if spdx == normalized {
			return true
		}
	}

	// Additional check for common SPDX patterns that we might not have in mappings
	// but are clearly SPDX identifiers
	if strings.Contains(normalized, "-") &&
		(strings.HasSuffix(normalized, ".0") ||
			strings.Contains(normalized, "-Clause") ||
			strings.Contains(normalized, "-Only") ||
			strings.Contains(normalized, "-Or-Later")) {
		// But exclude obviously made-up ones
		if !strings.Contains(strings.ToLower(normalized), "unknown") &&
			!strings.Contains(strings.ToLower(normalized), "custom") {
			return true
		}
	}

	return false
}

// GetSupportedLicenses returns all supported SPDX license mappings
func (n *Normalizer) GetSupportedLicenses() map[string]string {
	result := make(map[string]string)
	for k, v := range n.mappings {
		result[k] = v
	}
	return result
}
