// Package perl detects Perl projects (cpanfile + cpanfile.snapshot).
package perl

import (
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/providers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Detector implements Perl component detection.
type Detector struct{}

// Name returns the detector name.
func (d *Detector) Name() string { return "perl" }

// Detect scans for Perl projects (cpanfile).
func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	for _, file := range files {
		if file.Name == "cpanfile" {
			if payload := d.detectCpanfile(file, currentPath, basePath, provider, depDetector); payload != nil {
				return []*types.Payload{payload}
			}
		}
	}
	return nil
}

func (d *Detector) detectCpanfile(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	projectName := filepath.Base(currentPath)

	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}

	payload := types.NewPayloadWithPath(projectName, relativeFilePath)
	payload.SetComponentType("perl")
	payload.AddPrimaryTech("perl")
	payload.SetComponentProperty("perl", "package_name", projectName)
	payload.AddTech("cpan", "matched file: cpanfile")

	// Dependencies come from cpanfile.snapshot (resolved versions).
	var dependencies []types.Dependency
	if components.UseLockFiles() {
		if lockContent, err := provider.ReadFile(filepath.Join(currentPath, "cpanfile.snapshot")); err == nil && len(lockContent) > 0 {
			dependencies = parsers.NewCpanfileSnapshotParser().ParseCpanfileSnapshot(string(lockContent))
		}
	}

	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}
	if len(dependencies) > 0 {
		depDetector.ApplyMatchesToPayload(payload, depDetector.MatchDependencies(depNames, parsers.DependencyTypePerl))
		payload.Dependencies = dependencies
	}

	// Attach the dependency graph (no-op unless the mode is on and
	// cpanfile.snapshot is present).
	components.AttachLockfileGraph(payload, currentPath, provider, lockfileGraphProducers)

	return payload
}

// lockfileGraphProducers lists the Perl lockfile. cpanfile.snapshot states the
// distribution graph (provides/requires).
var lockfileGraphProducers = []components.LockfileGraphProducer{
	{Lockfile: "cpanfile.snapshot", Parse: parsers.ParseCpanfileSnapshotGraph},
}

func init() {
	components.Register(&Detector{})
	providers.Register(&providers.PackageProvider{
		DependencyType:      parsers.DependencyTypePerl,
		ExtractPackageNames: providers.SinglePropertyExtractor("perl", "package_name"),
	})
}
