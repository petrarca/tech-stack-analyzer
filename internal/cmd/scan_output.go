package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log/slog"

	"github.com/petrarca/tech-stack-analyzer/internal/aggregator"
	"github.com/petrarca/tech-stack-analyzer/internal/currency"
	currencycache "github.com/petrarca/tech-stack-analyzer/internal/currency/cache"
	"github.com/petrarca/tech-stack-analyzer/internal/sbom"
	"github.com/petrarca/tech-stack-analyzer/internal/store"
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

	// Resolve dependency currency alongside the scan output when
	// --resolve-currency is set. Network-gated and opt-in (like --deps-dev), not
	// a pure companion-file emitter, so it lives in its own step.
	if settings.ResolveCurrency {
		writeCurrencyForPayload(payload, logger)
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

// currencyOutputFile derives the currency companion filename from the primary
// output filename. Returns empty string when primary output is stdout.
// Example: "output.json" -> "output.currency.json".
func currencyOutputFile(outputFile string) string {
	if outputFile == "" {
		return ""
	}
	ext := filepath.Ext(outputFile)
	base := strings.TrimSuffix(outputFile, ext)
	return base + ".currency.json"
}

// writeCurrencyForPayload resolves dependency currency for the scanned payload
// and writes the {out}.currency.json companion. It aggregates the payload's
// dependencies (reusing the aggregator's dedup + direct/transitive merge) and
// runs the currency engine over the direct set. Failures here are non-fatal:
// the scan output is already written; a currency error is reported, not exited.
func writeCurrencyForPayload(payload interface{}, logger *slog.Logger) {
	p, ok := payload.(*types.Payload)
	if !ok {
		logger.Debug("Skipping currency: payload is not a scan tree")
		return
	}
	outFile := currencyOutputFile(settings.OutputFile)
	if outFile == "" {
		logger.Debug("Skipping currency output: primary output is stdout")
		return
	}

	deps := aggregator.NewAggregator([]string{"dependencies"}).Aggregate(p).Dependencies

	cachePath, _, err := store.ResolvePath(settings.CurrencyCache)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Currency skipped: %v\n", err)
		return
	}
	st, err := store.Open(cachePath, 5000)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Currency skipped: open cache: %v\n", err)
		return
	}
	defer func() { _ = st.Close() }()

	ttl := time.Duration(settings.CurrencyTTLHours) * time.Hour
	chain := currency.NewChainResolver(currency.NewDepsDevResolver(settings.DepsDevEndpoint))
	resolver, err := currencycache.New(st, chain, ttl, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Currency skipped: %v\n", err)
		return
	}

	stop := startCurrencyReporter(settings.Quiet)
	art := currency.Resolve(deps, resolver, currency.Options{
		SourceEndpoint: depsDevEndpointOrDefault(settings.DepsDevEndpoint),
		TTLHours:       settings.CurrencyTTLHours,
		DirectOnly:     true,
	})
	stop()

	if err := art.WriteFile(outFile); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write currency output file: %v\n", err)
		return
	}
	if !settings.Quiet {
		s := art.Summary
		fmt.Fprintf(os.Stderr,
			"Currency written to %s (%d direct: %d up-to-date, %d behind, %d unsupported, %d unknown)\n",
			outFile, s.Total, s.UpToDate, s.Patch+s.Minor+s.Major, s.Unsupported, s.Unknown)
	}
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
