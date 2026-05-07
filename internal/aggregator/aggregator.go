package aggregator

import (
	"sort"

	"github.com/petrarca/tech-stack-analyzer/internal/codestats"
	"github.com/petrarca/tech-stack-analyzer/internal/config"
	"github.com/petrarca/tech-stack-analyzer/internal/git"
	"github.com/petrarca/tech-stack-analyzer/internal/metadata"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ComponentEntry represents a single component in the flat component list
type ComponentEntry struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Type      string      `json:"type,omitempty"`
	Tech      []string    `json:"tech,omitempty"`
	Techs     []string    `json:"techs,omitempty"`
	Path      string      `json:"path,omitempty"`       // First path entry (primary location)
	SourceDir string      `json:"source_dir,omitempty"` // Directory this component owns, relative to scan root
	CodeStats interface{} `json:"code_stats,omitempty"` // Per-component code statistics (included when within component-stats-depth)
}

// AggregateOutput represents aggregated/rolled-up data from the scan
type AggregateOutput struct {
	Metadata           interface{}             `json:"metadata,omitempty"`            // Scan metadata (from root payload)
	Git                []*git.GitInfo          `json:"git,omitempty"`                 // Git repositories (deduplicated)
	Tech               []string                `json:"tech,omitempty"`                // All is_primary_tech technologies (from all components)
	Techs              []string                `json:"techs,omitempty"`               // All detected technologies
	PrimaryTechs       []string                `json:"primary_techs,omitempty"`       // Weight-filtered primary technologies (adaptive threshold on component count)
	Reason             map[string][]string     `json:"reason,omitempty"`              // Technology to detection reasons mapping, "_" for non-tech reasons
	Languages          map[string]int          `json:"languages,omitempty"`           // Language file counts
	PrimaryLanguages   []types.PrimaryLanguage `json:"primary_languages,omitempty"`   // Top programming languages (from code_stats)
	LicensesAggregated []string                `json:"licenses_aggregated,omitempty"` // Detected licenses (unique names only)
	Dependencies       []types.Dependency      `json:"dependencies,omitempty"`        // All dependencies serialized as [type, name, version, scope, direct, {metadata}] via Dependency.MarshalJSON
	Components         []ComponentEntry        `json:"components,omitempty"`          // Flat list of all components (id, name, type, tech, techs, path)
	CodeStats          interface{}             `json:"code_stats,omitempty"`          // Code statistics (if enabled)
	SubsystemStats     []types.SubsystemStat   `json:"subsystem_stats,omitempty"`     // Per-subsystem code stats rollup (when --subsystem-depth > 0)
	Ecosystems         []types.EcosystemEntry  `json:"ecosystems,omitempty"`          // Detected technology ecosystems (derived from components, techs, languages)
}

// Aggregator handles aggregation of scan results
type Aggregator struct {
	fields map[string]bool
}

// NewAggregator creates a new aggregator with specified fields
func NewAggregator(fields []string) *Aggregator {
	fieldMap := make(map[string]bool)
	for _, field := range fields {
		fieldMap[field] = true
	}
	return &Aggregator{fields: fieldMap}
}

// Aggregate processes a payload and returns aggregated data
func (a *Aggregator) Aggregate(payload *types.Payload) *AggregateOutput {
	output := &AggregateOutput{}

	// Copy metadata rather than mutating the original payload.
	// Aggregate() must be safe to call without side-effects on its input.
	if meta, ok := payload.Metadata.(*metadata.ScanMetadata); ok {
		cloned := *meta
		cloned.Format = "aggregated"
		output.Metadata = &cloned
	} else {
		output.Metadata = payload.Metadata
	}

	if a.fields["git"] {
		output.Git = a.collectGit(payload)
	}

	if a.fields["tech"] {
		output.Tech = a.collectPrimaryTechs(payload)
	}

	if a.fields["techs"] {
		output.Techs = a.collectTechs(payload)
	}

	if a.fields["reason"] {
		output.Reason = a.collectReasons(payload)
	}

	if a.fields["languages"] {
		output.Languages = a.collectLanguages(payload)
	}

	if a.fields["licenses"] {
		output.LicensesAggregated = a.collectLicenses(payload)
	}

	if a.fields["dependencies"] {
		output.Dependencies = a.collectDependencies(payload)
	}

	if a.fields["components"] {
		output.Components = a.collectComponents(payload)
	}

	// Include code stats if present
	output.CodeStats = payload.CodeStats

	// Copy primary_languages from root payload (already extracted from code_stats)
	output.PrimaryLanguages = payload.PrimaryLanguages

	// Compute primary_techs from flat component list (always, after components and tech are collected)
	output.PrimaryTechs = computePrimaryTechs(output.Components, output.Tech, output.PrimaryLanguages)

	// Copy subsystem stats if present (populated by cmd/scan.go after scanning)
	output.SubsystemStats = payload.SubsystemStats

	// Derive ecosystems from components, techs, and primary languages
	output.Ecosystems = computeEcosystems(output.Components, output.PrimaryLanguages)

	return output
}

