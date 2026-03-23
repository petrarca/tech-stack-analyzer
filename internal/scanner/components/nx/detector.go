// Package nx detects Nx monorepo library and application projects via project.json.
// Nx is a framework-agnostic build system for monorepos. Each library or application
// in an Nx workspace has a project.json that defines its build targets and metadata.
// This detector creates a component for each Nx project, enabling subsystem-level
// analysis of large Angular/React/Node monorepos without requiring a separate package.json.
package nx

import (
	"encoding/json"
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Detector implements Nx project detection via project.json
type Detector struct{}

// Name returns the detector name
func (d *Detector) Name() string {
	return "nx"
}

// Detect scans for Nx project.json files and returns a component for each one.
// Only files named exactly "project.json" are considered; they must contain a valid
// projectType ("library" or "application") to avoid false positives from other
// project.json conventions.
func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, _ components.DependencyDetector) []*types.Payload {
	var payloads []*types.Payload

	for _, file := range files {
		if file.Name != "project.json" {
			continue
		}
		if payload := processProjectJSON(file, currentPath, basePath, provider); payload != nil {
			payloads = append(payloads, payload)
		}
	}

	return payloads
}

// nxProjectJSON is the subset of project.json fields we care about
type nxProjectJSON struct {
	Name        string `json:"name"`
	ProjectType string `json:"projectType"` // "library" or "application"
}

func processProjectJSON(file types.File, currentPath, basePath string, provider types.Provider) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	var project nxProjectJSON
	if err := json.Unmarshal(content, &project); err != nil {
		return nil
	}

	// Only detect valid Nx project types — avoids false positives from other
	// tools that also use project.json (e.g. some .NET project formats)
	if project.ProjectType != "library" && project.ProjectType != "application" {
		return nil
	}

	// Name is required
	if project.Name == "" {
		return nil
	}

	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}

	payload := types.NewPayloadWithPath(project.Name, relativeFilePath)
	payload.SetComponentType("nx")

	return payload
}

func init() {
	components.Register(&Detector{})
}
