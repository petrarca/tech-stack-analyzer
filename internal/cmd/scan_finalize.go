package cmd

import (
	"sort"

	"github.com/petrarca/tech-stack-analyzer/internal/codestats"
	"github.com/petrarca/tech-stack-analyzer/internal/config"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// subsystemKeyResolver maps a component's depth-1 path prefix to a subsystem key.
type subsystemKeyResolver func(depthOnePath string) string

// finalizeCodeStats attaches global, per-component, and subsystem code stats to the payload.
func finalizeCodeStats(payload *types.Payload, analyzer codestats.Analyzer, statsDepth int, resolveKey subsystemKeyResolver, groups map[string]config.SubsystemGroup) {
	if !analyzer.IsEnabled() {
		return
	}
	stats := analyzer.GetStats()
	payload.CodeStats = stats

	if stats != nil && stats.ByType.Programming != nil && stats.ByType.Programming.Metrics != nil {
		payload.PrimaryLanguages = convertPrimaryLanguages(stats.ByType.Programming.Metrics.PrimaryLanguages)
	}

	attachComponentCodeStats(payload, analyzer, statsDepth)
	attachSubsystemStats(payload, analyzer, resolveKey, groups)
}

// attachComponentCodeStats attaches per-component code stats to child components up to statsDepth.
// Root keeps the global stats; only children at depth 1..statsDepth receive per-component stats.
// statsDepth=0 is a no-op.
func attachComponentCodeStats(payload *types.Payload, analyzer codestats.Analyzer, statsDepth int) {
	if statsDepth <= 0 {
		return
	}
	for _, child := range payload.Children {
		attachComponentCodeStatsRecursive(child, analyzer, 1, statsDepth)
	}
}

// attachComponentCodeStatsRecursive attaches stats to components at depth <= maxDepth.
func attachComponentCodeStatsRecursive(payload *types.Payload, analyzer codestats.Analyzer, depth, maxDepth int) {
	if depth <= maxDepth {
		key := payload.ComponentPath()
		if stats := analyzer.GetComponentStats(key); stats != nil {
			payload.CodeStats = stats
		}
	}
	if depth < maxDepth {
		for _, child := range payload.Children {
			attachComponentCodeStatsRecursive(child, analyzer, depth+1, maxDepth)
		}
	}
}

// attachSubsystemStats populates payload.SubsystemStats from the analyzer's subsystem buckets.
// Each entry is one subsystem key with its rolled-up code stats and component count.
// This is a no-op when subsystem tracking is disabled.
func attachSubsystemStats(payload *types.Payload, analyzer codestats.Analyzer, resolve subsystemKeyResolver, groups map[string]config.SubsystemGroup) {
	sa, ok := analyzer.(codestats.SubsystemAnalyzer)
	if !ok {
		return
	}
	keys := sa.SubsystemKeys()
	if len(keys) == 0 {
		return
	}
	counts := countComponentsBySubsystem(payload, resolve)

	stats := make([]types.SubsystemStat, 0, len(keys))
	for _, key := range keys {
		cs := sa.GetSubsystemStats(key)
		if cs == nil {
			continue
		}
		entry := types.SubsystemStat{
			Path:           key,
			ComponentCount: counts[key],
			CodeStats:      cs,
		}
		if g, ok := groups[key]; ok {
			entry.Paths = g.Paths
			entry.Description = g.Description
		}
		stats = append(stats, entry)
	}
	sortSubsystemStatsByCode(stats)
	payload.SubsystemStats = stats
}

// countComponentsBySubsystem counts components per subsystem using the resolver function.
func countComponentsBySubsystem(payload *types.Payload, resolve subsystemKeyResolver) map[string]int {
	counts := make(map[string]int)
	countComponentsBySubsystemRecursive(payload, counts, resolve)
	return counts
}

func countComponentsBySubsystemRecursive(payload *types.Payload, counts map[string]int, resolve subsystemKeyResolver) {
	cp := payload.ComponentPath()
	if cp != "" {
		if key := resolve(cp); key != "" {
			counts[key]++
		}
	}
	for _, child := range payload.Children {
		countComponentsBySubsystemRecursive(child, counts, resolve)
	}
}

// sortSubsystemStatsByCode sorts subsystem stats by total code lines descending.
func sortSubsystemStatsByCode(stats []types.SubsystemStat) {
	sort.Slice(stats, func(i, j int) bool {
		ci := codeLines(stats[i].CodeStats)
		cj := codeLines(stats[j].CodeStats)
		if ci != cj {
			return ci > cj
		}
		return stats[i].Path < stats[j].Path
	})
}

// codeLines extracts total code lines from a CodeStats interface value.
func codeLines(cs interface{}) int64 {
	if typed, ok := cs.(*codestats.CodeStats); ok {
		return typed.Total.Code
	}
	return 0
}

// convertPrimaryLanguages converts codestats.PrimaryLanguage to types.PrimaryLanguage.
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

// buildCodeStatsAnalyzer creates the code stats analyzer from settings.
func buildCodeStatsAnalyzer(s *config.Settings) codestats.Analyzer {
	if s.NoCodeStats {
		return codestats.NewNoopAnalyzer()
	}
	return codestats.NewAnalyzer(codestats.AnalyzerConfig{
		PerComponent:     s.ComponentStatsDepth > 0,
		Subsystem:        s.SubsystemDepth > 0 || len(s.SubsystemGroups) > 0,
		PrimaryThreshold: s.PrimaryLanguageThreshold,
		MaxPrimaryLangs:  5,
	})
}
