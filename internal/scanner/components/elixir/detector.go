// Package elixir detects Elixir/Mix projects (mix.exs + mix.lock).
package elixir

import (
	"path/filepath"
	"regexp"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/providers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Detector implements Elixir/Mix component detection.
type Detector struct{}

// Name returns the detector name.
func (d *Detector) Name() string { return "elixir" }

// mixAppRe extracts the OTP app name from mix.exs: `app: :my_app`.
var mixAppRe = regexp.MustCompile(`app:\s*:([A-Za-z0-9_]+)`)

// Detect scans for Elixir projects (mix.exs).
func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	for _, file := range files {
		if file.Name == "mix.exs" {
			if payload := d.detectMix(file, currentPath, basePath, provider, depDetector); payload != nil {
				return []*types.Payload{payload}
			}
		}
	}
	return nil
}

func (d *Detector) detectMix(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	projectName := filepath.Base(currentPath)
	if m := mixAppRe.FindStringSubmatch(string(content)); m != nil {
		projectName = m[1]
	}

	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}

	payload := types.NewPayloadWithPath(projectName, relativeFilePath)
	payload.SetComponentType("elixir")
	payload.AddPrimaryTech("elixir")
	payload.SetComponentProperty("elixir", "app_name", projectName)
	payload.AddTech("mix", "matched file: mix.exs")

	// Dependencies come from mix.lock (resolved versions).
	var dependencies []types.Dependency
	if components.UseLockFiles() {
		if lockContent, lerr := provider.ReadFile(filepath.Join(currentPath, "mix.lock")); lerr == nil && len(lockContent) > 0 {
			dependencies = parsers.NewElixirParser().ParseMixLock(string(lockContent))
		}
	}

	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}
	if len(dependencies) > 0 {
		depDetector.ApplyMatchesToPayload(payload, depDetector.MatchDependencies(depNames, parsers.DependencyTypeElixir))
		payload.Dependencies = dependencies
	}

	// Attach the dependency graph (no-op unless the mode is on and mix.lock
	// is present).
	components.AttachLockfileGraph(payload, currentPath, provider, lockfileGraphProducers)

	return payload
}

// lockfileGraphProducers lists the Elixir lockfile. mix.lock states per-package
// dependencies, so it is self-describing (no manifest needed for full mode).
var lockfileGraphProducers = []components.LockfileGraphProducer{
	{Lockfile: "mix.lock", Parse: parsers.ParseMixLockGraph},
}

func init() {
	components.Register(&Detector{})
	providers.Register(&providers.PackageProvider{
		DependencyType:      parsers.DependencyTypeElixir,
		ExtractPackageNames: providers.SinglePropertyExtractor("elixir", "app_name"),
	})
}
