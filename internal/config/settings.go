package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// Settings holds all scanner configuration
type Settings struct {
	// Output settings
	OutputFile  string
	PrettyPrint bool

	// Scan behavior
	ExcludePatterns []string
	Aggregate       string
	Verbose         bool
	Debug           bool
	TraceTimings    bool
	TraceRules      bool
	FilterRules     []string // Only use these rules (for debugging)
	NoCodeStats     bool     // Disable code statistics (enabled by default)

	// Logging
	LogLevel  logrus.Level
	LogFormat string // "text" or "json"
	LogFile   string // Optional: write logs to file instead of stderr
}

// DefaultSettings returns default configuration
func DefaultSettings() *Settings {
	return &Settings{
		OutputFile:      "stack-analysis.json",
		PrettyPrint:     true,
		ExcludePatterns: []string{},
		Aggregate:       "",
		Verbose:         false,
		Debug:           false,
		TraceTimings:    false,
		TraceRules:      false,
		FilterRules:     []string{},
		NoCodeStats:     false,             // Code stats enabled by default
		LogLevel:        logrus.ErrorLevel, // Changed from InfoLevel - only errors by default
		LogFormat:       "text",
		LogFile:         "", // Empty = stderr
	}
}

// LoadSettings creates settings from defaults and applies environment variable overrides
func LoadSettings() *Settings {
	settings := DefaultSettings()

	// Apply environment variable overrides
	if outputFile := os.Getenv("STACK_ANALYZER_OUTPUT"); outputFile != "" {
		settings.OutputFile = outputFile
	}

	if excludePatterns := os.Getenv("STACK_ANALYZER_EXCLUDE_DIRS"); excludePatterns != "" {
		settings.ExcludePatterns = strings.Split(excludePatterns, ",")
		for i, pattern := range settings.ExcludePatterns {
			settings.ExcludePatterns[i] = strings.TrimSpace(pattern)
		}
	}

	if aggregate := os.Getenv("STACK_ANALYZER_AGGREGATE"); aggregate != "" {
		settings.Aggregate = aggregate
	}

	if pretty := os.Getenv("STACK_ANALYZER_PRETTY"); pretty != "" {
		settings.PrettyPrint = strings.ToLower(pretty) == "true"
	}

	// Logging settings
	if logLevel := os.Getenv("STACK_ANALYZER_LOG_LEVEL"); logLevel != "" {
		if level, err := logrus.ParseLevel(logLevel); err == nil {
			settings.LogLevel = level
		}
	}

	if logFormat := os.Getenv("STACK_ANALYZER_LOG_FORMAT"); logFormat != "" {
		settings.LogFormat = logFormat
	}

	if logFile := os.Getenv("STACK_ANALYZER_LOG_FILE"); logFile != "" {
		settings.LogFile = logFile
	}

	if verbose := os.Getenv("STACK_ANALYZER_VERBOSE"); verbose != "" {
		settings.Verbose = strings.ToLower(verbose) == "true"
	}

	if debug := os.Getenv("STACK_ANALYZER_DEBUG"); debug != "" {
		settings.Debug = strings.ToLower(debug) == "true"
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

	return settings
}

// ConfigureLogger sets up the global logger based on settings
func (s *Settings) ConfigureLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(s.LogLevel)

	// Set log format
	if s.LogFormat == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// Set output destination
	if s.LogFile != "" {
		file, err := os.OpenFile(s.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// Fallback to stderr if file can't be opened
			fmt.Fprintf(os.Stderr, "Warning: Cannot open log file %s: %v\n", s.LogFile, err)
			logger.SetOutput(os.Stderr)
		} else {
			logger.SetOutput(file)
		}
	} else {
		// Default to stderr
		logger.SetOutput(os.Stderr)
	}

	return logger
}

// Validate checks if settings are valid
func (s *Settings) Validate() error {
	// TODO: Add validation logic
	// - Check if output directory exists/writable
	// - Validate aggregate fields
	// - Validate max depth is reasonable
	return nil
}
