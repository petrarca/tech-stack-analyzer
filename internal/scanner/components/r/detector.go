// Package r detects R projects (renv.lock).
package r

import (
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/providers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Detector implements R component detection.
type Detector struct{}

// Name returns the detector name.
func (d *Detector) Name() string { return "r" }

// Detect scans for R projects (renv.lock).
func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	for _, file := range files {
		if file.Name == "renv.lock" {
			if payload := d.detectRenv(file, currentPath, basePath, provider, depDetector); payload != nil {
				return []*types.Payload{payload}
			}
		}
	}
	return nil
}

func (d *Detector) detectRenv(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	projectName := filepath.Base(currentPath)

	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}

	payload := types.NewPayloadWithPath(projectName, relativeFilePath)
	payload.SetComponentType("r")
	payload.AddPrimaryTech("r")
	payload.SetComponentProperty("r", "package_name", projectName)
	payload.AddTech("renv", "matched file: renv.lock")

	dependencies := parsers.NewRenvParser().ParseRenvLock(string(content))

	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}
	if len(dependencies) > 0 {
		depDetector.ApplyMatchesToPayload(payload, depDetector.MatchDependencies(depNames, parsers.DependencyTypeR))
		payload.Dependencies = dependencies
	}

	// Attach the dependency graph (no-op unless the mode is on).
	components.AttachLockfileGraph(payload, currentPath, provider, lockfileGraphProducers)

	return payload
}

// lockfileGraphProducers lists the R lockfile. renv.lock states the package
// graph (Requirements arrays).
var lockfileGraphProducers = []components.LockfileGraphProducer{
	{Lockfile: "renv.lock", Parse: parsers.ParseRenvLockGraph},
}

func init() {
	components.Register(&Detector{})
	providers.Register(&providers.PackageProvider{
		DependencyType:      parsers.DependencyTypeR,
		ExtractPackageNames: providers.SinglePropertyExtractor("r", "package_name"),
	})
}
