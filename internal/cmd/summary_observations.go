package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/codestats"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// commonLangs is the set of widely-used languages that do not need to be
// flagged as niche in the Observations section.
var commonLangs = map[string]bool{
	"C": true, "C#": true, "C++": true, "CSS": true, "Dart": true,
	"Dockerfile": true, "Elixir": true, "Erlang": true, "Go": true,
	"Groovy": true, "HTML": true, "Haskell": true, "Java": true,
	"JavaScript": true, "JSON": true, "Kotlin": true, "Lua": true,
	"Makefile": true, "Markdown": true, "Objective-C": true, "PHP": true,
	"Perl": true, "PowerShell": true, "Python": true, "R": true,
	"Ruby": true, "Rust": true, "Sass": true, "SCSS": true, "SQL": true,
	"Scala": true, "Shell": true, "Swift": true, "TOML": true,
	"TSX": true, "TSQL": true, "TypeScript": true, "VB.NET": true,
	"Visual Basic 6.0": true, "XML": true, "YAML": true, "XAML": true,
}

// printObservations prints the Observations section if any signals are found.
func printObservations(p *types.Payload) {
	obs := collectObservations(p)
	if len(obs) == 0 {
		return
	}
	fmt.Println()
	fmt.Println("Observations")
	fmt.Println(strings.Repeat("-", 70))
	for _, o := range obs {
		fmt.Printf("  - %s\n", o)
	}
}

// collectObservations builds the list of observation strings from payload data.
func collectObservations(p *types.Payload) []string {
	var obs []string

	// Scanner-level file observations (generated, vendored, large, encoding)
	if scanObs, ok := p.ScanObservations.(*scanner.ScanObservations); ok {
		obs = append(obs, formatScanObservations(scanObs)...)
	}

	// Payload-derived observations
	if o := observeNicheLanguages(p); o != "" {
		obs = append(obs, o)
	}
	obs = append(obs, observeHighComplexity(p)...)
	if o := observeEcosystemDominance(p); o != "" {
		obs = append(obs, o)
	}
	if o := observeDuplicatedComponents(p); o != "" {
		obs = append(obs, o)
	}
	return obs
}

func observeNicheLanguages(p *types.Payload) string {
	var niche []string
	for lang, count := range p.Languages {
		if !commonLangs[lang] && count >= 10 {
			niche = append(niche, fmt.Sprintf("%s (%d files)", lang, count))
		}
	}
	if len(niche) == 0 {
		return ""
	}
	sort.Strings(niche)
	return "Niche/uncommon languages detected: " + strings.Join(niche, ", ")
}

func observeHighComplexity(p *types.Payload) []string {
	cs, ok := codeStatsFromPayload(p)
	if !ok {
		return nil
	}
	var obs []string
	for _, lang := range cs.Analyzed.ByLanguage {
		if lang.Code > 0 && lang.Complexity > 0 && lang.Files > 50 {
			ratio := float64(lang.Complexity) / float64(lang.Code) * 100
			if ratio > 15 {
				obs = append(obs, fmt.Sprintf("High cyclomatic complexity: %s (%.0f%% complexity/code ratio, %s files)",
					lang.Language, ratio, fmtInt(int64(lang.Files))))
			}
		}
	}
	return obs
}

func observeEcosystemDominance(p *types.Payload) string {
	if len(p.Ecosystems) <= 2 {
		return ""
	}
	total := 0
	for _, e := range p.Ecosystems {
		total += e.Components
	}
	if total == 0 {
		return ""
	}
	dominant := p.Ecosystems[0] // sorted by component count descending
	pct := float64(dominant.Components) / float64(total) * 100
	if pct <= 85 {
		return ""
	}
	return fmt.Sprintf("Ecosystem dominated by %s (%.0f%% of components), %d minor ecosystems also present",
		dominant.Ecosystem, pct, len(p.Ecosystems)-1)
}

func observeDuplicatedComponents(p *types.Payload) string {
	nameCount := make(map[string]int, len(p.Children))
	for _, child := range p.Children {
		nameCount[child.Name]++
	}
	type dupEntry struct {
		name  string
		count int
	}
	var dups []dupEntry
	for name, count := range nameCount {
		if count >= 5 {
			dups = append(dups, dupEntry{name, count})
		}
	}
	if len(dups) == 0 {
		return ""
	}
	sort.Slice(dups, func(i, j int) bool { return dups[i].count > dups[j].count })
	shown := min(3, len(dups))
	parts := make([]string, shown)
	for i := 0; i < shown; i++ {
		parts[i] = fmt.Sprintf("%s (%dx)", dups[i].name, dups[i].count)
	}
	msg := fmt.Sprintf("%d component names appear 5+ times (duplicated/copied structure): %s",
		len(dups), strings.Join(parts, ", "))
	if len(dups) > shown {
		msg += fmt.Sprintf(", ... and %d more", len(dups)-shown)
	}
	return msg
}

// --- scanner observation formatting ---

// formatScanObservations formats scanner-level ScanObservations into actionable strings.
func formatScanObservations(obs *scanner.ScanObservations) []string {
	if obs == nil {
		return nil
	}
	var result []string

	if o := formatDirObs(obs.Generated,
		"generated files detected (not excluded) — may include designer files, proto stubs, or template output",
		"exclude confirmed generated directories"); o != "" {
		result = append(result, o)
	}
	if o := formatDirObs(obs.Vendored,
		"vendored files detected (not excluded) — may include third-party bundles or copied libraries",
		"exclude confirmed vendor directories"); o != "" {
		result = append(result, o)
	}
	if o := formatDirObs(obs.LargeFiles,
		fmt.Sprintf("large programming files (>%dKB) — possible minified bundles or vendored assets", scanner.LargeFileThreshold()/1024),
		"exclude confirmed bundle/dist directories"); o != "" {
		result = append(result, o)
	}
	if obs.NonUTF8 > 0 {
		result = append(result, fmt.Sprintf(
			"%s files with non-UTF-8 encoding detected — consider setting `encoding` in ingestion config",
			fmtInt(int64(obs.NonUTF8))))
	}
	return result
}

func formatDirObs(obs *scanner.DirObservation, label, hint string) string {
	if obs == nil || obs.Total == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %s", fmtInt(int64(obs.Total)), label)
	if len(obs.TopDirs) > 0 {
		fmt.Fprintf(&sb, "\n    → %s — review directories below, then add confirmed ones to your scan config exclude:", hint)
		for _, d := range obs.TopDirs {
			fmt.Fprintf(&sb, "\n        - \"**/%s/**\"   # %s files", d.Dir, fmtInt(int64(d.Count)))
		}
	}
	return sb.String()
}

// --- helpers ---

// codeStatsFromPayload extracts the typed CodeStats from a payload's interface{} field.
func codeStatsFromPayload(p *types.Payload) (*codestats.CodeStats, bool) {
	if p.CodeStats == nil {
		return nil, false
	}
	cs, ok := p.CodeStats.(*codestats.CodeStats)
	return cs, ok
}
