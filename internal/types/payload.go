package types

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-enry/go-enry/v2"
	"github.com/petrarca/tech-stack-analyzer/internal/git"
)

// Payload represents the analysis result for a directory or component
type Payload struct {
	Metadata         interface{}            `json:"metadata,omitempty"`
	Git              *git.GitInfo           `json:"git,omitempty"`
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Path             []string               `json:"path"`
	Tech             []string               `json:"tech"` // Changed from *string to []string to support multiple primary technologies
	Techs            []string               `json:"techs"`
	Languages        map[string]int         `json:"languages"`
	PrimaryLanguages []PrimaryLanguage      `json:"primary_languages,omitempty"` // Top programming languages (from code_stats)
	Licenses         []License              `json:"licenses"`                    // Changed to structured License objects
	Reason           map[string][]string    `json:"reason"`                      // Maps technology to detection reasons, "_" for non-tech reasons
	Dependencies     []Dependency           `json:"dependencies"`
	Properties       map[string]interface{} `json:"properties,omitempty"`
	Childs           []*Payload             `json:"childs"` // Changed from Children to Childs
	Edges            []Edge                 `json:"edges"`
	CodeStats        interface{}            `json:"code_stats,omitempty"`
}

// Edge represents a relationship between components
type Edge struct {
	Target *Payload `json:"target"`
	Read   bool     `json:"read"`
	Write  bool     `json:"write"`
}

// PrimaryLanguage represents a primary programming language (top languages by lines of code)
type PrimaryLanguage struct {
	Language string  `json:"language"`
	Pct      float64 `json:"pct"`
}

// License represents a structured license entity for knowledge graph integration
type License struct {
	LicenseName     string  `json:"license_name"`               // Primary SPDX identifier (e.g., "MIT", "Apache-2.0")
	DetectionType   string  `json:"detection_type"`             // "direct", "normalized", "toml_parsed", "file_based"
	SourceFile      string  `json:"source_file"`                // Where detected (e.g., "package.json", "pyproject.toml")
	Confidence      float64 `json:"confidence"`                 // Detection confidence (0.0-1.0)
	OriginalLicense string  `json:"original_license,omitempty"` // Original license before normalization
}

// MarshalJSON customizes Edge JSON serialization to match TypeScript (target as ID string)
func (e Edge) MarshalJSON() ([]byte, error) {
	// Like TypeScript: serialize target as just the ID string
	targetID := ""
	if e.Target != nil {
		targetID = e.Target.ID
	}

	// Create a map to match the exact TypeScript format
	edgeMap := map[string]interface{}{
		"target": targetID,
		"read":   e.Read,
		"write":  e.Write,
	}

	return json.Marshal(edgeMap)
}

// NewPayload creates a new payload with a temporary ID (will be finalized by AssignIDs)
func NewPayload(name string, paths []string) *Payload {
	// Use first path for temporary ID generation
	var relativePath string
	if len(paths) > 0 {
		relativePath = paths[0]
	}

	return &Payload{
		ID:           GenerateComponentID("temp", name, relativePath), // Temporary ID, will be replaced
		Name:         name,
		Path:         paths,
		Techs:        make([]string, 0),
		Languages:    make(map[string]int),
		Dependencies: make([]Dependency, 0),
		Childs:       make([]*Payload, 0),
		Edges:        make([]Edge, 0),
		Licenses:     make([]License, 0),
		Reason:       make(map[string][]string),
		Properties:   make(map[string]interface{}),
	}
}

// NewPayloadWithPath creates a new payload with a single path (convenience function)
func NewPayloadWithPath(name, path string) *Payload {
	return NewPayload(name, []string{path})
}

// AssignIDs assigns unique IDs to the entire payload tree.
// The root gets the provided ID (or generates random if empty), and all children get deterministic IDs
// based on the root ID + their relative path.
// This should be called once after the entire tree is built.
func (p *Payload) AssignIDs(rootID string) {
	if rootID == "" {
		rootID = GenerateRootID()
	}
	p.ID = rootID

	// Recursively assign IDs to all children
	p.assignChildIDs(rootID)
}

// assignChildIDs recursively assigns deterministic IDs to children
func (p *Payload) assignChildIDs(rootID string) {
	for _, child := range p.Childs {
		// Use first path for ID generation
		var relativePath string
		if len(child.Path) > 0 {
			relativePath = child.Path[0]
		}
		child.ID = GenerateComponentID(rootID, child.Name, relativePath)

		// Recurse into grandchildren
		child.assignChildIDs(rootID)
	}
}

