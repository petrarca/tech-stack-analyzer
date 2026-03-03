// Copyright 2025 Google LLC (adapted from deps.dev)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package semver

import (
	"regexp"
	"strconv"
	"strings"
)

// mavenSystem implements Maven version parsing and canonicalization
// Based on: https://maven.apache.org/pom.html#Dependency_Version_Requirement_Specification
type mavenSystem struct{}

func (s *mavenSystem) Name() string {
	return "Maven"
}

func (s *mavenSystem) Parse(version string) (Version, error) {
	return parseMavenVersion(version)
}

// MavenVersion represents a Maven version with canonicalization support
// Format: [versionRange]major.minor.patch[-qualifier]
type MavenVersion struct {
	original string
	version  string
	isRange  bool
}

// parseMavenVersion parses a Maven version string and canonicalizes it
func parseMavenVersion(version string) (*MavenVersion, error) {
	if version == "" {
		return nil, parseError("Maven", version, "empty version string")
	}

	v := &MavenVersion{
		original: version,
	}

	// Check if it's a version range
	if isMavenVersionRange(version) {
		v.isRange = true
		v.version = canonicalizeMavenRange(version)
	} else {
		v.version = canonicalizeMavenVersion(version)
	}

	return v, nil
}

// Canon returns the canonical string representation of the Maven version
func (v *MavenVersion) Canon(includeEpoch bool) string {
	return v.version
}

// String returns the original version string
func (v *MavenVersion) String() string {
	return v.original
}

// Compare compares this version with another version
// For Maven, this is a simplified comparison focusing on the canonical form
func (v *MavenVersion) Compare(other Version) int {
	o, ok := other.(*MavenVersion)
	if !ok {
		return 0
	}

	// For now, compare canonical strings
	// Full Maven version comparison is complex and could be implemented later
	return strings.Compare(v.version, o.version)
}

// Pre-compiled regex patterns for Maven version parsing
var (
	// Maven version range patterns: [1.0,2.0), (1.0,2.0], [1.0,], (,2.0]
	mavenRangeRegex = regexp.MustCompile(`^[\[\(]([^,\]]*),([^\]\)]*)[\]\)]$`)

	// Maven version qualifier patterns: 1.0.0-RELEASE, 1.0.0.FINAL, 1.0.0-SNAPSHOT
	mavenQualifierRegex = regexp.MustCompile(`^(\d+(?:\.\d+)*)(?:[-.]?(RELEASE|FINAL|SNAPSHOT|GA|BUILD|SP|RC|M\d+|PRE))?$`)

	// Maven version with build number: 1.0.0-20131201.121010-1
	mavenBuildRegex = regexp.MustCompile(`^(\d+(?:\.\d+)*)(?:[-.]?(\d{8}\.\d{6})-(\d+))?$`)
)

// isMavenVersionRange checks if a version string is a Maven version range
func isMavenVersionRange(version string) bool {
	return mavenRangeRegex.MatchString(version)
}

// canonicalizeMavenRange converts Maven version ranges to canonical form
func canonicalizeMavenRange(rangeStr string) string {
	matches := mavenRangeRegex.FindStringSubmatch(rangeStr)
	if len(matches) < 3 {
		return rangeStr
	}

	lower := strings.TrimSpace(matches[1])
	upper := strings.TrimSpace(matches[2])

	// Canonicalize bounds
	if lower != "" {
		lower = canonicalizeMavenVersion(lower)
	}
	if upper != "" {
		upper = canonicalizeMavenVersion(upper)
	}

	// Convert to canonical range format
	if lower == "" && upper == "" {
		return "*"
	}
	if lower == "" {
		return "<=" + upper
	}
	if upper == "" {
		return ">=" + lower
	}

	return lower + "-" + upper
}

// canonicalizeMavenVersion converts Maven versions to canonical form
func canonicalizeMavenVersion(version string) string {
	// Handle build number versions first
	if matches := mavenBuildRegex.FindStringSubmatch(version); len(matches) >= 4 {
		base := matches[1]
		timestamp := matches[2]
		buildNum := matches[3]
		if timestamp != "" && buildNum != "" {
			return base + "-build." + buildNum
		}
		return base
	}

	// Handle standard Maven versions with qualifiers
	if matches := mavenQualifierRegex.FindStringSubmatch(version); len(matches) >= 3 {
		base := matches[1]
		qualifier := matches[2]

		// Normalize common qualifiers
		switch qualifier {
		case "RELEASE", "FINAL", "GA":
			return base // Remove these qualifiers as they don't affect version
		case "SNAPSHOT":
			return base + "-snapshot"
		case "BUILD":
			return base + "-build"
		case "":
			return base
		default:
			// Keep other qualifiers (RC, M1, SP, etc.) in lowercase
			if qualifier != "" {
				return base + "-" + strings.ToLower(qualifier)
			}
			return base
		}
	}

	// Handle simple numeric versions
	if isSimpleNumericVersion(version) {
		return version
	}

	// Return as-is for unknown patterns
	return version
}

// isSimpleNumericVersion checks if a version is a simple numeric version (e.g., "1.0.0")
func isSimpleNumericVersion(version string) bool {
	parts := strings.Split(version, ".")
	for _, part := range parts {
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return len(parts) > 0
}
