package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMavenVersion(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		// Simple numeric versions
		{
			name:     "simple version",
			input:    "1.0.0",
			expected: "1.0.0",
		},
		{
			name:     "two-part version",
			input:    "1.0",
			expected: "1.0",
		},
		{
			name:     "single part version",
			input:    "1",
			expected: "1",
		},

		// Maven qualifiers
		{
			name:     "RELEASE qualifier",
			input:    "1.0.0-RELEASE",
			expected: "1.0.0",
		},
		{
			name:     "FINAL qualifier",
			input:    "2.7.0.FINAL",
			expected: "2.7.0",
		},
		{
			name:     "GA qualifier",
			input:    "3.0.0.GA",
			expected: "3.0.0",
		},
		{
			name:     "SNAPSHOT qualifier",
			input:    "1.0.0-SNAPSHOT",
			expected: "1.0.0-snapshot",
		},
		{
			name:     "BUILD qualifier",
			input:    "1.0.0-BUILD",
			expected: "1.0.0-build",
		},
		{
			name:     "RC qualifier",
			input:    "1.0.0-RC",
			expected: "1.0.0-rc",
		},
		{
			name:     "M1 qualifier",
			input:    "1.0.0-M1",
			expected: "1.0.0-m1",
		},
		{
			name:     "SP qualifier",
			input:    "1.0.0-SP",
			expected: "1.0.0-sp",
		},

		// Build number versions
		{
			name:     "build number version",
			input:    "1.0.0-20131201.121010-1",
			expected: "1.0.0-build.1",
		},
		{
			name:     "build number without qualifier",
			input:    "1.0.0-20131201.121010-5",
			expected: "1.0.0-build.5",
		},

		// Version ranges
		{
			name:     "inclusive range",
			input:    "[1.0,2.0]",
			expected: "1.0-2.0",
		},
		{
			name:     "exclusive upper bound",
			input:    "[1.0,2.0)",
			expected: "1.0-2.0",
		},
		{
			name:     "exclusive lower bound",
			input:    "(1.0,2.0]",
			expected: "1.0-2.0",
		},
		{
			name:     "open range lower",
			input:    "(,2.0]",
			expected: "<=2.0",
		},
		{
			name:     "open range upper",
			input:    "[1.0,)",
			expected: ">=1.0",
		},
		{
			name:     "wildcard range",
			input:    "[,]",
			expected: "*",
		},

		// Edge cases
		{
			name:     "version with dash",
			input:    "1.0.0-alpha",
			expected: "1.0.0-alpha",
		},
		{
			name:     "complex qualifier",
			input:    "1.0.0.RELEASE",
			expected: "1.0.0",
		},

		// Error cases
		{
			name:        "empty version",
			input:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := parseMavenVersion(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, version)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, version)
			assert.Equal(t, tt.input, version.String())
			assert.Equal(t, tt.expected, version.Canon(false))
		})
	}
}

func TestMavenVersion_Compare(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{
			name:     "equal versions",
			v1:       "1.0.0",
			v2:       "1.0.0",
			expected: 0,
		},
		{
			name:     "v1 less than v2",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: -1,
		},
		{
			name:     "v1 greater than v2",
			v1:       "2.0.0",
			v2:       "1.0.0",
			expected: 1,
		},
		{
			name:     "canonicalized versions",
			v1:       "1.0.0-RELEASE",
			v2:       "1.0.0",
			expected: 0,
		},
		{
			name:     "snapshot vs release",
			v1:       "1.0.0-snapshot",
			v2:       "1.0.0",
			expected: 1, // '1.0.0-snapshot' > '1.0.0' in string comparison
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1, err1 := parseMavenVersion(tt.v1)
			v2, err2 := parseMavenVersion(tt.v2)

			require.NoError(t, err1)
			require.NoError(t, err2)

			result := v1.Compare(v2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMavenSystem(t *testing.T) {
	system := &mavenSystem{}

	assert.Equal(t, "Maven", system.Name())

	// Test Parse method
	version, err := system.Parse("1.0.0-RELEASE")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", version.Canon(false))
	assert.Equal(t, "1.0.0-RELEASE", version.String())
}

func TestMavenVersionRangeDetection(t *testing.T) {
	tests := []struct {
		input    string
		isRange  bool
		expected string
	}{
		{"1.0.0", false, "1.0.0"},
		{"[1.0,2.0]", true, "1.0-2.0"},
		{"(1.0,2.0)", true, "1.0-2.0"},
		{"[1.0,)", true, ">=1.0"},
		{"(,2.0]", true, "<=2.0"},
		{"[,]", true, "*"},
		{"1.0.0-SNAPSHOT", false, "1.0.0-snapshot"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.isRange, isMavenVersionRange(tt.input))

			version, err := parseMavenVersion(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, version.Canon(false))
		})
	}
}

func TestMavenVersionEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "multiple dots",
			input:    "1.2.3.4.5",
			expected: "1.2.3.4.5",
		},
		{
			name:     "version with text",
			input:    "1.0.0-custom",
			expected: "1.0.0-custom",
		},
		{
			name:     "just qualifier",
			input:    "RELEASE",
			expected: "RELEASE",
		},
		{
			name:     "empty range parts",
			input:    "[1.0,]",
			expected: ">=1.0",
		},
		{
			name:     "range with spaces",
			input:    "[ 1.0 , 2.0 ]",
			expected: "1.0-2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := parseMavenVersion(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, version.Canon(false))
		})
	}
}
