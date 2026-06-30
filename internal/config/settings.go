package config

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	"log/slog"

	"github.com/petrarca/tech-stack-analyzer/internal/store"
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
	Quiet                    bool
	Verbose                  bool
	Debug                    bool
	TraceTimings             bool
	TraceRules               bool
	FilterRules              []string                  // Only use these rules (for debugging)
	NoCodeStats              bool                      // Disable code statistics (enabled by default)
	ComponentStatsDepth      int                       // Collect and include code_stats on components up to this tree depth (0=none, 1=top-level, 2=two levels)
	SubsystemDepth           int                       // Collect and include subsystem_stats rolled up per depth-N path prefix (0=none, 1=top-level folders)
	SubsystemGroups          map[string]SubsystemGroup // Named subsystem groups overriding depth-based splitting (from config file)
	RootID                   string                    // Override random root ID for deterministic scans
	PrimaryLanguageThreshold float64                   // Minimum percentage for primary languages (default 0.05 = 5%)
	UseLockFiles             bool                      // Use lock files for dependency resolution (default true)
	DependencyGraph          string                    // Package-to-package edge emission: "off" (default), "direct", or "full"
	UseDepsDev               bool                      // Enable online deps.dev dependency-graph resolution (default false)
	DepsDevEndpoint          string                    // Base URL for deps.dev; empty = public. Override for a compatible facade or mirror
	UseMavenCentral          bool                      // Enable the public Maven Central fallback for Maven BOM/parent POM fetch (default false)
	MavenGraphSource         string                    // Maven transitive-graph source override: "" (follow --deps-dev) | "repo" | "deps-dev" | "none"
	MavenLocalRepo           bool                      // Read the local ~/.m2 repository for Maven BOM/parent POMs (offline; reads outside the scanned tree)
	MavenLocalRepoDir        string                    // Override the local Maven repo path; empty = MAVEN_REPO_LOCAL / MAVEN_OPTS / ~/.m2/repository
	MavenRepoURL             string                    // Remote Maven repository base for BOM/parent POM fetch (e.g. internal JFrog); always used when set (no extra flag needed)
	MavenSettings            string                    // Path to a Maven settings.xml (repos+credentials); empty = ~/.m2/settings.xml. Per-scan override for projects with their own settings
	MavenRepoToken           string                    // Token for an authenticated remote Maven repo; sourced from the environment, never persisted
	MavenRepoUser            string                    // Username for Basic auth against the remote Maven repo; sourced from the environment
	OmitFields               []string                  // Fields to omit from full output (e.g. "reason", "path", "edges")
	AlsoAggregate            string                    // Also produce an aggregate output alongside the full output (e.g. "tech,techs,languages")
	SBOM                     bool                      // Emit an SBOM as the primary output instead of the scan tree
	AlsoSBOM                 bool                      // Also write an SBOM alongside the scan output
	SBOMFormat               string                    // SBOM format: "cyclonedx" (default) or "spdx"
	ResolveCurrency          bool                      // Resolve dependency currency (deps.dev) and write a {out}.currency.json (opt-in; network)
	CurrencyCache            string                    // Override the currency cache DB path; empty = STACK_ANALYZER_CURRENCY_CACHE or OS cache dir
	CurrencyTTLHours         int                       // Per-entry currency cache TTL in hours (default 24)

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
		ComponentStatsDepth:      0,               // Per-component stats disabled by default
		SubsystemDepth:           0,               // Subsystem rollup disabled by default
		LogLevel:                 slog.LevelError, // Changed from InfoLevel - only errors by default
		LogFormat:                "text",
		LogFile:                  "",
		PrimaryLanguageThreshold: 0.05, // 5% threshold for primary languages
		UseLockFiles:             true, // Lock files enabled by default
		CurrencyTTLHours:         24,   // Currency cache entries valid for 24h by default
		// DependencyGraph left empty ("") = off. Empty (not "off") is the zero
		// value so a scanner-config.yml value can merge in; ParseDependencyGraphMode
		// maps empty -> off at the point of use.
	}
}

// LoadSettingsFromEnvironment loads settings from environment variables
func LoadSettingsFromEnvironment() *Settings {
	settings := DefaultSettings()

	applyStringEnv(settings)
	applyBoolEnv(settings)
	applyIntEnv(settings)
	applyListEnv(settings)

	// Log level needs parsing/validation (invalid values keep the default).
	if logLevel := os.Getenv("STACK_ANALYZER_LOG_LEVEL"); logLevel != "" {
		if level, err := parseLogLevel(logLevel); err == nil {
			settings.LogLevel = level
		}
	}

	return settings
}