// collectPrimaryTechs recursively collects all unique primary techs (tech field) from payload and children
func (a *Aggregator) collectPrimaryTechs(payload *types.Payload) []string {
	techSet := make(map[string]bool)
	a.collectPrimaryTechsRecursive(payload, techSet)

	// Convert to sorted slice
	techs := make([]string, 0, len(techSet))
	for tech := range techSet {
		techs = append(techs, tech)
	}

	return sortStrings(techs)
}

// collectPrimaryTechsRecursive helper function
func (a *Aggregator) collectPrimaryTechsRecursive(payload *types.Payload, techSet map[string]bool) {
	// Add all primary techs from current payload
	for _, tech := range payload.Tech {
		if tech != "" {
			techSet[tech] = true
		}
	}

	// Recursively process children
	for _, child := range payload.Children {
		a.collectPrimaryTechsRecursive(child, techSet)
	}
}

// collectTechs recursively collects all unique techs from payload and children
func (a *Aggregator) collectTechs(payload *types.Payload) []string {
	techSet := make(map[string]bool)
	a.collectTechsRecursive(payload, techSet)

	// Convert to sorted slice
	techs := make([]string, 0, len(techSet))
	for tech := range techSet {
		techs = append(techs, tech)
	}

	// Sort for consistent output
	return sortStrings(techs)
}

// collectReasons recursively collects all reasons from payload and children
func (a *Aggregator) collectReasons(payload *types.Payload) map[string][]string {
	reasons := make(map[string][]string)
	a.collectReasonsRecursive(payload, reasons)
	return reasons
}

// collectTechsRecursive helper function
func (a *Aggregator) collectTechsRecursive(payload *types.Payload, techSet map[string]bool) {
	// Add techs from current payload
	for _, tech := range payload.Techs {
		techSet[tech] = true
	}

	// Recursively process children
	for _, child := range payload.Children {
		a.collectTechsRecursive(child, techSet)
	}
}

// collectReasonsRecursive helper function
func (a *Aggregator) collectReasonsRecursive(payload *types.Payload, reasons map[string][]string) {
	// Add reasons from current payload
	for tech, techReasons := range payload.Reason {
		if reasons[tech] == nil {
			reasons[tech] = make([]string, 0)
		}
		// Add reasons with deduplication
		for _, reason := range techReasons {
			reasonExists := false
			for _, existing := range reasons[tech] {
				if existing == reason {
					reasonExists = true
					break
				}
			}
			if !reasonExists {
				reasons[tech] = append(reasons[tech], reason)
			}
		}
	}

	// Recursively process children
	for _, child := range payload.Children {
		a.collectReasonsRecursive(child, reasons)
	}
}

// collectLanguages recursively collects and sums all languages
func (a *Aggregator) collectLanguages(payload *types.Payload) map[string]int {
	languages := make(map[string]int)
	a.collectLanguagesRecursive(payload, languages)
	return languages
}

// collectLanguagesRecursive helper function
func (a *Aggregator) collectLanguagesRecursive(payload *types.Payload, languages map[string]int) {
	// Add languages from current payload
	for lang, count := range payload.Languages {
		languages[lang] += count
	}

	// Recursively process children
	for _, child := range payload.Children {
		a.collectLanguagesRecursive(child, languages)
	}
}

// collectLicenses recursively collects all unique licenses
func (a *Aggregator) collectLicenses(payload *types.Payload) []string {
	// Use a map with string key to track unique licenses
	licenseSet := make(map[string]bool)
	a.collectLicensesRecursive(payload, licenseSet)

	// Convert to sorted slice
	licenses := make([]string, 0, len(licenseSet))
	for license := range licenseSet {
		licenses = append(licenses, license)
	}

	return sortStrings(licenses)
}

// collectLicensesRecursive helper function
func (a *Aggregator) collectLicensesRecursive(payload *types.Payload, licenseSet map[string]bool) {
	// Add licenses from current payload (extract license_name from structured objects)
	for _, license := range payload.Licenses {
		licenseSet[license.LicenseName] = true
	}

	// Recursively process children
	for _, child := range payload.Children {
		a.collectLicensesRecursive(child, licenseSet)
	}
}

