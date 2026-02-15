package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/petrarca/tech-stack-analyzer/internal/aggregator"
	"github.com/petrarca/tech-stack-analyzer/internal/codestats"
	"github.com/petrarca/tech-stack-analyzer/internal/config"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/spf13/cobra"
)

// attachComponentCodeStats recursively attaches code stats to child components in the payload tree
// Note: This should be called AFTER setting the root payload's global CodeStats
// It only attaches stats to CHILD components, not the root (which keeps global stats)
func attachComponentCodeStats(payload *types.Payload, analyzer codestats.Analyzer) {
	// Early exit if per-component is not enabled - zero impact
	if !analyzer.IsPerComponentEnabled() {
		return
	}

	// Recursively attach to child components (NOT the root - it keeps global stats)
	for _, child := range payload.Children {
		attachComponentCodeStatsRecursive(child, analyzer)
	}
}

// convertPrimaryLanguages converts codestats.PrimaryLanguage to types.PrimaryLanguage
func convertPrimaryLanguages(src []codestats.PrimaryLanguage) []types.PrimaryLanguage {
	if len(src) == 0 {
		return nil
	}
	result := make([]types.PrimaryLanguage, len(src))
	for i, pl := range src {
		result[i] = types.PrimaryLanguage{
			Language: pl.Language,
			Pct:      pl.Pct,
		}
	}
	return result
}

// attachComponentCodeStatsRecursive attaches code stats to a component and its children
func attachComponentCodeStatsRecursive(payload *types.Payload, analyzer codestats.Analyzer) {
	// Attach stats to current component
	if stats := analyzer.GetComponentStats(payload.ID); stats != nil {
		payload.CodeStats = stats
	}

	// Recursively attach to child components
	for _, child := range payload.Children {
		attachComponentCodeStatsRecursive(child, analyzer)
	}
}

// parseLogLevel converts string log level to slog.Level
func parseLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	case "fatal":
		return slog.LevelError, nil // slog doesn't have fatal, use error
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level: %s", level)
	}
}

var (
	settings       *config.Settings
	scanConfig     *config.ScanConfigFile
	scanConfigPath string
)

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan a project or file for technology stack",
	Long: `Scan analyzes a project directory or single file to detect technologies,
frameworks, databases, and services used in your codebase.

Examples:
  stack-analyzer scan /path/to/project
  stack-analyzer scan /path/to/pom.xml
  stack-analyzer scan --config scan-config.yml /path/to/project
  stack-analyzer scan --config '{"scan":{"output":{"file":"$BUILD_DIR/scan-results.json"},"properties":{"build":"'$BUILD_NUMBER'"}}}' /path/to/project
  stack-analyzer scan --aggregate techs,languages /path/to/project
  stack-analyzer scan --aggregate all /path/to/project
  stack-analyzer scan --exclude vendor,node_modules /path/to/project
  stack-analyzer scan --exclude "**/__tests__/**" --exclude "*.log" /path/to/project`,
	Args: cobra.MaximumNArgs(1),
	Run:  runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)

	// Initialize settings with defaults and environment variables
	settings = config.LoadSettingsFromEnvironment()

	// Store environment variable values for flag defaults
	outputFile := settings.OutputFile
	aggregate := settings.Aggregate
	prettyPrint := settings.PrettyPrint
	verbose := settings.Verbose
	debug := settings.Debug
	traceTimings := settings.TraceTimings
	traceRules := settings.TraceRules
	logLevel := settings.LogLevel.String()
	logFormat := settings.LogFormat
	logFile := settings.LogFile

	// Set up flags with defaults from environment variables
	scanCmd.Flags().StringVarP(&settings.OutputFile, "output", "o", outputFile, "Output file path (default: stack-analysis.json)")
	scanCmd.Flags().StringVar(&settings.Aggregate, "aggregate", aggregate, "Aggregate fields: tech,techs,languages,licenses,dependencies,git,all")
	scanCmd.Flags().BoolVar(&settings.PrettyPrint, "pretty", prettyPrint, "Pretty print JSON output")
	scanCmd.Flags().BoolVarP(&settings.Verbose, "verbose", "v", verbose, "Show progress with simple output")
	scanCmd.Flags().BoolVarP(&settings.Debug, "debug", "d", debug, "Show progress with tree structure (cannot be used with --verbose)")
	scanCmd.Flags().BoolVar(&settings.TraceTimings, "trace-timings", traceTimings, "Show timing information for each directory (requires --verbose or --debug)")
	scanCmd.Flags().BoolVar(&settings.TraceRules, "trace-rules", traceRules, "Show detailed rule matching information (requires --verbose or --debug)")

	// Exclude patterns - support multiple flags or comma-separated values
	scanCmd.Flags().StringSliceVar(&settings.ExcludePatterns, "exclude", settings.ExcludePatterns, "Patterns to exclude (supports glob patterns, can be specified multiple times)")

	// Rule filtering for debugging
	scanCmd.Flags().StringSliceVar(&settings.FilterRules, "rules", settings.FilterRules, "Only use these rules (comma-separated tech names, e.g., c,cplusplus,nodejs - for debugging)")

	// Code statistics flag (enabled by default)
	scanCmd.Flags().BoolVar(&settings.NoCodeStats, "no-code-stats", settings.NoCodeStats, "Disable code statistics (lines of code, comments, blanks, complexity)")

	// Per-component code statistics flag (disabled by default)
	scanCmd.Flags().BoolVar(&settings.CodeStatsPerComponent, "component-code-stats", settings.CodeStatsPerComponent, "Enable per-component code statistics (lines of code, comments, blanks, complexity per component)")

	// Root ID override flag for deterministic scans
	scanCmd.Flags().StringVar(&settings.RootID, "root-id", "", "Override random root ID for deterministic scans (e.g., 'my-project-2024')")

	// Logging flags - use defaults from environment variables
	scanCmd.Flags().String("log-level", logLevel, "Log level: trace, debug, error, fatal")
	scanCmd.Flags().String("log-format", logFormat, "Log format: text or json")
	scanCmd.Flags().String("log-file", logFile, "Log file path (default: stderr)")

	// Scan configuration flag
	scanCmd.Flags().StringVar(&scanConfigPath, "config", "", "Scan configuration file path or inline JSON")
}

