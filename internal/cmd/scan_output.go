package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/petrarca/tech-stack-analyzer/internal/aggregator"
	"github.com/petrarca/tech-stack-analyzer/internal/sbom"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// generateAndWriteOutput generates JSON output (full or aggregated) and writes it.
func generateAndWriteOutput(payload interface{}, logger *slog.Logger) {
	logger.Debug("Generating output",
		"aggregate", settings.Aggregate,
		"also_aggregate", settings.AlsoAggregate,
		"sbom", settings.SBOM,
		"also_sbom", settings.AlsoSBOM,
		"pretty_print", settings.PrettyPrint)

	// --sbom makes the CycloneDX SBOM the primary output instead of the scan tree.
	if settings.SBOM {
		sbomData, err := generateSBOM(payload, settings.PrettyPrint)
		if err != nil {
			logger.Error("Failed to marshal SBOM", "error", err)
			os.Exit(1)
		}
		writeOutput(sbomData)
		return
	}

	jsonData, err := generateOutput(payload, settings.Aggregate, settings.PrettyPrint, settings.OmitFields)
	if err != nil {
		logger.Error("Failed to marshal JSON", "error", err)
		os.Exit(1)
	}

	writeOutput(jsonData)

	// Produce a CycloneDX SBOM companion file when --also-sbom is set.
	if settings.AlsoSBOM {
		sbomFile := sbomOutputFile(settings.OutputFile)
		sbomData, err := generateSBOM(payload, settings.PrettyPrint)
		if err != nil {
			logger.Error("Failed to marshal SBOM", "error", err)
			os.Exit(1)
		}
		if sbomFile != "" {
			if err = os.WriteFile(sbomFile, sbomData, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write SBOM output file: %v\n", err)
				os.Exit(1)
			}
			if !settings.Quiet {
				fmt.Fprintf(os.Stderr, "SBOM written to %s\n", sbomFile)
			}
		} else {
			logger.Debug("Skipping SBOM output: primary output is stdout")
		}
	}

	// Produce aggregate output alongside full output when --also-aggregate is set.
	if settings.AlsoAggregate != "" && settings.Aggregate == "" {
		aggFile := aggregateOutputFile(settings.OutputFile)
		aggData, err := generateOutput(payload, settings.AlsoAggregate, settings.PrettyPrint, nil)
		if err != nil {
			logger.Error("Failed to marshal aggregate JSON", "error", err)
			os.Exit(1)
		}
		if aggFile != "" {
			if err = os.WriteFile(aggFile, aggData, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write aggregate output file: %v\n", err)
				os.Exit(1)
			}
			if !settings.Quiet {
				fmt.Fprintf(os.Stderr, "Aggregate results written to %s\n", aggFile)
			}
		} else {
			// stdout mode — two JSON blobs cannot be written to stdout.
			logger.Debug("Skipping aggregate output: primary output is stdout")
		}
	}
}

// aggregateOutputFile derives the aggregate output filename from the primary output filename.
// Returns empty string when primary output is stdout.
// Example: "output.json" → "output-agg.json"
func aggregateOutputFile(outputFile string) string {
	if outputFile == "" {
		return ""
	}
	ext := filepath.Ext(outputFile)
	base := strings.TrimSuffix(outputFile, ext)
	return base + "-agg" + ext
}

// sbomOutputFile derives the SBOM companion filename from the primary output
// filename, using a format-specific infix. Returns empty string when primary
// output is stdout.
// Example: "output.json" → "output.cdx.json" (CycloneDX) or
// "output.spdx.json" (SPDX).
func sbomOutputFile(outputFile string) string {
	if outputFile == "" {
		return ""
	}
	ext := filepath.Ext(outputFile)
	base := strings.TrimSuffix(outputFile, ext)
	return base + "." + sbomFileInfix(settings.SBOMFormat) + ext
}

// sbomFileInfix returns the filename infix for an SBOM format.
func sbomFileInfix(format string) string {
	if strings.EqualFold(format, "spdx") {
		return "spdx"
	}
	return "cdx"
}

