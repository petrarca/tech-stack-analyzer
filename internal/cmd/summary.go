package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"log/slog"

	"github.com/petrarca/tech-stack-analyzer/internal/aggregator"
	"github.com/petrarca/tech-stack-analyzer/internal/codestats"
	gitpkg "github.com/petrarca/tech-stack-analyzer/internal/git"
	"github.com/petrarca/tech-stack-analyzer/internal/metadata"
	"github.com/petrarca/tech-stack-analyzer/internal/rules"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/spf13/cobra"
)

// langTechs lists tech IDs that represent programming languages and
// are already shown in the Languages section — they are suppressed from the
// Technologies "Also" list to avoid duplication.
var langTechs = map[string]bool{
	"c": true, "cplusplus": true, "csharp": true, "typescript": true,
	"javascript": true, "php": true, "python": true, "java": true,
	"ruby": true, "go": true, "rust": true, "swift": true, "kotlin": true,
}

var summaryCmd = &cobra.Command{
	Use:   "summary [path...]",
	Short: "Scan and print a human-readable codebase summary",
	Long: `Summary runs the same analysis as 'scan' but prints a concise text report
instead of JSON output. Useful for quick codebase introspection, onboarding,
and as input for LLM-based analysis.

The text report covers languages, technologies, components, code statistics,
and top-level directory structure.

Examples:
  stack-analyzer summary /path/to/project
  stack-analyzer summary --config scan-config.yml
  stack-analyzer summary --exclude vendor /path/to/project
  stack-analyzer summary --quiet /path/to/project`,
	Args: cobra.ArbitraryArgs,
	Run:  runSummary,
}

func init() {
	rootCmd.AddCommand(summaryCmd)

	// Scan behaviour flags — subset of scan flags, no output/format flags.
	summaryCmd.Flags().BoolVarP(&settings.Quiet, "quiet", "q", false, "Suppress scan progress output")
	summaryCmd.Flags().BoolVarP(&settings.Verbose, "verbose", "v", false, "Show scan progress with simple output")
	summaryCmd.Flags().BoolVarP(&settings.Debug, "debug", "d", false, "Show scan progress with tree structure")
	summaryCmd.Flags().StringSliceVar(&settings.ExcludePatterns, "exclude", settings.ExcludePatterns, "Patterns to exclude (supports glob patterns)")
	summaryCmd.Flags().BoolVar(&settings.NoCodeStats, "no-code-stats", settings.NoCodeStats, "Disable code statistics")
	summaryCmd.Flags().IntVar(&settings.ComponentStatsDepth, "component-stats-depth", 0, "Include code_stats on components up to this tree depth")
	summaryCmd.Flags().IntVar(&settings.SubsystemDepth, "subsystem-depth", 0, "Produce subsystem_stats per depth-N path prefix")
	summaryCmd.Flags().StringVar(&scanConfigPath, "config", "", "Scan configuration file path or inline JSON")
	summaryCmd.Flags().String("log-level", settings.LogLevel.String(), "Log level: trace, debug, error, fatal")
	summaryCmd.Flags().String("log-format", settings.LogFormat, "Log format: text or json")
	summaryCmd.Flags().String("log-file", settings.LogFile, "Log file path")
}

func runSummary(cmd *cobra.Command, args []string) {
	logger := configureLogging(cmd)
	scanConfig = loadAndMergeScanConfig(logger)

	if len(args) == 0 && scanConfig != nil && len(scanConfig.Scan.Paths) > 0 {
		args = scanConfig.Scan.Paths
	}

	// Enable per-component code stats (depth 1) by default for the component tree.
	if settings.ComponentStatsDepth == 0 {
		settings.ComponentStatsDepth = 1
	}

	var payload interface{}
	if len(args) > 1 {
		payload = runMultiPathSummary(args, cmd, logger)
	} else {
		payload = runSinglePathSummary(args, cmd, logger)
	}

	if p, ok := payload.(*types.Payload); ok {
		printSummary(p)
	}
}