// configureLogging sets up logging based on command flags
func configureLogging(cmd *cobra.Command) *slog.Logger {
	logLevel, _ := cmd.Flags().GetString("log-level")
	logFormat, _ := cmd.Flags().GetString("log-format")
	logFile, _ := cmd.Flags().GetString("log-file")

	if level, err := parseLogLevel(logLevel); err == nil {
		settings.LogLevel = level
	}
	settings.LogFormat = logFormat
	settings.LogFile = logFile

	return settings.ConfigureLogger()
}

// resolveScanPath resolves and validates the scan path from args
func resolveScanPath(args []string, logger *slog.Logger) (absPath string, isFile bool) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	path = strings.TrimSpace(path)
	absPath, err := filepath.Abs(path)
	if err != nil {
		logger.Error("Invalid path", "error", err)
		os.Exit(1)
	}

	fileInfo, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		logger.Error("Path does not exist", "path", absPath)
		os.Exit(1)
	}
	return absPath, !fileInfo.IsDir()
}

// resolveSingleScanPath resolves a single scan path, handling both config and args
func resolveSingleScanPath(configPath string, args []string, logger *slog.Logger) (absPath string, isFile bool) {
	// Use traditional args-based resolution (ignore configPath since we removed multi-path)
	return resolveScanPath(args, logger)
}

// configureExcludePatterns processes exclude patterns from command flags
func configureExcludePatterns(cmd *cobra.Command) {
	excludeList, _ := cmd.Flags().GetStringSlice("exclude")
	for i, pattern := range excludeList {
		excludeList[i] = strings.TrimSpace(pattern)
	}
	settings.ExcludePatterns = excludeList
}

func runScan(cmd *cobra.Command, args []string) {
	logger := configureLogging(cmd)

	// Load and merge scan configuration
	scanConfig = loadAndMergeScanConfig(logger)

	// Single path scan - use command line argument or default to current directory
	runSinglePathScan("", args, cmd, logger)
}

