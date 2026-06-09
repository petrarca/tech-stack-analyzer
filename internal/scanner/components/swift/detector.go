// Package swift detects Swift Package Manager projects (Package.swift +
// Package.resolved).
package swift

import (
	"path/filepath"
	"regexp"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/providers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Detector implements Swift Package Manager component detection.
type Detector struct{}

// Name returns the detector name.
func (d *Detector) Name() string { return "swift" }

// swiftNameRe extracts the package name from Package.swift: name: "MyPackage".
var swiftNameRe = regexp.MustCompile(`name:\s*"([^"]+)"`)

// Detect scans for Swift Package Manager projects (Package.swift).
func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	for _, file := range files {
		if file.Name == "Package.swift" {
			if payload := d.detectPackage(file, currentPath, basePath, provider, depDetector); payload != nil {
				return []*types.Payload{payload}
			}
		}
	}
	return nil
}

func (d *Detector) detectPackage(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	projectName := filepath.Base(currentPath)
	if m := swiftNameRe.FindStringSubmatch(string(content)); m != nil {
		projectName = m[1]
	}

	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}

	payload := types.NewPayloadWithPath(projectName, relativeFilePath)
	payload.SetComponentType("swift")
	payload.AddPrimaryTech("swift")
	payload.SetComponentProperty("swift", "package_name", projectName)
	payload.AddTech("swiftpm", "matched file: Package.swift")

	// Dependencies come from Package.resolved (resolved versions).
	var dependencies []types.Dependency
	if components.UseLockFiles() {
		if lockContent, lerr := provider.ReadFile(filepath.Join(currentPath, "Package.resolved")); lerr == nil && len(lockContent) > 0 {
			dependencies = parsers.NewSwiftParser().ParsePackageResolved(string(lockContent))
		}
	}

	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}
	if len(dependencies) > 0 {
		depDetector.ApplyMatchesToPayload(payload, depDetector.MatchDependencies(depNames, parsers.DependencyTypeSwift))
		payload.Dependencies = dependencies
	}

	// Attach the dependency graph (no-op unless the mode is on and
	// Package.resolved is present).
	components.AttachLockfileGraph(payload, currentPath, provider, lockfileGraphProducers)

	return payload
}

// lockfileGraphProducers lists the SPM lockfile. Package.resolved is a flat pin
// list (no package-to-package edges), so the producer emits a root-rooted
// closure.
var lockfileGraphProducers = []components.LockfileGraphProducer{
	{Lockfile: "Package.resolved", Parse: parsers.ParsePackageResolvedGraph},
}

func init() {
	components.Register(&Detector{})
	providers.Register(&providers.PackageProvider{
		DependencyType:      parsers.DependencyTypeSwift,
		ExtractPackageNames: providers.SinglePropertyExtractor("swift", "package_name"),
	})
}