// collectDependencies recursively collects all unique dependencies.
// Uniqueness is keyed on type|name|version. The result is sorted by
// type, then name, then version for deterministic output.
// JSON serialization of each Dependency is handled by Dependency.MarshalJSON,
// which produces the canonical [type, name, version, scope, direct, {metadata}] array.
func (a *Aggregator) collectDependencies(payload *types.Payload) []types.Dependency {
	depMap := make(map[string]types.Dependency)
	a.collectDependenciesRecursive(payload, depMap)

	dependencies := make([]types.Dependency, 0, len(depMap))
	for _, dep := range depMap {
		dependencies = append(dependencies, dep)
	}

	sort.Slice(dependencies, func(i, j int) bool {
		if dependencies[i].Type != dependencies[j].Type {
			return dependencies[i].Type < dependencies[j].Type
		}
		if dependencies[i].Name != dependencies[j].Name {
			return dependencies[i].Name < dependencies[j].Name
		}
		return dependencies[i].Version < dependencies[j].Version
	})

	return dependencies
}

// collectDependenciesRecursive helper function
func (a *Aggregator) collectDependenciesRecursive(payload *types.Payload, depMap map[string]types.Dependency) {
	// Add dependencies from current payload
	for _, dep := range payload.Dependencies {
		// Create unique key from type|name|version
		key := dep.Type + "|" + dep.Name + "|" + dep.Version
		depMap[key] = dep
	}

	// Recursively process children
	for _, child := range payload.Children {
		a.collectDependenciesRecursive(child, depMap)
	}
}

// collectGit recursively collects all unique git repositories
func (a *Aggregator) collectGit(payload *types.Payload) []*git.GitInfo {
	// Use a map with string key to track unique git repositories
	// Key format: "remote_url|branch|commit"
	gitMap := make(map[string]*git.GitInfo)
	a.collectGitRecursive(payload, gitMap)

	// Convert to slice format for JSON output
	gitRepos := make([]*git.GitInfo, 0, len(gitMap))
	for _, gitInfo := range gitMap {
		gitRepos = append(gitRepos, gitInfo)
	}

	// Sort by remote URL, then branch, then commit
	sort.Slice(gitRepos, func(i, j int) bool {
		if gitRepos[i].RemoteURL != gitRepos[j].RemoteURL {
			return gitRepos[i].RemoteURL < gitRepos[j].RemoteURL
		}
		if gitRepos[i].Branch != gitRepos[j].Branch {
			return gitRepos[i].Branch < gitRepos[j].Branch
		}
		return gitRepos[i].Commit < gitRepos[j].Commit
	})

	return gitRepos
}

// collectGitRecursive helper function
func (a *Aggregator) collectGitRecursive(payload *types.Payload, gitMap map[string]*git.GitInfo) {
	// Add git info from current payload
	if payload.Git != nil {
		// Create unique key from remote_url|branch|commit
		key := payload.Git.RemoteURL + "|" + payload.Git.Branch + "|" + payload.Git.Commit
		gitMap[key] = payload.Git
	}

	// Recursively process children
	for _, child := range payload.Children {
		a.collectGitRecursive(child, gitMap)
	}
}

// collectComponents recursively collects all components as a flat list, skipping the root node.
// code_stats is included on each component when already populated (controlled by --component-stats-depth).
func (a *Aggregator) collectComponents(payload *types.Payload) []ComponentEntry {
	var components []ComponentEntry
	collectComponentsRecursive(payload, &components, false)
	return components
}

// collectComponentsRecursive appends non-root components to the slice.
// skipCurrent is true only for the root call, false for all recursive calls.
func collectComponentsRecursive(payload *types.Payload, components *[]ComponentEntry, includeNode bool) {
	if includeNode {
		path := ""
		if len(payload.Path) > 0 {
			path = payload.Path[0]
		}
		*components = append(*components, ComponentEntry{
			ID:        payload.ID,
			Name:      payload.Name,
			Type:      payload.ComponentType,
			Tech:      payload.Tech,
			Techs:     payload.Techs,
			Path:      path,
			SourceDir: payload.SourceDir,
			CodeStats: payload.CodeStats, // non-nil only when --component-stats-depth covers this component
		})
	}
	for _, child := range payload.Children {
		collectComponentsRecursive(child, components, true)
	}
}