// applyStringEnv overrides plain string settings from their env vars.
func applyStringEnv(s *Settings) {
	strs := []struct {
		env   string
		field *string
	}{
		{"STACK_ANALYZER_OUTPUT", &s.OutputFile},
		{"STACK_ANALYZER_AGGREGATE", &s.Aggregate},
		// Currency cache is also overridable via --currency-cache (flag wins).
		{store.EnvCachePath, &s.CurrencyCache},
		{"STACK_ANALYZER_LOG_FORMAT", &s.LogFormat},
		{"STACK_ANALYZER_LOG_FILE", &s.LogFile},
	}
	for _, e := range strs {
		if v := os.Getenv(e.env); v != "" {
			*e.field = v
		}
	}
}

// applyBoolEnv overrides boolean settings. UseLockFiles is opt-out ("false"
// disables); every other flag is opt-in ("true" enables).
func applyBoolEnv(s *Settings) {
	bools := []struct {
		env   string
		field *bool
	}{
		{"STACK_ANALYZER_PRETTY", &s.PrettyPrint},
		{"STACK_ANALYZER_VERBOSE", &s.Verbose},
		{"STACK_ANALYZER_DEBUG", &s.Debug},
		{"STACK_ANALYZER_NO_CODE_STATS", &s.NoCodeStats},
		{"STACK_ANALYZER_TRACE_TIMINGS", &s.TraceTimings},
		{"STACK_ANALYZER_TRACE_RULES", &s.TraceRules},
	}
	for _, e := range bools {
		if v := os.Getenv(e.env); v != "" {
			*e.field = strings.ToLower(v) == "true"
		}
	}
	if v := os.Getenv("STACK_ANALYZER_USE_LOCK_FILES"); v != "" {
		s.UseLockFiles = strings.ToLower(v) != "false"
	}
}

// applyIntEnv overrides integer settings, ignoring unparseable values.
func applyIntEnv(s *Settings) {
	ints := []struct {
		env   string
		field *int
	}{
		{"STACK_ANALYZER_COMPONENT_STATS_DEPTH", &s.ComponentStatsDepth},
		{"STACK_ANALYZER_SUBSYSTEM_DEPTH", &s.SubsystemDepth},
	}
	for _, e := range ints {
		if v := os.Getenv(e.env); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				*e.field = n
			}
		}
	}
}

// applyListEnv overrides comma-separated list settings, trimming each element.
func applyListEnv(s *Settings) {
	lists := []struct {
		env   string
		field *[]string
	}{
		{"STACK_ANALYZER_FILTER_RULES", &s.FilterRules},
		{"STACK_ANALYZER_EXCLUDE", &s.ExcludePatterns},
	}
	for _, e := range lists {
		if v := os.Getenv(e.env); v != "" {
			*e.field = splitTrim(v)
		}
	}
}

// splitTrim splits a comma-separated env value and trims whitespace.
func splitTrim(value string) []string {
	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
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
	if s.Verbose && s.Debug {
		return fmt.Errorf("cannot use both --verbose and --debug flags")
	}
	if err := s.validateEnums(); err != nil {
		return err
	}
	if err := s.validateURLs(); err != nil {
		return err
	}
	if s.ResolveCurrency && s.CurrencyTTLHours <= 0 {
		return fmt.Errorf("invalid currency-ttl %d: must be a positive number of hours", s.CurrencyTTLHours)
	}
	return s.validateAggregate()
}

// validateEnums checks the fixed-vocabulary options (dependency-graph mode and
// SBOM format).
func (s *Settings) validateEnums() error {
	if s.DependencyGraph != "" {
		switch s.DependencyGraph {
		case "off", "direct", "full":
		default:
			return fmt.Errorf("invalid dependency-graph mode '%s'. Valid values: off, direct, full", s.DependencyGraph)
		}
	}
	if s.SBOMFormat != "" {
		switch strings.ToLower(s.SBOMFormat) {
		case "cyclonedx", "spdx":
		default:
			return fmt.Errorf("invalid sbom-format '%s'. Valid values: cyclonedx, spdx", s.SBOMFormat)
		}
	}
	return nil
}

// validateURLs checks that optional URL settings are well-formed http(s) URLs.
func (s *Settings) validateURLs() error {
	urls := []struct {
		flag  string
		value string
	}{
		{"deps-dev-endpoint", s.DepsDevEndpoint},
		{"maven-repo-url", s.MavenRepoURL},
	}
	for _, u := range urls {
		if u.value == "" {
			continue
		}
		if parsed, err := url.Parse(u.value); err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("invalid %s '%s': must be an http(s) URL", u.flag, u.value)
		}
	}
	return nil
}

// validateAggregate checks that each comma-separated aggregate field is known.
func (s *Settings) validateAggregate() error {
	if s.Aggregate == "" {
		return nil
	}
	validFields := map[string]bool{
		"tech": true, "techs": true, "reason": true,
		"languages": true, "licenses": true,
		"dependencies": true, "git": true,
		"components": true, "all": true,
	}
	for _, field := range strings.Split(s.Aggregate, ",") {
		if !validFields[strings.TrimSpace(field)] {
			return fmt.Errorf("invalid aggregate field '%s'. Valid fields: tech, techs, reason, languages, licenses, dependencies, git, all", strings.TrimSpace(field))
		}
	}
	return nil
}