// AddChild adds a child payload with deduplication (following TypeScript's addChild logic exactly)
func (p *Payload) AddChild(service *Payload) *Payload {
	// Check for existing component to merge (like TypeScript lines 130-138)
	var exist *Payload
	for _, child := range p.Childs {
		// we only merge if a tech is similar otherwise it's too easy to get a false-positive
		if len(child.Tech) == 0 && len(service.Tech) == 0 {
			continue
		}
		if child.Name == service.Name {
			exist = child
			break
		}
		// REMOVED: Don't merge by technology type, only by name
		// This was causing all Node.js components to be merged together
	}

	if exist != nil {
		// Merge with existing component (like TypeScript lines 155-175)
		// Log all paths where it was found
		for _, path := range service.Path {
			exist.AddPath(path)
		}

		// Merge primary techs
		for _, tech := range service.Tech {
			exist.AddPrimaryTech(tech)
		}

		// Update edges to point to the initial component (simplified for Go)
		// This would need parent reference which we don't track in edges

		// Merge dependencies
		for _, dep := range service.Dependencies {
			exist.AddDependency(dep)
		}

		// Merge properties
		exist.mergeProperties(service.Properties)

		return exist
	}

	// Add new child if no duplicate found
	p.Childs = append(p.Childs, service)
	return service
}

// AddPath adds a path to the payload (like TypeScript Set.add)
func (p *Payload) AddPath(path string) {
	// Check for duplicate (like TypeScript Set behavior)
	for _, existing := range p.Path {
		if existing == path {
			return // Already exists, don't add duplicate
		}
	}
	p.Path = append(p.Path, path)
}

// AddLanguageWithCount increments the count for a language
func (p *Payload) AddLanguageWithCount(language string, count int) {
	p.Languages[language] += count
}

// Combine merges another payload into this one (following TypeScript's combine method)
func (p *Payload) Combine(other *Payload) {
	p.mergePaths(other.Path)
	p.mergeLanguages(other.Languages)
	p.mergeTechs(other.Techs)
	p.mergeTechField(other.Tech)
	p.mergeDependencies(other.Dependencies)
	p.mergeLicenses(other.Licenses)
	p.mergeReasons(other.Reason)
	p.mergeProperties(other.Properties)
	p.mergeGit(other.Git)
}

// Helper functions to reduce cognitive complexity

func (p *Payload) mergePaths(paths []string) {
	for _, path := range paths {
		if !p.containsString(p.Path, path) {
			p.Path = append(p.Path, path)
		}
	}
}

func (p *Payload) mergeLanguages(languages map[string]int) {
	for lang, count := range languages {
		p.Languages[lang] += count
	}
}

func (p *Payload) mergeTechs(techs []string) {
	for _, tech := range techs {
		if !p.containsString(p.Techs, tech) {
			p.Techs = append(p.Techs, tech)
		}
	}
}

func (p *Payload) mergeTechField(techs []string) {
	for _, tech := range techs {
		if tech != "" {
			p.AddPrimaryTech(tech)
			if !p.containsString(p.Techs, tech) {
				p.Techs = append(p.Techs, tech)
			}
		}
	}
}

func (p *Payload) mergeDependencies(deps []Dependency) {
	for _, dep := range deps {
		if !p.containsDependency(dep) {
			p.Dependencies = append(p.Dependencies, dep)
		}
	}
}

func (p *Payload) mergeLicenses(licenses []License) {
	for _, license := range licenses {
		if !p.containsLicense(p.Licenses, license.LicenseName) {
			p.Licenses = append(p.Licenses, license)
		}
	}
}

// containsLicense checks if a license with the given name exists in the license slice
func (p *Payload) containsLicense(licenses []License, licenseName string) bool {
	for _, existing := range licenses {
		if existing.LicenseName == licenseName {
			return true
		}
	}
	return false
}

func (p *Payload) mergeReasons(reasons map[string][]string) {
	for tech, reasons := range reasons {
		// Skip "_" key as it's for non-tech reasons, not actual technologies
		if tech == "_" {
			// Add "_" reasons directly without adding "_" as a tech
			for _, reason := range reasons {
				p.AddReason(reason)
			}
			continue
		}

		for _, reason := range reasons {
			// Use AddTech to handle deduplication and proper merging
			p.AddTech(tech, reason)
		}
	}
}

func (p *Payload) mergeProperties(properties map[string]interface{}) {
	if len(properties) == 0 {
		return
	}
	if p.Properties == nil {
		p.Properties = make(map[string]interface{})
	}
	for key, value := range properties {
		// Special handling for array properties (docker, terraform) - merge arrays
		if key == "docker" || key == "terraform" {
			existing, existsInP := p.Properties[key]
			newArray, isArray := value.([]interface{})

			if existsInP && isArray {
				// Both exist and new value is array - merge them
				if existingArray, ok := existing.([]interface{}); ok {
					p.Properties[key] = append(existingArray, newArray...)
				} else {
					// Existing is not array, wrap it and merge
					p.Properties[key] = append([]interface{}{existing}, newArray...)
				}
			} else {
				// Just set the value
				p.Properties[key] = value
			}
		} else {
			// For other properties, later values override earlier ones
			p.Properties[key] = value
		}
	}
}