// computePrimaryTechs derives primary technologies from the flat component list.
// Uses code-line weighting (≥1% of total typed code) when per-component code_stats
// cover ≥50% of typed components. Falls back to component-count threshold
// (≤10 typed: all; >10: ≥3% of typed component count) when stats are sparse.
//
// Languages that dominate primary_languages (≥50%) but have no typed components
// (e.g. C++ in legacy codebases without detected build manifests) are promoted
// from tech[] into primary_techs so the graph ingestion captures them correctly.
func computePrimaryTechs(components []ComponentEntry, techEligible []string, primaryLangs []types.PrimaryLanguage) []string {
	if len(techEligible) == 0 {
		return nil
	}
	eligible := make(map[string]bool, len(techEligible))
	for _, t := range techEligible {
		eligible[t] = true
	}

	// Count typed components and those with code stats
	typedCount := 0
	withStats := 0
	for _, comp := range components {
		if comp.Type == "" {
			continue
		}
		typedCount++
		if code := extractCode(comp.CodeStats); code > 0 {
			withStats++
		}
	}

	// Use code-weight when stats cover ≥50% of typed components
	var result []string
	if withStats > 0 && withStats*2 >= typedCount {
		result = computeByCodeWeight(components, eligible)
	} else {
		result = computeByComponentCount(components, eligible, typedCount)
	}

	// Promote dominant languages that are eligible (in tech[]) but have no typed
	// components — e.g. C++ codebases without detected build manifests.
	result = promoteDominantLanguages(result, eligible, components, primaryLangs)

	return result
}

// promoteDominantLanguages adds eligible languages that dominate primary_languages
// (≥50% of code) but are absent from the computed primary_techs because no typed
// components carry them (build manifests not detected).
func promoteDominantLanguages(result []string, eligible map[string]bool, components []ComponentEntry, primaryLangs []types.PrimaryLanguage) []string {
	if len(primaryLangs) == 0 {
		return result
	}

	// Build set of techs already in result for O(1) lookup
	inResult := make(map[string]bool, len(result))
	for _, t := range result {
		inResult[t] = true
	}

	// Build set of techs carried by ANY typed component
	inTypedComponent := make(map[string]bool)
	for _, comp := range components {
		if comp.Type == "" {
			continue
		}
		for _, t := range comp.Tech {
			inTypedComponent[t] = true
		}
	}

	promoted := false
	for _, pl := range primaryLangs {
		if pl.Pct < 0.50 {
			continue
		}
		// Map language name to tech key (primary_languages uses display name, tech[] uses scanner key)
		techKey := languageNameToTechKey(pl.Language)
		if techKey == "" || !eligible[techKey] {
			continue
		}
		// Only promote if not already in result and not present in any typed component
		if !inResult[techKey] && !inTypedComponent[techKey] {
			result = append(result, techKey)
			promoted = true
		}
	}

	if promoted {
		return sortStrings(result)
	}
	return result
}

// languageNameToTechKey maps a primary_languages display name to its scanner tech key.
// Only covers languages that can dominate a codebase without detectable build manifests
// (i.e. where the scanner creates a virtual node rather than a typed component).
// This mapping mirrors the `name` field in the corresponding rule YAML files.
// If a new language rule is added for such a language, add it here too.
func languageNameToTechKey(lang string) string {
	switch lang {
	case "C++":
		return "cplusplus"
	case "C":
		return "c"
	case "Delphi":
		return "delphi"
	case "COBOL":
		return "cobol"
	case "Fortran":
		return "fortran"
	case "Assembly":
		return "assembly"
	default:
		return ""
	}
}

// computeByCodeWeight sums code lines per eligible tech and applies 1% threshold.
func computeByCodeWeight(components []ComponentEntry, eligible map[string]bool) []string {
	techCode := make(map[string]int64)
	var totalCode int64

	for _, comp := range components {
		if comp.Type == "" {
			continue
		}
		code := extractCode(comp.CodeStats)
		if code <= 0 {
			continue
		}
		totalCode += code
		for _, t := range comp.Tech {
			if eligible[t] {
				techCode[t] += code
			}
		}
	}

	threshold := totalCode / 100
	if threshold < 1 {
		threshold = 1
	}

	result := make([]string, 0)
	for tech, code := range techCode {
		if code >= threshold {
			result = append(result, tech)
		}
	}
	return sortStrings(result)
}

// computeByComponentCount counts typed components per eligible tech with adaptive threshold.
func computeByComponentCount(components []ComponentEntry, eligible map[string]bool, typedCount int) []string {
	techCount := make(map[string]int)
	for _, comp := range components {
		if comp.Type == "" {
			continue
		}
		for _, t := range comp.Tech {
			if eligible[t] {
				techCount[t]++
			}
		}
	}

	threshold := 1
	if typedCount > 10 {
		threePercent := typedCount * 3 / 100
		if threePercent < 2 {
			threePercent = 2
		}
		threshold = threePercent
	}

	result := make([]string, 0)
	for tech, count := range techCount {
		if count >= threshold {
			result = append(result, tech)
		}
	}
	return sortStrings(result)
}

