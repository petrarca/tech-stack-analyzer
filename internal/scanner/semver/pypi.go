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

// pypiSystem implements PEP 440 version parsing
type pypiSystem struct{}

func (s *pypiSystem) Name() string {
	return "PyPI"
}

func (s *pypiSystem) Parse(version string) (Version, error) {
	return parsePyPIVersion(version)
}

// PyPIVersion represents a PEP 440 compliant version
// Based on: https://www.python.org/dev/peps/pep-0440/
type PyPIVersion struct {
	original string
	epoch    int
	release  []int
	pre      *preRelease
	post     *int
	dev      *int
	local    string
}

type preRelease struct {
	phase  string // "a", "b", "rc"
	number int
}

// parsePyPIVersion parses a PEP 440 version string
func parsePyPIVersion(version string) (*PyPIVersion, error) {
	if version == "" {
		return nil, parseError("PyPI", version, "empty version string")
	}

	v := &PyPIVersion{original: version}
	s := strings.ToLower(strings.TrimSpace(version))

	// Parse components in order: epoch, local, dev, post, pre, release
	var err error
	s, err = v.parseEpoch(s, version)
	if err != nil {
		return nil, err
	}

	s = v.parseLocal(s)

	s, err = v.parseDev(s, version)
	if err != nil {
		return nil, err
	}

	s, err = v.parsePost(s, version)
	if err != nil {
		return nil, err
	}

	s, err = v.parsePreRelease(s, version)
	if err != nil {
		return nil, err
	}

	err = v.parseRelease(s, version)
	if err != nil {
		return nil, err
	}

	return v, nil
}

// parseEpoch parses the epoch component (e.g., "1!")
func (v *PyPIVersion) parseEpoch(s, version string) (string, error) {
	if idx := strings.IndexByte(s, '!'); idx > 0 {
		epochStr := s[:idx]
		epoch, err := strconv.Atoi(epochStr)
		if err != nil {
			return s, parseError("PyPI", version, fmt.Sprintf("invalid epoch: %s", epochStr))
		}
		v.epoch = epoch
		return s[idx+1:], nil
	}
	return s, nil
}

// parseLocal parses the local version component (e.g., "+local.version")
func (v *PyPIVersion) parseLocal(s string) string {
	if idx := strings.IndexByte(s, '+'); idx >= 0 {
		v.local = s[idx+1:]
		return s[:idx]
	}
	return s
}

// parseDev parses the dev release component (e.g., ".dev0" or "dev0")
func (v *PyPIVersion) parseDev(s, version string) (string, error) {
	patterns := []struct {
		prefix string
		offset int
	}{
		{".dev", 4},
		{"dev", 3},
	}

	for _, pattern := range patterns {
		if idx := strings.Index(s, pattern.prefix); idx >= 0 {
			devStr := s[idx+pattern.offset:]
			dev, err := parseOptionalNumber(devStr, version, "dev")
			if err != nil {
				return s, err
			}
			v.dev = dev
			return s[:idx], nil
		}
	}
	return s, nil
}

// parsePost parses the post release component (e.g., ".post0", "post0", or "-0")
func (v *PyPIVersion) parsePost(s, version string) (string, error) {
	patterns := []struct {
		prefix string
		offset int
	}{
		{".post", 5},
		{"post", 4},
	}

	for _, pattern := range patterns {
		if idx := strings.Index(s, pattern.prefix); idx >= 0 {
			postStr := s[idx+pattern.offset:]
			post, err := parseOptionalNumber(postStr, version, "post")
			if err != nil {
				return s, err
			}
			v.post = post
			return s[:idx], nil
		}
	}

	// Handle post release with dash
	if idx := strings.LastIndexByte(s, '-'); idx >= 0 {
		postStr := s[idx+1:]
		if postStr != "" && isAllDigits(postStr) {
			if post, err := strconv.Atoi(postStr); err == nil {
				v.post = &post
				return s[:idx], nil
			}
		}
	}

	return s, nil
}

// parsePreRelease parses the pre-release component (alpha, beta, rc)
func (v *PyPIVersion) parsePreRelease(s, version string) (string, error) {
	preIdx, prePhase := findEarliestPreReleasePhase(s)

	if preIdx >= 0 {
		preNumStr := s[preIdx+len(prePhase):]
		s = s[:preIdx]

		prePhase = normalizePreReleasePhase(prePhase)

		preNum := 0
		if preNumStr != "" {
			var err error
			preNum, err = strconv.Atoi(preNumStr)
			if err != nil {
				return s, parseError("PyPI", version, fmt.Sprintf("invalid pre-release number: %s", preNumStr))
			}
		}

		v.pre = &preRelease{phase: prePhase, number: preNum}
	}

	return s, nil
}

// parseRelease parses the release numbers (e.g., "1.2.3")
func (v *PyPIVersion) parseRelease(s, version string) error {
	s = strings.TrimRight(s, ".")
	if s == "" {
		return parseError("PyPI", version, "no release numbers found")
	}

	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	})

	for _, part := range parts {
		if part == "" {
			continue
		}
		num, err := strconv.Atoi(part)
		if err != nil {
			return parseError("PyPI", version, fmt.Sprintf("invalid release number: %s", part))
		}
		v.release = append(v.release, num)
	}

	if len(v.release) == 0 {
		return parseError("PyPI", version, "no valid release numbers")
	}

	return nil
}

