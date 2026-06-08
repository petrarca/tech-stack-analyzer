// Package dart detects Dart/Flutter projects (pubspec.yaml + pubspec.lock).
package dart

import (
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/providers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Detector implements Dart/Flutter component detection.
type Detector struct{}

// Name returns the detector name.
func (d *Detector) Name() string { return "dart" }

// Detect scans for Dart projects (pubspec.yaml).
func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	for _, file := range files {
		if file.Name == "pubspec.yaml" {
			if payload := d.detectPubspec(file, currentPath, basePath, provider, depDetector); payload != nil {
				return []*types.Payload{payload}
			}
		}
	}
	return nil
}

func (d *Detector) detectPubspec(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	dartParser := parsers.NewDartParser()
	projectName := dartParser.ParsePubspecName(string(content))
	if projectName == "" {
		projectName = filepath.Base(currentPath)
	}

	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}

	payload := types.NewPayloadWithPath(projectName, relativeFilePath)
	payload.SetComponentType("dart")
	payload.AddPrimaryTech("dart")
	payload.SetComponentProperty("dart", "package_name", projectName)

	// Prefer pubspec.lock (resolved versions); fall back to pubspec.yaml.
	dependencies := d.extractDependencies(currentPath, string(content), provider, dartParser)

	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}
	payload.AddTech("pub", "matched file: pubspec.yaml")

	if len(dependencies) > 0 {
		depDetector.ApplyMatchesToPayload(payload, depDetector.MatchDependencies(depNames, parsers.DependencyTypeDart))
		payload.Dependencies = dependencies
	}

	// Attach the dependency graph (no-op unless the mode is on and
	// pubspec.lock is present).
	components.AttachLockfileGraph(payload, currentPath, provider, lockfileGraphProducers)

	return payload
}

func (d *Detector) extractDependencies(currentPath, manifest string, provider types.Provider, dartParser *parsers.DartParser) []types.Dependency {
	if components.UseLockFiles() {
		if lockContent, err := provider.ReadFile(filepath.Join(currentPath, "pubspec.lock")); err == nil && len(lockContent) > 0 {
			if deps := dartParser.ParsePubspecLock(string(lockContent)); len(deps) > 0 {
				return deps
			}
		}
	}
	return dartParser.ParsePubspecYAML(manifest)
}

// lockfileGraphProducers lists the Dart lockfile; pubspec.yaml is the manifest
// for direct-mode derivation.
var lockfileGraphProducers = []components.LockfileGraphProducer{
	{Lockfile: "pubspec.lock", Manifest: "pubspec.yaml", Parse: parsers.ParsePubspecLockGraph},
}

func init() {
	components.Register(&Detector{})
	providers.Register(&providers.PackageProvider{
		DependencyType:      parsers.DependencyTypeDart,
		ExtractPackageNames: providers.SinglePropertyExtractor("dart", "package_name"),
	})
}
