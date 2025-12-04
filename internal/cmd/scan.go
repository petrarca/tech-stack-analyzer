package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/aggregator"
	"github.com/petrarca/tech-stack-analyzer/internal/codestats"
	"github.com/petrarca/tech-stack-analyzer/internal/config"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

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
	scanCmd.Flags().StringSliceVar(&settings.ExcludeDirs, "exclude", settings.ExcludeDirs, "Patterns to exclude (supports glob patterns, can be specified multiple times)")

	// Rule filtering for debugging
	scanCmd.Flags().StringSliceVar(&settings.FilterRules, "rules", settings.FilterRules, "Only use these rules (comma-separated tech names, e.g., c,cplusplus,nodejs - for debugging)")

	// Code statistics flag (enabled by default)
	scanCmd.Flags().BoolVar(&settings.NoCodeStats, "no-code-stats", settings.NoCodeStats, "Disable code statistics (lines of code, comments, blanks, complexity)")

	// Logging flags - use defaults from environment variables
	scanCmd.Flags().String("log-level", logLevel, "Log level: trace, debug, error, fatal")
	scanCmd.Flags().String("log-format", logFormat, "Log format: text or json")
	scanCmd.Flags().String("log-file", logFile, "Log file path (default: stderr)")
}

func runScan(cmd *cobra.Command, args []string) {
	// Get logging flags and configure logger
	logLevel, _ := cmd.Flags().GetString("log-level")
	logFormat, _ := cmd.Flags().GetString("log-format")
	logFile, _ := cmd.Flags().GetString("log-file")

	// Update settings with flag values
	if level, err := logrus.ParseLevel(logLevel); err == nil {
		settings.LogLevel = level
	}
	settings.LogFormat = logFormat
	settings.LogFile = logFile

	// Configure logger
	logger := settings.ConfigureLogger()

	// Get path from args or use current directory
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Clean and validate project path
	path = strings.TrimSpace(path)
	absPath, err := filepath.Abs(path)
	if err != nil {
		logger.WithError(err).Fatal("Invalid path")
	}

	// Check if path exists and determine if it's a file or directory
	fileInfo, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		logger.WithField("path", absPath).Fatal("Path does not exist")
	}
	isFile := !fileInfo.IsDir()

	// Get the exclude flag value (already parsed as StringSlice by cobra)
	excludeList, _ := cmd.Flags().GetStringSlice("exclude")

	// Trim whitespace from each pattern
	for i, pattern := range excludeList {
		excludeList[i] = strings.TrimSpace(pattern)
	}

	// Update settings with actual flag values
	settings.ExcludeDirs = excludeList

	// Handle special case: -o - means stdout
	if settings.OutputFile == "-" {
		settings.OutputFile = ""
	}

	// Check for mutually exclusive flags
	if settings.Verbose && settings.Debug {
		logger.Fatal("Cannot use --verbose and --debug together. Choose one.")
	}

	// Auto-enable debug mode when trace flags are used without explicit output mode
	if (settings.TraceRules || settings.TraceTimings) && !settings.Verbose && !settings.Debug {
		settings.Debug = true
		logger.Debug("Auto-enabled --debug mode for trace output")
	}

	// Validate settings
	if err := settings.Validate(); err != nil {
		logger.WithError(err).Fatal("Invalid settings")
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

	logger.WithFields(logrus.Fields{
		"path":         scannerPath,
		"exclude_dirs": settings.ExcludeDirs,
		"code_stats":   !settings.NoCodeStats,
	}).Debug("Initializing scanner")

	// Create code stats analyzer (enabled by default, disabled with --no-code-stats)
	codeStatsAnalyzer := codestats.NewAnalyzer(!settings.NoCodeStats)

	s, err := scanner.NewScannerWithOptionsAndLogger(scannerPath, settings.ExcludeDirs, settings.Verbose, settings.Debug, settings.TraceTimings, settings.TraceRules, codeStatsAnalyzer, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create scanner")
	}

	// Scan project or file
	var payload interface{}
	if isFile {
		logger.WithField("file", absPath).Debug("Scanning file")
		payload, err = s.ScanFile(filepath.Base(absPath))
	} else {
		logger.WithField("directory", absPath).Debug("Scanning directory")
		payload, err = s.Scan()
	}

	if err != nil {
		logger.WithError(err).Fatal("Failed to scan")
	}

	// Attach code stats to payload if enabled
	if codeStatsAnalyzer.IsEnabled() {
		if p, ok := payload.(*types.Payload); ok {
			p.CodeStats = codeStatsAnalyzer.GetStats()
		}
	}

	// Generate output (aggregated or full payload)
	logger.WithFields(logrus.Fields{
		"aggregate":    settings.Aggregate,
		"pretty_print": settings.PrettyPrint,
	}).Debug("Generating output")

	jsonData, err := generateOutput(payload, settings.Aggregate, settings.PrettyPrint)
	if err != nil {
		logger.WithError(err).Fatal("Failed to marshal JSON")
	}

	// Write output
	if settings.OutputFile != "" {
		err = os.WriteFile(settings.OutputFile, jsonData, 0644)
		if err != nil {
			logger.WithError(err).Fatal("Failed to write output file")
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
