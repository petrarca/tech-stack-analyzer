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
	for _, child := range payload.Childs {
		attachComponentCodeStatsRecursive(child, analyzer)
	}
}

// attachComponentCodeStatsRecursive attaches code stats to a component and its children
func attachComponentCodeStatsRecursive(payload *types.Payload, analyzer codestats.Analyzer) {
	// Attach stats to current component
	if stats := analyzer.GetComponentStats(payload.ID); stats != nil {
		payload.CodeStats = stats
	}

	// Recursively attach to child components
	for _, child := range payload.Childs {
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
	settings *config.Settings
)

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan a project or file for technology stack",
	Long: `Scan analyzes a project directory or single file to detect technologies,
frameworks, databases, and services used in your codebase.

Examples:
  stack-analyzer scan /path/to/project
  stack-analyzer scan /path/to/pom.xml
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
	settings = config.LoadSettings()

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

	// Logging flags - use defaults from environment variables
	scanCmd.Flags().String("log-level", logLevel, "Log level: trace, debug, error, fatal")
	scanCmd.Flags().String("log-format", logFormat, "Log format: text or json")
	scanCmd.Flags().String("log-file", logFile, "Log file path (default: stderr)")
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
	absPath, isFile := resolveScanPath(args, logger)
	configureExcludePatterns(cmd)

	// Handle special case: -o - means stdout
	if settings.OutputFile == "-" {
		settings.OutputFile = ""
	}

	// Check for mutually exclusive flags
	if settings.Verbose && settings.Debug {
		logger.Error("Cannot use --verbose and --debug together. Choose one.")
		os.Exit(1)
	}

	// Auto-enable debug mode when trace flags are used without explicit output mode
	if (settings.TraceRules || settings.TraceTimings) && !settings.Verbose && !settings.Debug {
		settings.Debug = true
		logger.Debug("Auto-enabled --debug mode for trace output")
	}

	// Validate settings
	if err := settings.Validate(); err != nil {
		logger.Error("Invalid settings", "error", err)
		os.Exit(1)
	}

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
	codeStatsAnalyzer := codestats.NewAnalyzerWithPerComponent(!settings.NoCodeStats, settings.CodeStatsPerComponent)

	s, err := scanner.NewScannerWithOptionsAndLogger(scannerPath, settings.ExcludePatterns, settings.Verbose, settings.Debug, settings.TraceTimings, settings.TraceRules, codeStatsAnalyzer, logger)
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
			p.CodeStats = codeStatsAnalyzer.GetStats()

			// Attach per-component stats if enabled (post-processing)
			if codeStatsAnalyzer.IsPerComponentEnabled() {
				attachComponentCodeStats(p, codeStatsAnalyzer)
			}
		}
	}

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
	if settings.OutputFile != "" {
		err = os.WriteFile(settings.OutputFile, jsonData, 0644)
		if err != nil {
			logger.Error("Failed to write output file", "error", err)
			os.Exit(1)
		}
		// Always show confirmation to user (like curl -o)
		fmt.Fprintf(os.Stderr, "Results written to %s\n", settings.OutputFile)
	} else {
		logger.Debug("Outputting to stdout")
		fmt.Println(string(jsonData))
	}
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
