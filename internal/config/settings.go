package config

import (
	"fmt"
	"io"
	"os"
	"strings"

	"log/slog"
)

// Settings holds all scanner configuration
// Field names match ScanOptions for reflection-based merging
type Settings struct {
	// Output settings
	OutputFile  string
	PrettyPrint bool
	Aggregate   string

	// Scan behavior
	ExcludePatterns          []string
	Verbose                  bool
	Debug                    bool
	TraceTimings             bool
	TraceRules               bool
	FilterRules              []string // Only use these rules (for debugging)
	NoCodeStats              bool     // Disable code statistics (enabled by default)
	CodeStatsPerComponent    bool     // Enable per-component code statistics (disabled by default)
	RootID                   string   // Override random root ID for deterministic scans
	PrimaryLanguageThreshold float64  // Minimum percentage for primary languages (default 0.05 = 5%)

	// Logging
	LogLevel  slog.Level
	LogFormat string // "text" or "json"
	LogFile   string // Optional: write logs to file instead of stderr
}

// DefaultSettings returns default configuration
func DefaultSettings() *Settings {
	return &Settings{
		OutputFile:               "stack-analysis.json",
		PrettyPrint:              true,
		Aggregate:                "",
		ExcludePatterns:          []string{},
		Verbose:                  false,
		Debug:                    false,
		TraceTimings:             false,
		TraceRules:               false,
		FilterRules:              []string{},
		NoCodeStats:              false,           // Code stats enabled by default
		CodeStatsPerComponent:    false,           // Per-component code stats disabled by default
		LogLevel:                 slog.LevelError, // Changed from InfoLevel - only errors by default
		LogFormat:                "text",
		LogFile:                  "",
		PrimaryLanguageThreshold: 0.05, // 5% threshold for primary languages
	}
}

// LoadSettingsFromEnvironment loads settings from environment variables
func LoadSettingsFromEnvironment() *Settings {
	settings := DefaultSettings()

	// Override with environment variables if set
	if outputFile := os.Getenv("STACK_ANALYZER_OUTPUT"); outputFile != "" {
		settings.OutputFile = outputFile
	}

	if pretty := os.Getenv("STACK_ANALYZER_PRETTY"); pretty != "" {
		settings.PrettyPrint = strings.ToLower(pretty) == "true"
	}

	if aggregate := os.Getenv("STACK_ANALYZER_AGGREGATE"); aggregate != "" {
		settings.Aggregate = aggregate
	}

	if verbose := os.Getenv("STACK_ANALYZER_VERBOSE"); verbose != "" {
		settings.Verbose = strings.ToLower(verbose) == "true"
	}

	if debug := os.Getenv("STACK_ANALYZER_DEBUG"); debug != "" {
		settings.Debug = strings.ToLower(debug) == "true"
	}

	if noCodeStats := os.Getenv("STACK_ANALYZER_NO_CODE_STATS"); noCodeStats != "" {
		settings.NoCodeStats = strings.ToLower(noCodeStats) == "true"
	}

	if componentStats := os.Getenv("STACK_ANALYZER_COMPONENT_CODE_STATS"); componentStats != "" {
		settings.CodeStatsPerComponent = strings.ToLower(componentStats) == "true"
	}

	if traceTimings := os.Getenv("STACK_ANALYZER_TRACE_TIMINGS"); traceTimings != "" {
		settings.TraceTimings = strings.ToLower(traceTimings) == "true"
	}

	if traceRules := os.Getenv("STACK_ANALYZER_TRACE_RULES"); traceRules != "" {
		settings.TraceRules = strings.ToLower(traceRules) == "true"
	}

	if filterRules := os.Getenv("STACK_ANALYZER_FILTER_RULES"); filterRules != "" {
		settings.FilterRules = strings.Split(filterRules, ",")
		for i, rule := range settings.FilterRules {
			settings.FilterRules[i] = strings.TrimSpace(rule)
		}
	}

	if excludes := os.Getenv("STACK_ANALYZER_EXCLUDE"); excludes != "" {
		settings.ExcludePatterns = strings.Split(excludes, ",")
		for i, exclude := range settings.ExcludePatterns {
			settings.ExcludePatterns[i] = strings.TrimSpace(exclude)
		}
	}

	if logLevel := os.Getenv("STACK_ANALYZER_LOG_LEVEL"); logLevel != "" {
		if level, err := parseLogLevel(logLevel); err == nil {
			settings.LogLevel = level
		}
	}

	if logFormat := os.Getenv("STACK_ANALYZER_LOG_FORMAT"); logFormat != "" {
		settings.LogFormat = logFormat
	}

	if logFile := os.Getenv("STACK_ANALYZER_LOG_FILE"); logFile != "" {
		settings.LogFile = logFile
	}

	return settings
}

// parseLogLevel converts string log level to slog.Level
func parseLogLevel(level string) (slog.Level, error) {
	switch strings.ToUpper(level) {
	case "TRACE":
		return slog.LevelDebug - 4, nil
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "WARN", "WARNING":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	case "FATAL":
		return slog.LevelError + 4, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level: %s", level)
	}
}

// ConfigureLogger sets up the logger based on settings
func (s *Settings) ConfigureLogger() *slog.Logger {
	var handler slog.Handler

	// Set output destination
	var output io.Writer = os.Stderr
	if s.LogFile != "" {
		file, err := os.OpenFile(s.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// Fallback to stderr if file can't be opened
			fmt.Fprintf(os.Stderr, "Warning: Cannot open log file %s: %v\n", s.LogFile, err)
			output = os.Stderr
		} else {
			output = file
		}
	}

	// Configure handler based on format
	opts := &slog.HandlerOptions{
		Level: s.LogLevel,
	}

	switch strings.ToLower(s.LogFormat) {
	case "json":
		handler = slog.NewJSONHandler(output, opts)
	default:
		handler = slog.NewTextHandler(output, opts)
	}

	return slog.New(handler)
}

// Validate checks if the settings are valid
func (s *Settings) Validate() error {
	// Check for mutually exclusive flags
	if s.Verbose && s.Debug {
		return fmt.Errorf("cannot use both --verbose and --debug flags")
	}

	// Validate aggregate fields if specified
	if s.Aggregate != "" {
		validFields := map[string]bool{
			"tech": true, "techs": true, "reason": true,
			"languages": true, "licenses": true,
			"dependencies": true, "git": true, "all": true,
		}

		fields := strings.Split(s.Aggregate, ",")
		for _, field := range fields {
			field = strings.TrimSpace(field)
			if !validFields[field] {
				return fmt.Errorf("invalid aggregate field '%s'. Valid fields: tech, techs, reason, languages, licenses, dependencies, git, all", field)
			}
		}
	}

	return nil
}