// generateSBOM builds an SBOM from the payload in the configured format and
// marshals it. Defaults to CycloneDX when no format is set.
func generateSBOM(payload interface{}, prettyPrint bool) ([]byte, error) {
	p, ok := payload.(*types.Payload)
	if !ok {
		return nil, fmt.Errorf("SBOM output requires a scan payload")
	}
	if strings.EqualFold(settings.SBOMFormat, "spdx") {
		doc := sbom.SPDXFromPayload(p)
		sbom.SPDXStamp(doc) // per-emission documentNamespace + timestamp
		return marshalJSON(doc, prettyPrint)
	}
	bom := sbom.FromPayload(p)
	sbom.Stamp(bom) // per-emission serialNumber + timestamp
	return marshalJSON(bom, prettyPrint)
}

// stripFields recursively removes the specified fields from a payload tree.
func stripFields(p *types.Payload, fields map[string]bool) {
	if p == nil {
		return
	}
	if fields["reason"] {
		p.Reason = nil
	}
	if fields["path"] {
		p.Path = nil
	}
	if fields["edges"] {
		p.Edges = nil
	}
	if fields["licenses"] {
		p.Licenses = nil
	}
	if fields["dependencies"] {
		p.Dependencies = nil
	}
	if fields["component_refs"] {
		p.ComponentRefs = nil
	}
	if fields["properties"] {
		p.Properties = nil
	}
	if fields["code_stats"] {
		p.CodeStats = nil
	}
	if fields["primary_languages"] {
		p.PrimaryLanguages = nil
	}
	if fields["primary_techs"] {
		p.PrimaryTechs = nil
	}
	for _, child := range p.Children {
		stripFields(child, fields)
	}
}

// normalizeTech ensures the tech field is always an empty array rather than
// null. A nil []string slice marshals to JSON null; an empty (non-nil) slice
// marshals to []. Components with no primary technology must emit "tech": []
// so the field is uniformly an array across full and aggregated output.
func normalizeTech(p *types.Payload) {
	if p == nil {
		return
	}
	if p.Tech == nil {
		p.Tech = []string{}
	}
	for _, child := range p.Children {
		normalizeTech(child)
	}
}

// generateOutput marshals the payload to JSON, optionally aggregating first.
func generateOutput(payload interface{}, aggregateFields string, prettyPrint bool, omitFields []string) ([]byte, error) {
	var result interface{}

	if p, ok := payload.(*types.Payload); ok {
		normalizeTech(p)
	}

	if len(omitFields) > 0 {
		if p, ok := payload.(*types.Payload); ok {
			omitSet := make(map[string]bool, len(omitFields))
			for _, f := range omitFields {
				omitSet[strings.TrimSpace(f)] = true
			}
			stripFields(p, omitSet)
		}
	}

	if aggregateFields != "" {
		fields := strings.Split(aggregateFields, ",")
		for i, field := range fields {
			fields[i] = strings.TrimSpace(field)
		}

		if len(fields) == 1 && fields[0] == "all" {
			fields = []string{"tech", "techs", "languages", "licenses", "dependencies", "git", "components"}
		}

		validFields := map[string]bool{
			"tech": true, "techs": true, "reason": true, "languages": true,
			"licenses": true, "dependencies": true, "git": true, "components": true,
		}
		for _, field := range fields {
			if !validFields[field] {
				return nil, fmt.Errorf("invalid aggregate field: %s. Valid fields: tech, techs, reason, languages, licenses, dependencies, git, components, all", field)
			}
		}

		agg := aggregator.NewAggregator(fields)
		result = agg.Aggregate(payload.(*types.Payload))
	} else {
		result = payload
	}

	return marshalJSON(result, prettyPrint)
}

// writeOutput writes JSON data to the configured output file or stdout.
func writeOutput(jsonData []byte) {
	if settings.OutputFile != "" {
		if err := os.WriteFile(settings.OutputFile, jsonData, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Results written to %s\n", settings.OutputFile)
	} else {
		fmt.Println(string(jsonData))
	}
}
