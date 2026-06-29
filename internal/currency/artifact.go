package currency

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// SchemaID is the versioned identifier of the currency artifact schema.
const SchemaID = "stack-analyzer.currency/v1"

// Artifact is the {out}.currency.json document: a dedicated, time-varying view
// of dependency freshness, joined to dependencies by PURL. It is separate from
// the scan output and the SBOM.
type Artifact struct {
	Schema         string       `json:"schema"`
	GeneratedAt    string       `json:"generated_at"`
	Source         string       `json:"source"`
	SourceEndpoint string       `json:"source_endpoint"`
	TTLHours       int          `json:"ttl_hours"`
	Scope          string       `json:"scope"` // "direct" in v1
	Summary        Summary      `json:"summary"`
	Dependencies   []Dependency `json:"dependencies"`
}

// Summary is the aggregate count breakdown over Dependencies.
type Summary struct {
	Total       int `json:"total"`
	Resolved    int `json:"resolved"`
	UpToDate    int `json:"up_to_date"`
	Patch       int `json:"patch"`
	Minor       int `json:"minor"`
	Major       int `json:"major"`
	Unsupported int `json:"unsupported"`
	Unpinned    int `json:"unpinned"`
	Unknown     int `json:"unknown"`
	Errors      int `json:"error"`
	Deprecated  int `json:"deprecated"`
}

// Dependency is one entry in the currency artifact.
type Dependency struct {
	PURL              string `json:"purl"`
	System            string `json:"system"` // deps.dev system, or "" if unsupported
	Name              string `json:"name"`
	Installed         string `json:"installed"`
	Latest            string `json:"latest,omitempty"`
	Currency          Bucket `json:"currency"`
	Direct            bool   `json:"direct"`
	Scope             string `json:"scope,omitempty"`
	IsDeprecated      bool   `json:"is_deprecated,omitempty"`
	LatestPublishedAt string `json:"latest_published_at,omitempty"`
	CheckedAt         string `json:"checked_at,omitempty"`
	Source            string `json:"source,omitempty"` // e.g. "deps.dev"
}

// newArtifact builds an empty artifact with the header fields populated.
func newArtifact(sourceEndpoint string, ttlHours int) *Artifact {
	return &Artifact{
		Schema:         SchemaID,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		Source:         "deps.dev",
		SourceEndpoint: sourceEndpoint,
		TTLHours:       ttlHours,
		Scope:          "direct",
	}
}

// addToSummary updates the summary counters for one classified dependency.
func (a *Artifact) addToSummary(d Dependency) {
	a.Summary.Total++
	switch d.Currency {
	case UpToDate:
		a.Summary.UpToDate++
		a.Summary.Resolved++
	case Patch:
		a.Summary.Patch++
		a.Summary.Resolved++
	case Minor:
		a.Summary.Minor++
		a.Summary.Resolved++
	case Major:
		a.Summary.Major++
		a.Summary.Resolved++
	case Unsupported:
		a.Summary.Unsupported++
	case Unpinned:
		a.Summary.Unpinned++
	case Unknown:
		a.Summary.Unknown++
	case ResolutionError:
		a.Summary.Errors++
	}
	if d.IsDeprecated {
		a.Summary.Deprecated++
	}
}

// Marshal renders the artifact as indented JSON.
func (a *Artifact) Marshal() ([]byte, error) {
	return json.MarshalIndent(a, "", "  ")
}

// WriteFile writes the artifact to path as indented JSON.
func (a *Artifact) WriteFile(path string) error {
	data, err := a.Marshal()
	if err != nil {
		return fmt.Errorf("currency: marshal artifact: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("currency: write %s: %w", path, err)
	}
	return nil
}
