package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/petrarca/tech-stack-analyzer/internal/aggregator"
	"github.com/petrarca/tech-stack-analyzer/internal/config"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// loadAndMergeScanConfig loads scan configuration and merges with settings.
func loadAndMergeScanConfig(logger *slog.Logger) *config.ScanConfigFile {
	if scanConfigPath == "" {
		return nil
	}

	scanConfig, err := config.LoadScanConfig(scanConfigPath)
	if err != nil {
		logger.Error("Failed to load scan configuration", "error", err)
		os.Exit(1)
	}

	scanConfig.MergeWithSettings(settings)
	return scanConfig
}

// setupScanSettings validates and normalises scan settings from flags.
func setupScanSettings(logger *slog.Logger) {
	if settings.OutputFile == "-" {
		settings.OutputFile = ""
	}

	if settings.Verbose && settings.Debug {
		logger.Error("Cannot use --verbose and --debug together. Choose one.")
		os.Exit(1)
	}

	if (settings.TraceRules || settings.TraceTimings) && !settings.Verbose && !settings.Debug {
		var flags []string
		if settings.TraceTimings {
			flags = append(flags, "--trace-timings")
		}
		if settings.TraceRules {
			flags = append(flags, "--trace-rules")
		}
		fmt.Fprintf(os.Stderr, "Error: %s requires --verbose or --debug\n", strings.Join(flags, " and "))
		fmt.Fprintf(os.Stderr, "\nUsage:\n")
		fmt.Fprintf(os.Stderr, "  stack-analyzer scan . --verbose %s  # Human-readable output\n", strings.Join(flags, " "))
		fmt.Fprintf(os.Stderr, "  stack-analyzer scan . --debug %s    # Machine-readable CSV output\n", strings.Join(flags, " "))
		fmt.Fprintf(os.Stderr, "\nSee --help for more information.\n")
		os.Exit(1)
	}

	if err := settings.Validate(); err != nil {
		logger.Error("Invalid settings", "error", err)
		os.Exit(1)
	}
}

// loadAndMergeProjectConfig loads the project-level .stack-analyzer.yml and merges
// it with the --config scan config. Returns (projectConfig, mergedConfig).
func loadAndMergeProjectConfig(absPath string, logger *slog.Logger) (*config.ScanConfig, *config.ScanConfig) {
	projectConfig, err := config.LoadConfig(absPath)
	if err != nil {
		logger.Error("Failed to load project configuration", "error", err)
		os.Exit(1)
	}

	var mergedConfig *config.ScanConfig
	if scanConfig != nil {
		mergedConfig = scanConfig.GetMergedConfig(projectConfig)
	} else {
		mergedConfig = projectConfig
	}

	settings.ExcludePatterns = mergedConfig.MergeExcludes(settings.ExcludePatterns)

	if settings.RootID == "" && mergedConfig.RootID != "" {
		settings.RootID = mergedConfig.RootID
	}

	return projectConfig, mergedConfig
}

// runScanner creates and runs the scanner, finalises code stats and primary_techs.
func runScanner(absPath string, isFile bool, mergedConfig *config.ScanConfig, logger *slog.Logger) interface{} {
	scannerPath := absPath
	if isFile {
		scannerPath = filepath.Dir(absPath)
	}

	if isFile {
		fmt.Fprintf(os.Stderr, "Scanning file: %s\n", absPath)
	} else {
		fmt.Fprintf(os.Stderr, "Scanning: %s\n", scannerPath)
	}

	if len(settings.FilterRules) > 0 {
		fmt.Fprintf(os.Stderr, "Active rules: %s\n", strings.Join(settings.FilterRules, ", "))
	}

	logger.Debug("Initializing scanner",
		"path", scannerPath,
		"exclude_patterns", settings.ExcludePatterns,
		"code_stats", !settings.NoCodeStats)

	codeStatsAnalyzer := buildCodeStatsAnalyzer(settings)

	s, err := scanner.NewScannerWithOptionsAndLogger(scannerPath, settings.ExcludePatterns, settings.Verbose, settings.Debug, settings.TraceTimings, settings.TraceRules, codeStatsAnalyzer, logger, settings.RootID, mergedConfig)
	if err != nil {
		logger.Error("Failed to create scanner", "error", err)
		os.Exit(1)
	}
	s.SetSubsystemDepth(settings.SubsystemDepth)
	s.SetSubsystemGroups(settings.SubsystemGroups)

	var payload interface{}
	if isFile {
		logger.Debug("Scanning file", "file", absPath)
		payload, err = s.ScanFile(filepath.Base(absPath))
	} else {
		logger.Debug("Scanning directory", "directory", absPath)
		payload, err = s.Scan()
	}

	if err != nil {
		logger.Error("Failed to scan", "error", err)
		os.Exit(1)
	}

	if p, ok := payload.(*types.Payload); ok {
		finalizeCodeStats(p, codeStatsAnalyzer, settings.ComponentStatsDepth, s.ResolveSubsystemKeyFromPath, settings.SubsystemGroups)
		p.PrimaryTechs = computePrimaryTechsFromPayload(p)
	}

	return payload
}

// enhanceSinglePayload adds configuration-driven data to the payload (properties, techs).
// Must run before computePrimaryTechsFromPayload so config-injected techs are included.
func enhanceSinglePayload(payload interface{}, mergedConfig *config.ScanConfig) {
	if mergedConfig == nil {
		return
	}

	if p, ok := payload.(*types.Payload); ok && len(mergedConfig.Properties) > 0 {
		if p.Properties == nil {
			p.Properties = make(map[string]interface{})
		}
		for k, v := range mergedConfig.Properties {
			p.Properties[k] = v
		}
	}

	if p, ok := payload.(*types.Payload); ok && len(mergedConfig.Techs) > 0 {
		allRules, _ := LoadRulesAndCategories()
		ruleMap := make(map[string]*types.Rule)
		for i := range allRules {
			ruleMap[allRules[i].Tech] = &allRules[i]
		}

		for _, configTech := range mergedConfig.Techs {
			techKey := configTech.Tech
			reason := configTech.Reason

			if _, exists := ruleMap[techKey]; !exists {
				techKey = "unmapped_unknown"
				if reason == "" {
					reason = fmt.Sprintf("configured tech: %s (source: config)", configTech.Tech)
				} else {
					reason = fmt.Sprintf("configured tech: %s (source: config) - %s", configTech.Tech, reason)
				}
			} else if reason == "" {
				reason = "configured tech (source: config)"
			}

			p.AddTech(techKey, reason)
		}
	}
}

// computePrimaryTechsFromPayload computes primary_techs via a temporary aggregation
// over the flat component list. This ensures a single, deterministic code path for
// both full and aggregated output formats.
func computePrimaryTechsFromPayload(p *types.Payload) []string {
	agg := aggregator.NewAggregator([]string{"tech", "components"})
	return agg.Aggregate(p).PrimaryTechs
}