func runSinglePathSummary(args []string, cmd *cobra.Command, logger *slog.Logger) interface{} {
	absPath, isFile := resolveScanPath(args, logger)
	configureExcludePatterns(cmd)
	setupScanSettings(logger)

	_, mergedConfig := loadAndMergeProjectConfig(absPath, logger)
	payload := runScanner(absPath, isFile, mergedConfig, logger, scanner.NewObservationCollector(absPath))

	enhanceSinglePayload(payload, mergedConfig)
	if p, ok := payload.(*types.Payload); ok {
		p.PrimaryTechs = computePrimaryTechsFromPayload(p)
		p.Ecosystems = aggregator.ComputeEcosystemsFromPayload(p)
	}
	return payload
}

func runMultiPathSummary(args []string, cmd *cobra.Command, logger *slog.Logger) interface{} {
	commonParent, relPaths, absPaths := resolveMultiScanPaths(args, logger)
	configureExcludePatterns(cmd)
	setupScanSettings(logger)

	_, mergedConfig := loadAndMergeProjectConfig(commonParent, logger)
	loadAndMergeScanConfig(logger)

	if !settings.Quiet {
		fmt.Fprintf(os.Stderr, "Scanning %d paths under: %s\n", len(absPaths), commonParent)
	}

	rootID := settings.RootID
	if rootID == "" {
		rootID = gitpkg.GenerateRootIDFromMultiPaths(commonParent, relPaths)
	}

	codeStatsAnalyzer := buildCodeStatsAnalyzer(settings)
	s, err := scanner.NewScannerWithOptionsAndLogger(
		commonParent, settings.ExcludePatterns,
		settings.Quiet, settings.Verbose, settings.Debug,
		settings.TraceTimings, settings.TraceRules,
		codeStatsAnalyzer, logger, rootID, mergedConfig,
	)
	if err != nil {
		logger.Error("Failed to create scanner", "error", err)
		os.Exit(1)
	}
	s.SetSubsystemDepth(settings.SubsystemDepth)
	s.SetSubsystemGroups(settings.SubsystemGroups)
	s.SetIncludePaths(relPaths)
	s.SetObservationCollector(scanner.NewObservationCollector(commonParent))

	payload, err := s.Scan()
	if err != nil {
		logger.Error("Failed to scan", "error", err)
		os.Exit(1)
	}

	finalizeCodeStats(payload, codeStatsAnalyzer, settings.ComponentStatsDepth, s.ResolveSubsystemKeyFromPath, settings.SubsystemGroups)
	enhanceSinglePayload(payload, mergedConfig)
	payload.PrimaryTechs = computePrimaryTechsFromPayload(payload)
	payload.Ecosystems = aggregator.ComputeEcosystemsFromPayload(payload)

	return payload
}

// printSummary renders the payload as a human-readable text report to stdout.
func printSummary(p *types.Payload) {
	techNames := loadTechDisplayNames()

	fmt.Println("Codebase Summary")
	fmt.Println(strings.Repeat("=", 70))

	printMetadata(p)
	printCodeStats(p)
	printLanguages(p)
	printTechnologies(p, techNames)
	printStructure(p)
	printSubsystems(p)
	printObservations(p)
}

func printMetadata(p *types.Payload) {
	meta, ok := p.Metadata.(*metadata.ScanMetadata)
	if !ok {
		return
	}
	fmt.Printf("\n  Scan path:      %s\n", meta.ScanPath)
	fmt.Printf("  Components:     %d\n", meta.ComponentCount)
	fmt.Printf("  Languages:      %d\n", meta.LanguageCount)
	fmt.Printf("  Technologies:   %d\n", meta.TechsCount)
	if meta.DurationMs > 0 {
		fmt.Printf("  Scan time:      %.1fs\n", float64(meta.DurationMs)/1000)
	}
}

