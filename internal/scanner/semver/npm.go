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
	"fmt"
	"strconv"
	"strings"
)

// npmSystem implements npm/node-semver version parsing
// Based on: https://github.com/npm/node-semver
type npmSystem struct{}

func (s *npmSystem) Name() string {
	return "npm"
}

func (s *npmSystem) Parse(version string) (Version, error) {
	return parseNPMVersion(version)
}

// NPMVersion represents an npm semver version
// Format: [v]major.minor.patch[-prerelease][+build]
type NPMVersion struct {
	original   string
	major      int
	minor      int
	patch      int
	prerelease []string // e.g., ["alpha", "1"]
	build      []string // e.g., ["001", "20130313144700"]
}

// parseNPMVersion parses an npm semver string
func parseNPMVersion(version string) (*NPMVersion, error) {
	if version == "" {
		return nil, parseError("npm", version, "empty version string")
	}

	v := &NPMVersion{original: version}
	s := strings.TrimSpace(version)

	// Remove leading 'v' or '='
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	s = strings.TrimPrefix(s, "=")

	// Parse build metadata (e.g., "+build.123")
	if idx := strings.IndexByte(s, '+'); idx >= 0 {
		buildStr := s[idx+1:]
		if buildStr != "" {
			v.build = strings.Split(buildStr, ".")
		}
		s = s[:idx]
	}

	// Parse prerelease (e.g., "-alpha.1")
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		prereleaseStr := s[idx+1:]
		if prereleaseStr != "" {
			v.prerelease = strings.Split(prereleaseStr, ".")
		}
		s = s[:idx]
	}

	// Parse major.minor.patch
	parts := strings.Split(s, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return nil, parseError("npm", version, "invalid version format")
	}

	// Parse major
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, parseError("npm", version, fmt.Sprintf("invalid major version: %s", parts[0]))
	}
	v.major = major

	// Parse minor (default to 0)
	if len(parts) >= 2 {
		minor, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, parseError("npm", version, fmt.Sprintf("invalid minor version: %s", parts[1]))
		}
		v.minor = minor
	}

	// Parse patch (default to 0)
	if len(parts) >= 3 {
		patch, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, parseError("npm", version, fmt.Sprintf("invalid patch version: %s", parts[2]))
		}
		v.patch = patch
	}

	return v, nil
}

// Canon returns the canonical string representation of the version
func (v *NPMVersion) Canon(includeEpoch bool) string {
	var b strings.Builder

	// Major.minor.patch
	b.WriteString(strconv.Itoa(v.major))
	b.WriteByte('.')
	b.WriteString(strconv.Itoa(v.minor))
	b.WriteByte('.')
	b.WriteString(strconv.Itoa(v.patch))

	// Prerelease
	if len(v.prerelease) > 0 {
		b.WriteByte('-')
		b.WriteString(strings.Join(v.prerelease, "."))
	}

	// Build metadata
	if len(v.build) > 0 {
		b.WriteByte('+')
		b.WriteString(strings.Join(v.build, "."))
	}

	return b.String()
}

// String returns the original version string
func (v *NPMVersion) String() string {
	return v.original
}

// Compare compares this version with another version
// Following semver 2.0.0 precedence rules
func (v *NPMVersion) Compare(other Version) int {
	o, ok := other.(*NPMVersion)
	if !ok {
		return 0
	}

	// Compare core version numbers (major.minor.patch)
	if cmp := v.compareCoreVersion(o); cmp != 0 {
		return cmp
	}

	// Compare prerelease versions
	return v.comparePrerelease(o)
}

// compareCoreVersion compares major, minor, and patch versions
func (v *NPMVersion) compareCoreVersion(o *NPMVersion) int {
	if cmp := compareInt(v.major, o.major); cmp != 0 {
		return cmp
	}
	if cmp := compareInt(v.minor, o.minor); cmp != 0 {
		return cmp
	}
	return compareInt(v.patch, o.patch)
}

// comparePrerelease compares prerelease versions according to semver 2.0.0
func (v *NPMVersion) comparePrerelease(o *NPMVersion) int {
	// When a major, minor, and patch are equal, a pre-release version has lower precedence than a normal version
	if len(v.prerelease) == 0 && len(o.prerelease) > 0 {
		return 1
	}
	if len(v.prerelease) > 0 && len(o.prerelease) == 0 {
		return -1
	}

	if len(v.prerelease) == 0 && len(o.prerelease) == 0 {
		return 0 // Build metadata is ignored in version precedence
	}

	return v.comparePrereleaseIdentifiers(o)
}

// comparePrereleaseIdentifiers compares prerelease identifiers
func (v *NPMVersion) comparePrereleaseIdentifiers(o *NPMVersion) int {
	minLen := len(v.prerelease)
	if len(o.prerelease) < minLen {
		minLen = len(o.prerelease)
	}

	for i := 0; i < minLen; i++ {
		if cmp := comparePrereleaseIdentifier(v.prerelease[i], o.prerelease[i]); cmp != 0 {
			return cmp
		}
	}

	// All compared parts are equal, longer prerelease has higher precedence
	return compareInt(len(v.prerelease), len(o.prerelease))
}

// comparePrereleaseIdentifier compares two prerelease identifiers
func comparePrereleaseIdentifier(vPart, oPart string) int {
	vNum, vErr := strconv.Atoi(vPart)
	oNum, oErr := strconv.Atoi(oPart)

	// Both are numbers
	if vErr == nil && oErr == nil {
		return compareInt(vNum, oNum)
	}

	// One is number, one is string - numbers have lower precedence
	if vErr == nil && oErr != nil {
		return -1
	}
	if vErr != nil && oErr == nil {
		return 1
	}

	// Both are strings - lexical comparison
	if vPart < oPart {
		return -1
	}
	if vPart > oPart {
		return 1
	}
	return 0
}

// compareInt compares two integers
func compareInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// NormalizeNPMVersion normalizes npm version strings
// Handles common npm version patterns like workspace:, file:, git:, etc.
// Following deps.dev patterns: preserve full strings for git/github/http URLs
func NormalizeNPMVersion(version string) string {
	version = strings.TrimSpace(version)

	// Handle workspace protocol - normalize to "workspace"
	if strings.HasPrefix(version, "workspace:") {
		return "workspace"
	}

	// Handle file protocol - normalize to "local"
	if strings.HasPrefix(version, "file:") {
		return "local"
	}

	// Handle npm protocol (e.g., "npm:package@^1.0.0") - extract package@version
	if strings.HasPrefix(version, "npm:") {
		// Remove "npm:" prefix and return the rest (package@version)
		return strings.TrimPrefix(version, "npm:")
	}

	// Preserve full git URLs (deps.dev pattern)
	if strings.HasPrefix(version, "git:") || strings.HasPrefix(version, "git+") {
		return version
	}

	// Preserve full github references (deps.dev pattern)
	if strings.HasPrefix(version, "github:") {
		return version
	}

	// Preserve full HTTP/HTTPS URLs (deps.dev pattern)
	if strings.HasPrefix(version, "http:") || strings.HasPrefix(version, "https:") {
		return version
	}

	// Handle link protocol
	if strings.HasPrefix(version, "link:") {
		return "link"
	}

	// Handle version ranges and constraints
	if version == "" || version == "*" || version == "latest" {
		return "latest"
	}

	// Try to parse and normalize
	v, err := NPM.Parse(version)
	if err != nil {
		// Return original if parsing fails
		return version
	}

	return v.Canon(true)
}
