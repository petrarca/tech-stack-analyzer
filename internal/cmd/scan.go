package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/petrarca/tech-stack-analyzer/internal/config"
	gitpkg "github.com/petrarca/tech-stack-analyzer/internal/git"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/spf13/cobra"
)

var (
	settings       *config.Settings
	scanConfig     *config.ScanConfigFile
	scanConfigPath string
)

var scanCmd = &cobra.Command{
	Use:   "scan [path...]",
	Short: "Scan a project or file for technology stack",
	Long: `Scan analyzes a project directory, single file, or multiple directories to detect
technologies, frameworks, databases, and services used in your codebase.

When multiple paths are specified, they are scanned as a single unified project.
All paths must share a common parent directory (e.g., /myprojects/proj1 /myprojects/proj2).

Examples:
  stack-analyzer scan /path/to/project
  stack-analyzer scan /path/to/pom.xml
  stack-analyzer scan /path/to/proj1 /path/to/proj2
  stack-analyzer scan --config scan-config.yml /path/to/project
  stack-analyzer scan --config '{"scan":{"output":{"file":"$BUILD_DIR/scan-results.json"},"properties":{"build":"'$BUILD_NUMBER'"}}}' /path/to/project
  stack-analyzer scan --aggregate techs,languages /path/to/project
  stack-analyzer scan --aggregate all /path/to/project
  stack-analyzer scan --exclude vendor,node_modules /path/to/project
  stack-analyzer scan --exclude "**/__tests__/**" --exclude "*.log" /path/to/project`,
	Args: cobra.ArbitraryArgs,
	Run:  runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)

	settings = config.LoadSettingsFromEnvironment()

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

	scanCmd.Flags().StringVarP(&settings.OutputFile, "output", "o", outputFile, "Output file path (default: stack-analysis.json)")
	scanCmd.Flags().StringVar(&settings.Aggregate, "aggregate", aggregate, "Aggregate fields: tech,techs,languages,licenses,dependencies,git,all")
	scanCmd.Flags().BoolVar(&settings.PrettyPrint, "pretty", prettyPrint, "Pretty print JSON output")
	scanCmd.Flags().BoolVarP(&settings.Verbose, "verbose", "v", verbose, "Show progress with simple output")
	scanCmd.Flags().BoolVarP(&settings.Debug, "debug", "d", debug, "Show progress with tree structure (cannot be used with --verbose)")
	scanCmd.Flags().BoolVar(&settings.TraceTimings, "trace-timings", traceTimings, "Show timing information for each directory (requires --verbose or --debug)")
	scanCmd.Flags().BoolVar(&settings.TraceRules, "trace-rules", traceRules, "Show detailed rule matching information (requires --verbose or --debug)")
	scanCmd.Flags().StringSliceVar(&settings.ExcludePatterns, "exclude", settings.ExcludePatterns, "Patterns to exclude (supports glob patterns, can be specified multiple times)")
	scanCmd.Flags().StringSliceVar(&settings.FilterRules, "rules", settings.FilterRules, "Only use these rules (comma-separated tech names, e.g., c,cplusplus,nodejs - for debugging)")
	scanCmd.Flags().BoolVar(&settings.NoCodeStats, "no-code-stats", settings.NoCodeStats, "Disable code statistics (lines of code, comments, blanks, complexity)")
	scanCmd.Flags().IntVar(&settings.ComponentStatsDepth, "component-stats-depth", 0, "Include code_stats on components up to this tree depth in output (0=none, 1=top-level only, 2=two levels deep, ...)")
	scanCmd.Flags().IntVar(&settings.SubsystemDepth, "subsystem-depth", 0, "Produce subsystem_stats[] rolled up per depth-N path prefix (0=none, 1=top-level folders). Useful for large monorepos.")
	scanCmd.Flags().StringVar(&settings.RootID, "root-id", "", "Override random root ID for deterministic scans (e.g., 'my-project-2024')")
	scanCmd.Flags().String("log-level", logLevel, "Log level: trace, debug, error, fatal")
	scanCmd.Flags().String("log-format", logFormat, "Log format: text or json")
	scanCmd.Flags().String("log-file", logFile, "Log file path (default: stderr)")
	scanCmd.Flags().StringVar(&scanConfigPath, "config", "", "Scan configuration file path or inline JSON")
	scanCmd.Flags().StringSliceVar(&settings.OmitFields, "omit-fields", settings.OmitFields, "Fields to omit from output (e.g. reason,path,edges). Applies to all components recursively.")
	scanCmd.Flags().StringVar(&settings.AlsoAggregate, "also-aggregate", "", "Also produce an aggregate output alongside the full output. Suffix -agg is added to the output filename. (e.g. tech,techs,languages,dependencies,git)")
}

