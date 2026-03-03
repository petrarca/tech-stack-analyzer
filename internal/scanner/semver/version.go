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

// Package semver provides semantic version parsing and comparison for multiple package ecosystems.
// Supports: PyPI (PEP 440), npm (semver), cargo (Rust), and more.
package semver

import (
	"fmt"
)

// System represents a versioning system (PyPI, npm, cargo, etc.)
type System interface {
	// Parse parses a version string according to the system's rules
	Parse(version string) (Version, error)

	// Name returns the name of the versioning system
	Name() string
}

// Version represents a parsed semantic version
type Version interface {
	// Canon returns the canonical string representation of the version
	Canon(includeEpoch bool) string

	// Compare compares this version with another version
	// Returns: -1 if this < other, 0 if this == other, 1 if this > other
	Compare(other Version) int

	// String returns the original version string
	String() string
}

// Common versioning systems
var (
	PyPI  System = &pypiSystem{}
	NPM   System = &npmSystem{}
	Cargo System = &cargoSystem{}
	Maven System = &mavenSystem{}
)

// ParseError represents a version parsing error
type ParseError struct {
	System  string
	Version string
	Reason  string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("%s version parse error: %s: %s", e.System, e.Version, e.Reason)
}

// parseError creates a new ParseError
func parseError(system, version, reason string) ParseError {
	return ParseError{
		System:  system,
		Version: version,
		Reason:  reason,
	}
}

// Normalize attempts to normalize a version string for the given system
// Returns the original string if parsing fails
func Normalize(system System, version string) string {
	v, err := system.Parse(version)
	if err != nil {
		return version
	}
	return v.Canon(true)
}

// cargoSystem is a placeholder for cargo semver support (to be implemented)
type cargoSystem struct{}

func (s *cargoSystem) Name() string {
	return "cargo"
}

func (s *cargoSystem) Parse(version string) (Version, error) {
	// TODO: Implement cargo semver parsing
	return nil, parseError("cargo", version, "not yet implemented")
}

// isDigit returns true if the byte is a digit
func isDigit(b byte) bool {
	return '0' <= b && b <= '9'
}