func printCodeStats(p *types.Payload) {
	cs, ok := codeStatsFromPayload(p)
	if !ok {
		return
	}
	fmt.Println()
	fmt.Println("Code Statistics")
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("  %-20s  %10s  %10s  %10s  %10s\n", "Type", "Files", "Code LoC", "Comments", "Blanks")
	printTypeBucketRow("Programming", cs.ByType.Programming)
	printTypeBucketRow("Markup", cs.ByType.Markup)
	printTypeBucketRow("Data", cs.ByType.Data)
	printTypeBucketRow("Prose", cs.ByType.Prose)
	if cs.Unanalyzed.Total.Files > 0 {
		fmt.Printf("  %-20s  %10s  %10s  (lines only)\n",
			"Other (unanalyzed)", fmtInt(int64(cs.Unanalyzed.Total.Files)), fmtInt(cs.Unanalyzed.Total.Lines))
	}
	fmt.Printf("  %-20s  %10s  %10s  %10s  %10s\n",
		"Total (analyzed)", fmtInt(int64(cs.Total.Files)), fmtInt(cs.Total.Code), fmtInt(cs.Total.Comments), fmtInt(cs.Total.Blanks))
}

func printTypeBucketRow(label string, tb *codestats.TypeBucket) {
	if tb == nil || tb.Total.Files == 0 {
		return
	}
	fmt.Printf("  %-20s  %10s  %10s  %10s  %10s\n",
		label, fmtInt(int64(tb.Total.Files)), fmtInt(tb.Total.Code), fmtInt(tb.Total.Comments), fmtInt(tb.Total.Blanks))
}

func printLanguages(p *types.Payload) {
	cs, ok := codeStatsFromPayload(p)
	if !ok {
		return
	}
	fmt.Println()
	fmt.Println("Languages (top 15 by Code LoC)")
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("  %-25s  %10s  %10s\n", "Language", "Files", "Code LoC")

	langs := cs.Analyzed.ByLanguage
	limit := min(15, len(langs))
	for i := 0; i < limit; i++ {
		l := langs[i]
		fmt.Printf("  %-25s  %10s  %10s\n", l.Language, fmtInt(int64(l.Files)), fmtInt(l.Code))
	}
	if len(langs) > limit {
		fmt.Printf("  ... and %d more\n", len(langs)-limit)
	}
	if len(p.PrimaryLanguages) > 0 {
		parts := make([]string, 0, len(p.PrimaryLanguages))
		for _, pl := range p.PrimaryLanguages {
			parts = append(parts, fmt.Sprintf("%s (%.0f%%)", pl.Language, pl.Pct*100))
		}
		fmt.Printf("\n  Primary: %s\n", strings.Join(parts, ", "))
	}
}

func printTechnologies(p *types.Payload, techNames map[string]string) {
	if len(p.Techs) == 0 {
		return
	}
	fmt.Println()
	fmt.Println("Technologies")
	fmt.Println(strings.Repeat("-", 70))

	if len(p.PrimaryTechs) > 0 {
		names := make([]string, 0, len(p.PrimaryTechs))
		for _, t := range p.PrimaryTechs {
			names = append(names, displayName(t, techNames))
		}
		fmt.Printf("  Primary:  %s\n", strings.Join(names, ", "))
	}

	primarySet := make(map[string]bool, len(p.PrimaryTechs))
	for _, t := range p.PrimaryTechs {
		primarySet[t] = true
	}
	secondary := make([]string, 0)
	for _, t := range p.Techs {
		if !primarySet[t] && !langTechs[t] {
			secondary = append(secondary, displayName(t, techNames))
		}
	}
	if len(secondary) > 0 {
		sort.Strings(secondary)
		fmt.Printf("  Also:     %s\n", strings.Join(secondary, ", "))
	}

	if len(p.Ecosystems) > 0 {
		parts := make([]string, 0, len(p.Ecosystems))
		for _, e := range p.Ecosystems {
			parts = append(parts, fmt.Sprintf("%s (%d)", e.Ecosystem, e.Components))
		}
		fmt.Printf("  Ecosystems: %s\n", strings.Join(parts, ", "))
	}
}

