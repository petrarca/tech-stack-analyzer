package license

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessLicenseExpression_SingleLicense(t *testing.T) {
	payload := &types.Payload{Reason: make(map[string][]string)}
	ProcessLicenseExpression("MIT", "package.json", payload)

	require.Len(t, payload.Licenses, 1)
	assert.Equal(t, "MIT", payload.Licenses[0].LicenseName)
	assert.Equal(t, "direct", payload.Licenses[0].DetectionType)
	assert.Equal(t, "package.json", payload.Licenses[0].SourceFile)
	assert.Equal(t, 1.0, payload.Licenses[0].Confidence)
}

func TestProcessLicenseExpression_NormalizedLicense(t *testing.T) {
	payload := &types.Payload{Reason: make(map[string][]string)}
	ProcessLicenseExpression("apache 2.0", "pom.xml", payload)

	require.Len(t, payload.Licenses, 1)
	assert.Equal(t, "Apache-2.0", payload.Licenses[0].LicenseName)
	assert.Equal(t, "normalized", payload.Licenses[0].DetectionType)
	assert.Equal(t, "apache 2.0", payload.Licenses[0].OriginalLicense)
}

func TestProcessLicenseExpression_CompoundOR(t *testing.T) {
	payload := &types.Payload{Reason: make(map[string][]string)}
	ProcessLicenseExpression("MIT OR Apache-2.0", "Cargo.toml", payload)

	require.Len(t, payload.Licenses, 2)

	names := make(map[string]bool)
	for _, lic := range payload.Licenses {
		names[lic.LicenseName] = true
		assert.Equal(t, "expression_parsed", lic.DetectionType)
		assert.Equal(t, "MIT OR Apache-2.0", lic.OriginalLicense)
		assert.Equal(t, "Cargo.toml", lic.SourceFile)
	}
	assert.True(t, names["MIT"])
	assert.True(t, names["Apache-2.0"])
}

func TestProcessLicenseExpression_Empty(t *testing.T) {
	payload := &types.Payload{Reason: make(map[string][]string)}
	ProcessLicenseExpression("", "test.json", payload)

	assert.Empty(t, payload.Licenses)
}

func TestProcessLicenseExpression_DuplicateAvoidance(t *testing.T) {
	payload := &types.Payload{Reason: make(map[string][]string)}
	ProcessLicenseExpression("MIT", "package.json", payload)
	ProcessLicenseExpression("MIT", "deno.json", payload)

	assert.Len(t, payload.Licenses, 1, "Should not add duplicate license")
}

func TestAddLicenseToPayload_NoDuplicates(t *testing.T) {
	payload := &types.Payload{}
	lic := types.License{LicenseName: "MIT", SourceFile: "a.json"}
	AddLicenseToPayload(payload, lic)
	AddLicenseToPayload(payload, lic)

	assert.Len(t, payload.Licenses, 1)
}