// runSinglePathScan executes a single path scan with all the logic
func runSinglePathScan(configPath string, args []string, cmd *cobra.Command, logger *slog.Logger) {
	absPath, isFile := resolveSingleScanPath(configPath, args, logger)
	configureExcludePatterns(cmd)

	// Setup and validate scan settings
	setupScanSettings(logger)

	// Load project config and merge with scan config
	_, mergedConfig := loadAndMergeProjectConfig(absPath, logger)

	// Initialize and run scanner
	payload := runScanner(absPath, isFile, mergedConfig, logger)

	// Enhance payload with configuration data
	enhanceSinglePayload(payload, mergedConfig)

	// Generate and write output
	generateAndWriteOutput(payload, logger)
}

// loadAndMergeScanConfig loads scan configuration and merges with settings
func loadAndMergeScanConfig(logger *slog.Logger) *config.ScanConfigFile {
	if scanConfigPath == "" {
		return nil
	}

	scanConfig, err := config.LoadScanConfig(scanConfigPath)
	if err != nil {
		logger.Error("Failed to load scan configuration", "error", err)
		os.Exit(1)
	}

	// Merge config with settings (CLI flags take precedence)
	scanConfig.MergeWithSettings(settings)
	return scanConfig
}

func setupScanSettings(logger *slog.Logger) {
	// Handle special case: -o - means stdout
	if settings.OutputFile == "-" {
		settings.OutputFile = ""
	}

	// Check for mutually exclusive flags
	if settings.Verbose && settings.Debug {
		logger.Error("Cannot use --verbose and --debug together. Choose one.")
		os.Exit(1)
	}

	// Validate trace flags require explicit mode selection
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

	// Validate settings
	if err := settings.Validate(); err != nil {
		logger.Error("Invalid settings", "error", err)
		os.Exit(1)
	}
}

// loadAndMergeProjectConfig loads project config and merges with scan config
func loadAndMergeProjectConfig(absPath string, logger *slog.Logger) (*config.ScanConfig, *config.ScanConfig) {
	// Load project config for additional metadata and excludes
	projectConfig, err := config.LoadConfig(absPath)
	if err != nil {
		logger.Error("Failed to load project configuration", "error", err)
		os.Exit(1)
	}

	// Merge scan config with project config
	var mergedConfig *config.ScanConfig
	if scanConfig != nil {
		mergedConfig = scanConfig.GetMergedConfig(projectConfig)
	} else {
		mergedConfig = projectConfig
	}

	// Apply merged excludes to settings
	settings.ExcludePatterns = mergedConfig.MergeExcludes(settings.ExcludePatterns)

	// Apply root ID from config if not set by CLI (CLI takes precedence)
	if settings.RootID == "" && mergedConfig.RootID != "" {
		settings.RootID = mergedConfig.RootID
	}

	return projectConfig, mergedConfig
}

// runScanner creates and runs the scanner
func runScanner(absPath string, isFile bool, mergedConfig *config.ScanConfig, logger *slog.Logger) interface{} {
	// Initialize scanner
	scannerPath := absPath
	if isFile {
		scannerPath = filepath.Dir(absPath)
	}

	// Show scan start message (always, even without verbose)
	if isFile {
		fmt.Fprintf(os.Stderr, "Scanning file: %s\n", absPath)
	} else {
		fmt.Fprintf(os.Stderr, "Scanning: %s\n", scannerPath)
	}

	// Show filtered rules if specified
	if len(settings.FilterRules) > 0 {
		fmt.Fprintf(os.Stderr, "Active rules: %s\n", strings.Join(settings.FilterRules, ", "))
	}

	logger.Debug("Initializing scanner",
		"path", scannerPath,
		"exclude_patterns", settings.ExcludePatterns,
		"code_stats", !settings.NoCodeStats)

	// Create code stats analyzer (enabled by default, disabled with --no-code-stats)
	codeStatsAnalyzer := codestats.NewAnalyzerWithOptions(
		!settings.NoCodeStats,
		settings.CodeStatsPerComponent,
		settings.PrimaryLanguageThreshold,
		5, // maxPrimaryLangs - could be configurable in the future
	)

	s, err := scanner.NewScannerWithOptionsAndLogger(scannerPath, settings.ExcludePatterns, settings.Verbose, settings.Debug, settings.TraceTimings, settings.TraceRules, codeStatsAnalyzer, logger, settings.RootID, mergedConfig)
	if err != nil {
		logger.Error("Failed to create scanner", "error", err)
		os.Exit(1)
	}

	// Scan project or file
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

	// Attach code stats to payload if enabled
	if codeStatsAnalyzer.IsEnabled() {
		if p, ok := payload.(*types.Payload); ok {
			stats := codeStatsAnalyzer.GetStats()
			p.CodeStats = stats

			// Extract primary_languages from code_stats and set on root payload
			if cs, ok := stats.(*codestats.CodeStats); ok {
				if cs.ByType.Programming != nil && cs.ByType.Programming.Metrics != nil {
					p.PrimaryLanguages = convertPrimaryLanguages(cs.ByType.Programming.Metrics.PrimaryLanguages)
				}
			}

			// Attach per-component stats if enabled (post-processing)
			if codeStatsAnalyzer.IsPerComponentEnabled() {
				attachComponentCodeStats(p, codeStatsAnalyzer)
			}
		}
	}

	return payload
}

