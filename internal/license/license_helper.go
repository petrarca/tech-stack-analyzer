// Package license provides shared license detection, normalization, and SPDX expression
// parsing for use by component detectors.
package license

import (
	"fmt"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ProcessLicenseExpression processes a raw license string from a manifest file,
// normalizes it via SPDX mappings, handles compound expressions (OR/AND),
// and adds the resulting license(s) to the payload with traceability reasons.
//
// This is the shared implementation used by all component detectors that support
// manifest-based license detection (Node.js, PHP, Rust, Java, .NET, C++, Python, etc.).
func ProcessLicenseExpression(rawLicense string, sourceFile string, payload *types.Payload) {
	if rawLicense == "" {
		return
	}

	normalizer := NewNormalizer()
	licenses := normalizer.ParseLicenseExpression(rawLicense)

	if len(licenses) == 0 {
		payload.AddReason(fmt.Sprintf("license ignored: %q (invalid expression from %s)", rawLicense, sourceFile))
		return
	}

	if len(licenses) == 1 {
		licenseObj := types.License{
			LicenseName: licenses[0],
			SourceFile:  sourceFile,
			Confidence:  1.0,
		}

		if licenses[0] == rawLicense {
			licenseObj.DetectionType = "direct"
			payload.AddReason(fmt.Sprintf("license detected: %s (from %s)", licenses[0], sourceFile))
		} else {
			licenseObj.DetectionType = "normalized"
			licenseObj.OriginalLicense = rawLicense
			payload.AddReason(fmt.Sprintf("license normalized: %q -> %s (from %s, SPDX format)", rawLicense, licenses[0], sourceFile))
		}

		AddLicenseToPayload(payload, licenseObj)
	} else {
		reason := fmt.Sprintf("license expression parsed: %q -> [%s] (from %s, SPDX format)", rawLicense, strings.Join(licenses, ", "), sourceFile)
		for _, licenseName := range licenses {
			licenseObj := types.License{
				LicenseName:     licenseName,
				DetectionType:   "expression_parsed",
				SourceFile:      sourceFile,
				Confidence:      1.0,
				OriginalLicense: rawLicense,
			}
			AddLicenseToPayload(payload, licenseObj)
			payload.AddReason(reason)
		}
	}
}

// AddLicenseToPayload adds a license to the payload, avoiding duplicates by license name.
func AddLicenseToPayload(payload *types.Payload, license types.License) {
	for _, existing := range payload.Licenses {
		if existing.LicenseName == license.LicenseName {
			return
		}
	}
	payload.Licenses = append(payload.Licenses, license)
}