// extractCode extracts code lines from a component's code_stats field.
// CodeStats is stored as interface{} on ComponentEntry to avoid circular imports,
// but is always *codestats.CodeStats at runtime when present.
func extractCode(stats interface{}) int64 {
	if cs, ok := stats.(*codestats.CodeStats); ok {
		return cs.Total.Code
	}
	return 0
}

// ComputeEcosystemsFromPayload derives technology ecosystems directly from a Payload tree.
// Used to populate the ecosystems field on the full (non-aggregated) output so consumers
// don't need to cross-reference the -agg file.
func ComputeEcosystemsFromPayload(payload *types.Payload) []types.EcosystemEntry {
	components := collectComponentsFromPayload(payload)
	return computeEcosystems(components, payload.PrimaryLanguages)
}

// collectComponentsFromPayload walks the payload tree and collects all components.
func collectComponentsFromPayload(payload *types.Payload) []ComponentEntry {
	var components []ComponentEntry
	collectComponentsRecursive(payload, &components, false)
	return components
}

// computeEcosystems derives technology ecosystems from the flat component list and primary languages.
// Detection priority:
//  1. component_types -- strongest signal (package manager / build system)
//  2. techs -- framework/runtime techs in component tech[] arrays
//  3. languages -- fallback from primary_languages for ecosystems without package managers
//
// Returns a sorted slice of EcosystemEntry (sorted by component count descending).
// Returns nil when no ecosystems are detected.
func computeEcosystems(components []ComponentEntry, primaryLangs []types.PrimaryLanguage) []types.EcosystemEntry {
	ecosystemsCfg, err := config.LoadEcosystemsConfig()
	if err != nil || len(ecosystemsCfg.Ecosystems) == 0 {
		return nil
	}

	typeMap, techMap, langMap := buildEcosystemLookups(ecosystemsCfg.Ecosystems)

	type ecosystemMatch struct {
		name       string
		components int
	}
	results := make(map[string]*ecosystemMatch)

	incr := func(ecoName string) {
		if results[ecoName] == nil {
			results[ecoName] = &ecosystemMatch{name: ecoName}
		}
		results[ecoName].components++
	}

	// Signal 1: Match component types (strongest signal)
	for _, comp := range components {
		if ecoName, ok := typeMap[comp.Type]; ok && comp.Type != "" {
			incr(ecoName)
		}
	}

	// Signal 2: Match component tech[] entries (untyped components only)
	for _, comp := range components {
		if comp.Type != "" {
			continue
		}
		matched := make(map[string]bool)
		for _, t := range comp.Tech {
			if ecoName, ok := techMap[t]; ok && !matched[ecoName] {
				incr(ecoName)
				matched[ecoName] = true
			}
		}
	}

	// Signal 3: Language fallback — for ecosystems not yet detected
	for _, pl := range primaryLangs {
		if ecoName, ok := langMap[pl.Language]; ok && results[ecoName] == nil {
			results[ecoName] = &ecosystemMatch{name: ecoName, components: 1}
		}
	}

	if len(results) == 0 {
		return nil
	}

	entries := make([]types.EcosystemEntry, 0, len(results))
	for _, m := range results {
		entries = append(entries, types.EcosystemEntry{Ecosystem: m.name, Components: m.components})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Components != entries[j].Components {
			return entries[i].Components > entries[j].Components
		}
		return entries[i].Ecosystem < entries[j].Ecosystem
	})
	return entries
}

// buildEcosystemLookups creates reverse maps from component_type/tech/language to ecosystem name.
func buildEcosystemLookups(ecosystems []types.EcosystemDefinition) (typeMap, techMap, langMap map[string]string) {
	typeMap = make(map[string]string)
	techMap = make(map[string]string)
	langMap = make(map[string]string)
	for _, eco := range ecosystems {
		for _, ct := range eco.ComponentTypes {
			typeMap[ct] = eco.Name
		}
		for _, t := range eco.Techs {
			techMap[t] = eco.Name
		}
		for _, l := range eco.Languages {
			langMap[l] = eco.Name
		}
	}
	return
}

// sortStrings sorts a slice of strings in place and returns it
func sortStrings(s []string) []string {
	sort.Strings(s)
	return s
}
