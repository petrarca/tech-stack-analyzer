package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/petrarca/tech-stack-analyzer/internal/aggregator"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// generateAndWriteOutput generates JSON output (full or aggregated) and writes it.
func generateAndWriteOutput(payload interface{}, logger *slog.Logger) {
	logger.Debug("Generating output",
		"aggregate", settings.Aggregate,
		"also_aggregate", settings.AlsoAggregate,
		"pretty_print", settings.PrettyPrint)

	jsonData, err := generateOutput(payload, settings.Aggregate, settings.PrettyPrint, settings.OmitFields)
	if err != nil {
		logger.Error("Failed to marshal JSON", "error", err)
		os.Exit(1)
	}

	writeOutput(jsonData)

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

// generateOutput marshals the payload to JSON, optionally aggregating first.
func generateOutput(payload interface{}, aggregateFields string, prettyPrint bool, omitFields []string) ([]byte, error) {
	var result interface{}

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
			fields = []string{"tech", "techs", "reason", "languages", "licenses", "dependencies", "git", "components"}
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

	if prettyPrint {
		return json.MarshalIndent(result, "", "  ")
	}
	return json.Marshal(result)
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
