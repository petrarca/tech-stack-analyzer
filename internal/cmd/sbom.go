package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/sbom"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/spf13/cobra"
)

var (
	sbomFormat     string
	sbomOutput     string
	sbomPretty     bool
	sbomDirectOnly bool
)

// sbomCmd re-projects a previously written scan output JSON into an SBOM,
// without re-scanning. The scan's resolved dependencies are already in the
// output file, so this is a pure transformation -- useful to emit additional
// SBOM formats (or refresh one) from a single expensive scan.
var sbomCmd = &cobra.Command{
	Use:   "sbom <scan-output.json>",
	Short: "Generate an SBOM from a previously written scan output JSON",
	Long: `Generate a CycloneDX or SPDX SBOM from a scan output JSON file.

The SBOM is a projection of the scan's resolved dependencies, which are already
present in the scan output. This command reads that output and re-emits it as an
SBOM, so a single (potentially long) scan can produce multiple SBOM formats
without re-scanning.

The input must be a full scan output that still contains the "dependencies"
field (i.e. produced without --omit-fields dependencies and without --aggregate
stripping them).`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return runSBOM(args[0])
	},
}

func runSBOM(inputPath string) error {
	switch strings.ToLower(sbomFormat) {
	case "", "cyclonedx", "spdx":
	default:
		return fmt.Errorf("invalid --format %q: valid values are cyclonedx, spdx", sbomFormat)
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read scan output: %w", err)
	}

	var payload types.Payload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("parse scan output (expected a stack-analyzer scan JSON): %w", err)
	}

	out, err := marshalSBOM(&payload, sbomFormat, sbomDirectOnly, sbomPretty)
	if err != nil {
		return fmt.Errorf("generate SBOM: %w", err)
	}

	if sbomOutput == "" {
		_, err = os.Stdout.Write(out)
		return err
	}
	if err := os.WriteFile(sbomOutput, out, 0644); err != nil {
		return fmt.Errorf("write SBOM: %w", err)
	}
	fmt.Fprintf(os.Stderr, "SBOM written to %s\n", sbomOutput)
	return nil
}

// marshalSBOM builds and marshals an SBOM in the requested format from a
// payload. Shared shape with scan's generateSBOM, but operating on a payload
// loaded from disk rather than the in-memory scan result. When directOnly is
// set, only the project's direct dependencies are emitted, excluding transitive
// graph nodes even when the scan captured a full dependency graph.
func marshalSBOM(p *types.Payload, format string, directOnly, pretty bool) ([]byte, error) {
	if strings.EqualFold(format, "spdx") {
		var doc *sbom.SPDXDocument
		if directOnly {
			doc = sbom.SPDXFromPayloadDirect(p)
		} else {
			doc = sbom.SPDXFromPayload(p)
		}
		sbom.SPDXStamp(doc)
		return marshalJSON(doc, pretty)
	}
	var bom *sbom.BOM
	if directOnly {
		bom = sbom.FromPayloadDirect(p)
	} else {
		bom = sbom.FromPayload(p)
	}
	sbom.Stamp(bom)
	return marshalJSON(bom, pretty)
}

func init() {
	rootCmd.AddCommand(sbomCmd)
	sbomCmd.Flags().StringVar(&sbomFormat, "format", "cyclonedx", "SBOM format: 'cyclonedx' (CycloneDX 1.7 JSON) or 'spdx' (SPDX 2.3 JSON).")
	sbomCmd.Flags().StringVarP(&sbomOutput, "output", "o", "", "Output file path (default: stdout).")
	sbomCmd.Flags().BoolVar(&sbomPretty, "pretty", true, "Pretty-print the JSON output.")
	sbomCmd.Flags().BoolVar(&sbomDirectOnly, "direct-only", false, "Emit only the project's direct dependencies, excluding transitive (dependency-of-dependency) graph nodes. Matches the direct/transitive axis of --dependency-graph; has no effect when the scan captured no transitive graph.")
}