func printStructure(p *types.Payload) {
	if len(p.Children) == 0 {
		return
	}
	fmt.Println()
	fmt.Println("Component Tree")
	fmt.Println(strings.Repeat("-", 70))

	type dirInfo struct {
		name       string
		components int
		files      int
		codeLines  int64
	}

	dirMap := make(map[string]*dirInfo)
	for _, child := range p.Children {
		dir := topLevelDir(child)
		if dir == "" {
			dir = "(root)"
		}
		info, exists := dirMap[dir]
		if !exists {
			info = &dirInfo{name: dir}
			dirMap[dir] = info
		}
		info.components += 1 + countDescendants(child)
		if cs, ok := codeStatsFromPayload(child); ok {
			info.files += cs.Total.Files
			info.codeLines += cs.Total.Code
		}
	}

	dirs := make([]*dirInfo, 0, len(dirMap))
	for _, info := range dirMap {
		dirs = append(dirs, info)
	}
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].components > dirs[j].components
	})

	fmt.Printf("  %-35s  %10s  %10s  %10s\n", "Directory", "Components", "Files", "Code LoC")
	for _, d := range dirs {
		fmt.Printf("  %-35s  %10d  %10s  %10s\n",
			d.name, d.components, fmtInt(int64(d.files)), fmtInt(d.codeLines))
	}
}

func printSubsystems(p *types.Payload) {
	if len(p.SubsystemStats) == 0 {
		return
	}
	fmt.Println()
	fmt.Println("Subsystems")
	fmt.Println(strings.Repeat("-", 70))
	for _, ss := range p.SubsystemStats {
		if cs, ok := ss.CodeStats.(*codestats.CodeStats); ok {
			fmt.Printf("  %-25s  %4d components  %s files  %s LoC\n",
				ss.Path, ss.ComponentCount, fmtInt(int64(cs.Total.Files)), fmtInt(cs.Total.Code))
		} else {
			fmt.Printf("  %-25s  %4d components\n", ss.Path, ss.ComponentCount)
		}
		if ss.Description != "" {
			fmt.Printf("    %s\n", ss.Description)
		}
	}
}

// --- helpers ---

// topLevelDir extracts the first path segment from a component's source directory.
// E.g. "/frontend/app/src" -> "frontend"
func topLevelDir(p *types.Payload) string {
	sd := p.SourceDir
	if sd == "" || sd == "/" {
		if len(p.Path) > 0 {
			sd = p.Path[0]
		}
	}
	if sd == "" || sd == "/" {
		return ""
	}
	trimmed := strings.TrimPrefix(sd, "/")
	if idx := strings.Index(trimmed, "/"); idx >= 0 {
		return trimmed[:idx]
	}
	return trimmed
}

// countDescendants recursively counts all descendants in the component tree.
func countDescendants(p *types.Payload) int {
	count := len(p.Children)
	for _, child := range p.Children {
		count += countDescendants(child)
	}
	return count
}

// fmtInt formats an integer with thousands separators (e.g. 1234567 -> "1,234,567").
func fmtInt(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1_000_000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d,%03d,%03d", n/1_000_000, (n/1000)%1000, n%1000)
}

// displayName returns the human-readable name for a tech ID, falling back to the ID itself.
func displayName(tech string, names map[string]string) string {
	if name, ok := names[tech]; ok {
		return name
	}
	return tech
}

// loadTechDisplayNames builds a map from tech ID to display name from embedded rules.
func loadTechDisplayNames() map[string]string {
	m := make(map[string]string)
	allRules, err := rules.LoadEmbeddedRules()
	if err != nil {
		return m
	}
	for _, r := range allRules {
		if r.Name != "" {
			m[r.Tech] = r.Name
		}
	}
	return m
}