// configureLogging sets up logging based on command flags.
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

// parseLogLevel converts a string log level to slog.Level.
func parseLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error", "fatal":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level: %s", level)
	}
}

// configureExcludePatterns processes exclude patterns from command flags.
func configureExcludePatterns(cmd *cobra.Command) {
	excludeList, _ := cmd.Flags().GetStringSlice("exclude")
	for i, pattern := range excludeList {
		excludeList[i] = strings.TrimSpace(pattern)
	}
	settings.ExcludePatterns = excludeList
}

func runScan(cmd *cobra.Command, args []string) {
	logger := configureLogging(cmd)
	scanConfig = loadAndMergeScanConfig(logger)

	if len(args) > 1 {
		runMultiPathScan(args, cmd, logger)
	} else {
		runSinglePathScan(args, cmd, logger)
	}
}

// runSinglePathScan scans a single path and writes output.
// Note: runScanner already calls finalizeCodeStats and an initial computePrimaryTechs.
// enhanceSinglePayload may add config-injected techs, so primary_techs is recomputed after.
func runSinglePathScan(args []string, cmd *cobra.Command, logger *slog.Logger) {
	absPath, isFile := resolveScanPath(args, logger)
	configureExcludePatterns(cmd)
	setupScanSettings(logger)

	_, mergedConfig := loadAndMergeProjectConfig(absPath, logger)
	payload := runScanner(absPath, isFile, mergedConfig, logger)

	// Recompute primary_techs after enhancement so config-injected techs are included.
	enhanceSinglePayload(payload, mergedConfig)
	if p, ok := payload.(*types.Payload); ok {
		p.PrimaryTechs = computePrimaryTechsFromPayload(p)
	}

	generateAndWriteOutput(payload, logger)
}

// runMultiPathScan scans multiple directories as a single unified project.
func runMultiPathScan(args []string, cmd *cobra.Command, logger *slog.Logger) {
	commonParent, relPaths, absPaths := resolveMultiScanPaths(args, logger)
	configureExcludePatterns(cmd)
	setupScanSettings(logger)

	_, mergedConfig := loadAndMergeProjectConfig(commonParent, logger)
	loadAndMergeScanConfig(logger)

	rootID := settings.RootID
	if rootID == "" {
		rootID = gitpkg.GenerateRootIDFromMultiPaths(commonParent, relPaths)
	}

	fmt.Fprintf(os.Stderr, "Scanning %d paths under: %s\n", len(absPaths), commonParent)
	for _, p := range absPaths {
		fmt.Fprintf(os.Stderr, "  - %s\n", p)
	}

	logger.Debug("Multi-path scan",
		"common_parent", commonParent,
		"include_paths", relPaths,
		"root_id", rootID,
	)

	codeStatsAnalyzer := buildCodeStatsAnalyzer(settings)

	s, err := scanner.NewScannerWithOptionsAndLogger(
		commonParent,
		settings.ExcludePatterns,
		settings.Verbose,
		settings.Debug,
		settings.TraceTimings,
		settings.TraceRules,
		codeStatsAnalyzer,
		logger,
		rootID,
		mergedConfig,
	)
	if err != nil {
		logger.Error("Failed to create scanner", "error", err)
		os.Exit(1)
	}

	s.SetSubsystemDepth(settings.SubsystemDepth)
	s.SetSubsystemGroups(settings.SubsystemGroups)
	s.SetIncludePaths(relPaths)

	payload, err := s.Scan()
	if err != nil {
		logger.Error("Failed to scan", "error", err)
		os.Exit(1)
	}

	finalizeCodeStats(payload, codeStatsAnalyzer, settings.ComponentStatsDepth, s.ResolveSubsystemKeyFromPath, settings.SubsystemGroups)

	// Enhance before computing primary_techs so config techs are included.
	enhanceSinglePayload(payload, mergedConfig)
	payload.PrimaryTechs = computePrimaryTechsFromPayload(payload)

	generateAndWriteOutput(payload, logger)
}