// parseOptionalNumber parses an optional number, returning a pointer to 0 if empty
func parseOptionalNumber(numStr, version, component string) (*int, error) {
	if numStr != "" {
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return nil, parseError("PyPI", version, fmt.Sprintf("invalid %s number: %s", component, numStr))
		}
		return &num, nil
	}
	zero := 0
	return &zero, nil
}

// findEarliestPreReleasePhase finds the earliest pre-release phase in the string
func findEarliestPreReleasePhase(s string) (int, string) {
	preIdx := -1
	prePhase := ""

	for _, phase := range []string{"rc", "c", "beta", "b", "alpha", "a"} {
		if idx := strings.Index(s, phase); idx >= 0 {
			if preIdx == -1 || idx < preIdx {
				preIdx = idx
				prePhase = phase
			}
		}
	}

	return preIdx, prePhase
}

// normalizePreReleasePhase normalizes pre-release phase names
func normalizePreReleasePhase(phase string) string {
	switch phase {
	case "alpha", "a":
		return "a"
	case "beta", "b":
		return "b"
	case "c", "rc":
		return "rc"
	}
	return phase
}

// Canon returns the canonical string representation of the version
func (v *PyPIVersion) Canon(includeEpoch bool) string {
	var b strings.Builder

	// Epoch
	if includeEpoch && v.epoch > 0 {
		b.WriteString(strconv.Itoa(v.epoch))
		b.WriteByte('!')
	}

	// Release
	for i, num := range v.release {
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(strconv.Itoa(num))
	}

	// Pre-release
	if v.pre != nil {
		b.WriteString(v.pre.phase)
		b.WriteString(strconv.Itoa(v.pre.number))
	}

	// Post-release
	if v.post != nil {
		b.WriteString(".post")
		b.WriteString(strconv.Itoa(*v.post))
	}

	// Dev-release
	if v.dev != nil {
		b.WriteString(".dev")
		b.WriteString(strconv.Itoa(*v.dev))
	}

	// Local version
	if v.local != "" {
		b.WriteByte('+')
		b.WriteString(v.local)
	}

	return b.String()
}

// String returns the original version string
func (v *PyPIVersion) String() string {
	return v.original
}

// Compare compares this version with another version
func (v *PyPIVersion) Compare(other Version) int {
	o, ok := other.(*PyPIVersion)
	if !ok {
		return 0
	}

	// Compare components in PEP 440 precedence order
	if cmp := compareInt(v.epoch, o.epoch); cmp != 0 {
		return cmp
	}

	if cmp := v.compareRelease(o); cmp != 0 {
		return cmp
	}

	if cmp := v.comparePreReleasePart(o); cmp != 0 {
		return cmp
	}

	if cmp := v.comparePostRelease(o); cmp != 0 {
		return cmp
	}

	return v.compareDevRelease(o)
}

// compareRelease compares release numbers
func (v *PyPIVersion) compareRelease(o *PyPIVersion) int {
	minLen := len(v.release)
	if len(o.release) < minLen {
		minLen = len(o.release)
	}

	for i := 0; i < minLen; i++ {
		if cmp := compareInt(v.release[i], o.release[i]); cmp != 0 {
			return cmp
		}
	}

	return compareInt(len(v.release), len(o.release))
}

// comparePreReleasePart compares pre-release versions (no pre-release > has pre-release)
func (v *PyPIVersion) comparePreReleasePart(o *PyPIVersion) int {
	if v.pre == nil && o.pre != nil {
		return 1
	}
	if v.pre != nil && o.pre == nil {
		return -1
	}
	if v.pre != nil && o.pre != nil {
		return comparePreRelease(v.pre, o.pre)
	}
	return 0
}

// comparePreRelease compares two pre-release versions
func comparePreRelease(v, o *preRelease) int {
	phaseOrder := map[string]int{"a": 1, "b": 2, "rc": 3}
	if cmp := compareInt(phaseOrder[v.phase], phaseOrder[o.phase]); cmp != 0 {
		return cmp
	}
	return compareInt(v.number, o.number)
}

// comparePostRelease compares post-release versions (no post < has post)
func (v *PyPIVersion) comparePostRelease(o *PyPIVersion) int {
	if v.post == nil && o.post != nil {
		return -1
	}
	if v.post != nil && o.post == nil {
		return 1
	}
	if v.post != nil && o.post != nil {
		return compareInt(*v.post, *o.post)
	}
	return 0
}

// compareDevRelease compares dev-release versions (no dev > has dev)
func (v *PyPIVersion) compareDevRelease(o *PyPIVersion) int {
	if v.dev == nil && o.dev != nil {
		return 1
	}
	if v.dev != nil && o.dev == nil {
		return -1
	}
	if v.dev != nil && o.dev != nil {
		return compareInt(*v.dev, *o.dev)
	}
	return 0
}

// isAllDigits returns true if the string contains only digits
func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return len(s) > 0
}
