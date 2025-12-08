package aggregator

import (
	"sort"

	"github.com/petrarca/tech-stack-analyzer/internal/git"
	"github.com/petrarca/tech-stack-analyzer/internal/metadata"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// AggregateOutput represents aggregated/rolled-up data from the scan
type AggregateOutput struct {
	Metadata     interface{}         `json:"metadata,omitempty"`     // Scan metadata (from root payload)
	Git          []*git.GitInfo      `json:"git,omitempty"`          // Git repositories (deduplicated)
	Tech         []string            `json:"tech,omitempty"`         // Primary/main technologies
	Techs        []string            `json:"techs,omitempty"`        // All detected technologies
	Reason       map[string][]string `json:"reason,omitempty"`       // Technology to detection reasons mapping, "_" for non-tech reasons
	Languages    map[string]int      `json:"languages,omitempty"`    // Language file counts
	Licenses     []string            `json:"licenses,omitempty"`     // Detected licenses
	Dependencies [][]string          `json:"dependencies,omitempty"` // All dependencies [type, name, version]
	CodeStats    interface{}         `json:"code_stats,omitempty"`   // Code statistics (if enabled)
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
	return &Aggregator{
		fields: fieldMap,
	}
}

// Aggregate processes a payload and returns aggregated data
func (a *Aggregator) Aggregate(payload *types.Payload) *AggregateOutput {
	output := &AggregateOutput{}

	// Include metadata from the root payload and update format
	output.Metadata = payload.Metadata
	if meta, ok := payload.Metadata.(*metadata.ScanMetadata); ok {
		meta.SetFormat("aggregated")
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
		output.Licenses = a.collectLicenses(payload)
	}

	if a.fields["dependencies"] {
		output.Dependencies = a.collectDependencies(payload)
	}

	// Include code stats if present
	output.CodeStats = payload.CodeStats

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
	for _, child := range payload.Childs {
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
	for _, child := range payload.Childs {
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
	for _, child := range payload.Childs {
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
	for _, child := range payload.Childs {
		a.collectLanguagesRecursive(child, languages)
	}
}

// collectLicenses recursively collects all unique licenses
func (a *Aggregator) collectLicenses(payload *types.Payload) []string {
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
	// Add licenses from current payload
	for _, license := range payload.Licenses {
		licenseSet[license] = true
	}

	// Recursively process children
	for _, child := range payload.Childs {
		a.collectLicensesRecursive(child, licenseSet)
	}
}

// collectDependencies recursively collects all unique dependencies
func (a *Aggregator) collectDependencies(payload *types.Payload) [][]string {
	// Use a map with string key to track unique dependencies
	// Key format: "type|name|version"
	depMap := make(map[string]types.Dependency)
	a.collectDependenciesRecursive(payload, depMap)

	// Convert to array format for JSON output
	dependencies := make([][]string, 0, len(depMap))
	for _, dep := range depMap {
		dependencies = append(dependencies, []string{dep.Type, dep.Name, dep.Example})
	}

	// Sort by type, then name, then version
	sort.Slice(dependencies, func(i, j int) bool {
		if dependencies[i][0] != dependencies[j][0] {
			return dependencies[i][0] < dependencies[j][0]
		}
		if dependencies[i][1] != dependencies[j][1] {
			return dependencies[i][1] < dependencies[j][1]
		}
		return dependencies[i][2] < dependencies[j][2]
	})

	return dependencies
}

// collectDependenciesRecursive helper function
func (a *Aggregator) collectDependenciesRecursive(payload *types.Payload, depMap map[string]types.Dependency) {
	// Add dependencies from current payload
	for _, dep := range payload.Dependencies {
		// Create unique key from type|name|version
		key := dep.Type + "|" + dep.Name + "|" + dep.Example
		depMap[key] = dep
	}

	// Recursively process children
	for _, child := range payload.Childs {
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
	for _, child := range payload.Childs {
		a.collectGitRecursive(child, gitMap)
	}
}

// sortStrings sorts a slice of strings in place and returns it
func sortStrings(s []string) []string {
	sort.Strings(s)
	return s
}