func (p *Payload) mergeGit(gitInfo *git.GitInfo) {
	// Only set git info if we don't already have it (preserve first detected)
	if p.Git == nil && gitInfo != nil {
		p.Git = gitInfo
	}
}

func (p *Payload) containsString(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

func (p *Payload) containsDependency(dep Dependency) bool {
	for _, existing := range p.Dependencies {
		if existing.Type == dep.Type && existing.Name == dep.Name && existing.Version == dep.Version {
			return true
		}
	}
	return false
}

// AddTech adds a technology to the payload
func (p *Payload) AddTech(tech string, reason string) {
	// Avoid duplicates for techs, but still add reasons
	techExists := false
	for _, existing := range p.Techs {
		if existing == tech {
			techExists = true
			break
		}
	}

	if !techExists {
		p.Techs = append(p.Techs, tech)
		// NOTE: Don't set primary tech here like the original
		// The original's addTech method only adds to techs set, doesn't set this.tech
	}

	// Add reason to Reason mapping for clear association
	if reason != "" {
		// Initialize tech reasons slice if not exists
		if p.Reason[tech] == nil {
			p.Reason[tech] = make([]string, 0)
		}

		// Check if reason already exists for this tech to avoid duplicates
		reasonExists := false
		for _, existing := range p.Reason[tech] {
			if existing == reason {
				reasonExists = true
				break
			}
		}
		if !reasonExists {
			p.Reason[tech] = append(p.Reason[tech], reason)
		}
	}
}

// AddTechs adds multiple technologies
func (p *Payload) AddTechs(techs map[string][]string) {
	for tech, reasons := range techs {
		for _, reason := range reasons {
			p.AddTech(tech, reason)
		}
	}
}

// AddReason adds a non-tech reason to the "_" key
func (p *Payload) AddReason(reason string) {
	if reason != "" {
		// Initialize "_" slice if not exists
		if p.Reason["_"] == nil {
			p.Reason["_"] = make([]string, 0)
		}

		// Check if reason already exists to avoid duplicates
		reasonExists := false
		for _, existing := range p.Reason["_"] {
			if existing == reason {
				reasonExists = true
				break
			}
		}
		if !reasonExists {
			p.Reason["_"] = append(p.Reason["_"], reason)
		}
	}
}

// AddLanguage increments the count for a language
func (p *Payload) AddLanguage(language string) {
	p.Languages[language]++
}

// AddPrimaryTech adds a technology to the primary tech array (avoiding duplicates)
func (p *Payload) AddPrimaryTech(tech string) {
	// Avoid duplicates
	for _, t := range p.Tech {
		if t == tech {
			return
		}
	}
	p.Tech = append(p.Tech, tech)
}

// HasPrimaryTech checks if a technology is in the primary tech array
func (p *Payload) HasPrimaryTech(tech string) bool {
	for _, t := range p.Tech {
		if t == tech {
			return true
		}
	}
	return false
}

// AddLicense adds a license to the payload (like TypeScript addLicenses)
func (p *Payload) AddLicense(license License) {
	// Avoid duplicates (like TypeScript Set behavior)
	for _, existing := range p.Licenses {
		if existing.LicenseName == license.LicenseName {
			return
		}
	}

	p.Licenses = append(p.Licenses, license)
}

// AddEdges adds an edge to another payload (like TypeScript addEdges)
func (p *Payload) AddEdges(target *Payload) {
	edge := Edge{
		Target: target,
		Read:   true,
		Write:  true,
	}

	p.Edges = append(p.Edges, edge)
}

// AddDependency adds a dependency with deduplication
func (p *Payload) AddDependency(dep Dependency) {
	if !p.containsDependency(dep) {
		p.Dependencies = append(p.Dependencies, dep)
	}
}

// DetectLanguage detects the language from a file name using a LanguageDetector
// This is a convenience method that delegates to the language detector
// Deprecated: Use LanguageDetector directly for better modularity
func (p *Payload) DetectLanguage(filename string, content []byte) {
	// Try detection by extension first (fast path)
	lang, safe := enry.GetLanguageByExtension(filename)

	// If not safe (ambiguous extension), use content analysis for better accuracy
	if !safe && lang != "" && len(content) > 0 {
		lang = enry.GetLanguage(filepath.Base(filename), content)
	}

	// If no result from extension, try by filename (handles special files like Makefile, Dockerfile)
	if lang == "" {
		lang, _ = enry.GetLanguageByFilename(filename)
	}

	// Add language if detected
	if lang != "" {
		p.AddLanguage(lang)
	}
}

// GetFullPath returns the full path as a string
func (p *Payload) GetFullPath() string {
	return strings.Join(p.Path, "/")
}

// String returns a string representation
func (p *Payload) String() string {
	techStr := "[]"
	if len(p.Tech) > 0 {
		techStr = fmt.Sprintf("%v", p.Tech)
	}
	return fmt.Sprintf("Payload{id:%s, name:%s, tech:%s, techs:%v}",
		p.ID, p.Name, techStr, p.Techs)
}