// enhanceSinglePayload adds configuration data to the payload
func enhanceSinglePayload(payload interface{}, mergedConfig *config.ScanConfig) {
	// Add merged config properties to payload metadata
	if p, ok := payload.(*types.Payload); ok && len(mergedConfig.Properties) > 0 {
		if p.Properties == nil {
			p.Properties = make(map[string]interface{})
		}
		for k, v := range mergedConfig.Properties {
			p.Properties[k] = v
		}
	}

	// Add configured techs to payload with validation
	if p, ok := payload.(*types.Payload); ok && len(mergedConfig.Techs) > 0 {
		// Load rules for tech validation
		allRules, _ := LoadRulesAndCategories()
		ruleMap := make(map[string]*types.Rule)
		for i := range allRules {
			ruleMap[allRules[i].Tech] = &allRules[i]
		}

		for _, configTech := range mergedConfig.Techs {
			// Check if tech exists in taxonomy
			techKey := configTech.Tech
			reason := configTech.Reason

			// If tech doesn't exist, map to unmapped_unknown
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

			// Add tech to payload using existing AddTech method
			p.AddTech(techKey, reason)
		}
	}
}

// generateAndWriteOutput generates output and writes to file or stdout
func generateAndWriteOutput(payload interface{}, logger *slog.Logger) {
	// Generate output (aggregated or full payload)
	logger.Debug("Generating output",
		"aggregate", settings.Aggregate,
		"pretty_print", settings.PrettyPrint)

	jsonData, err := generateOutput(payload, settings.Aggregate, settings.PrettyPrint)
	if err != nil {
		logger.Error("Failed to marshal JSON", "error", err)
		os.Exit(1)
	}

	// Write output
	writeOutput(jsonData)
}

func generateOutput(payload interface{}, aggregateFields string, prettyPrint bool) ([]byte, error) {
	var result interface{}

	if aggregateFields != "" {
		// Parse aggregate fields
		fields := strings.Split(aggregateFields, ",")
		for i, field := range fields {
			fields[i] = strings.TrimSpace(field)
		}

		// Handle "all" as special case - aggregate all available fields
		if len(fields) == 1 && fields[0] == "all" {
			fields = []string{"tech", "techs", "reason", "languages", "licenses", "dependencies", "git"}
		}

		// Validate fields
		validFields := map[string]bool{"tech": true, "techs": true, "reason": true, "languages": true, "licenses": true, "dependencies": true, "git": true}
		for _, field := range fields {
			if !validFields[field] {
				return nil, fmt.Errorf("invalid aggregate field: %s. Valid fields: tech, techs, reason, languages, licenses, dependencies, git, all", field)
			}
		}

		// Create aggregator and aggregate
		agg := aggregator.NewAggregator(fields)
		result = agg.Aggregate(payload.(*types.Payload))
	} else {
		result = payload
	}

	// Marshal to JSON
	if prettyPrint {
		return json.MarshalIndent(result, "", "  ")
	}
	return json.Marshal(result)
}

// writeOutput writes the JSON data to file or stdout
func writeOutput(jsonData []byte) {
	if settings.OutputFile != "" {
		err := os.WriteFile(settings.OutputFile, jsonData, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Results written to %s\n", settings.OutputFile)
	} else {
		fmt.Println(string(jsonData))
	}
}