// resolveScanPath resolves and validates the scan path from args.
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
	if err != nil {
		if os.IsNotExist(err) {
			logger.Error("Path does not exist", "path", absPath)
		} else {
			logger.Error("Cannot access path", "path", absPath, "error", err)
		}
		os.Exit(1)
	}
	return absPath, !fileInfo.IsDir()
}

// computeCommonParent returns the deepest common parent directory of the given absolute paths.
func computeCommonParent(paths []string) string {
	if len(paths) == 0 {
		return "."
	}
	if len(paths) == 1 {
		return paths[0]
	}

	splitPaths := make([][]string, len(paths))
	for i, p := range paths {
		splitPaths[i] = strings.Split(filepath.Clean(p), string(filepath.Separator))
	}

	common := splitPaths[0]
	for _, sp := range splitPaths[1:] {
		minLen := len(common)
		if len(sp) < minLen {
			minLen = len(sp)
		}
		matchLen := 0
		for j := 0; j < minLen; j++ {
			if common[j] != sp[j] {
				break
			}
			matchLen++
		}
		common = common[:matchLen]
	}

	if len(common) == 0 {
		return string(filepath.Separator)
	}

	result := strings.Join(common, string(filepath.Separator))
	if result == "" {
		return string(filepath.Separator)
	}
	return result
}

// isSystemRoot returns true if the path is a filesystem root or well-known system directory.
func isSystemRoot(path string) bool {
	cleaned := filepath.Clean(path)
	for _, root := range []string{"/", "/home", "/Users", "/tmp", "/var", "/opt", "/usr"} {
		if cleaned == root {
			return true
		}
	}
	return false
}

// resolveMultiScanPaths validates and resolves multiple scan paths.
// Returns the common parent, relative subfolder names, and absolute paths.
func resolveMultiScanPaths(args []string, logger *slog.Logger) (commonParent string, relPaths []string, absPaths []string) {
	absPaths = make([]string, 0, len(args))
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		abs, err := filepath.Abs(arg)
		if err != nil {
			logger.Error("Invalid path", "path", arg, "error", err)
			os.Exit(1)
		}
		info, err := os.Stat(abs)
		if os.IsNotExist(err) {
			logger.Error("Path does not exist", "path", abs)
			os.Exit(1)
		}
		if err != nil {
			logger.Error("Cannot access path", "path", abs, "error", err)
			os.Exit(1)
		}
		if !info.IsDir() {
			logger.Error("Multi-path scan requires directories, not files", "path", abs)
			os.Exit(1)
		}
		absPaths = append(absPaths, abs)
	}

	commonParent = computeCommonParent(absPaths)

	if isSystemRoot(commonParent) {
		fmt.Fprintf(os.Stderr, "Error: the specified paths share only %q as common parent.\n", commonParent)
		fmt.Fprintf(os.Stderr, "Multi-path scanning requires paths that share a project-level common parent.\n")
		os.Exit(1)
	}

	relPaths = make([]string, 0, len(absPaths))
	for _, abs := range absPaths {
		rel, err := filepath.Rel(commonParent, abs)
		if err != nil {
			logger.Error("Cannot compute relative path", "base", commonParent, "target", abs, "error", err)
			os.Exit(1)
		}
		relPaths = append(relPaths, rel)
	}

	// Warn when one input path is an ancestor of another.
	for i, a := range absPaths {
		for j, b := range absPaths {
			if i != j && strings.HasPrefix(b+string(filepath.Separator), a+string(filepath.Separator)) {
				fmt.Fprintf(os.Stderr, "Warning: %q is an ancestor of %q; only the explicitly listed sub-paths will be scanned under it.\n", a, b)
			}
		}
	}

	return commonParent, relPaths, absPaths
}
