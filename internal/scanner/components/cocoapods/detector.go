package cocoapods

import (
	"path/filepath"
	"regexp"
	"strings"

	licensenormalizer "github.com/petrarca/tech-stack-analyzer/internal/license"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Detector implements CocoaPods component detection
type Detector struct {
	cocoapodsParser *parsers.CocoaPodsParser
}

// Name returns the detector name
func (d *Detector) Name() string {
	return "cocoapods"
}

// Detect scans for CocoaPods projects (Podfile, Podfile.lock, .podspec)
func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var payloads []*types.Payload

	// Initialize parser if not already done
	if d.cocoapodsParser == nil {
		d.cocoapodsParser = parsers.NewCocoaPodsParser()
	}

	podspecFiles, hasPodfile := classifyCocoaPodsFiles(files)

	// Process Podfile files first (higher priority for project name), then
	// Podfile.lock, then standalone .podspec files when no Podfile is present.
	payloads = append(payloads, d.processPodfiles(files, podspecFiles, currentPath, basePath, provider, depDetector)...)
	payloads = append(payloads, d.processPodfileLocks(files, podspecFiles, currentPath, basePath, provider, depDetector)...)
	if !hasPodfile {
		for _, file := range podspecFiles {
			if payload := d.processPodspec(file, currentPath, basePath, provider, depDetector); payload != nil {
				payloads = append(payloads, payload)
			}
		}
	}

	return payloads
}

// classifyCocoaPodsFiles collects the .podspec files and reports whether a
// Podfile or Podfile.lock is present in the directory.
func classifyCocoaPodsFiles(files []types.File) (podspecFiles []types.File, hasPodfile bool) {
	for i, file := range files {
		if strings.HasSuffix(file.Name, ".podspec") {
			podspecFiles = append(podspecFiles, files[i])
		}
		if file.Name == "Podfile" || file.Name == "Podfile.lock" {
			hasPodfile = true
		}
	}
	return podspecFiles, hasPodfile
}

// processPodfiles builds a payload for each Podfile, attaching the podspec
// license when one is available.
func (d *Detector) processPodfiles(files, podspecFiles []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var payloads []*types.Payload
	for _, file := range files {
		if file.Name != "Podfile" {
			continue
		}
		payload := d.processPodfile(file, currentPath, basePath, provider, depDetector)
		if payload == nil {
			continue
		}
		if len(podspecFiles) > 0 {
			d.addPodspecLicense(payload, podspecFiles[0], currentPath, provider)
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

// processPodfileLocks builds a payload for each Podfile.lock, attaching the
// podspec license as a fallback when the lock yielded none.
func (d *Detector) processPodfileLocks(files, podspecFiles []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var payloads []*types.Payload
	for _, file := range files {
		if file.Name != "Podfile.lock" {
			continue
		}
		payload := d.processPodfileLock(file, currentPath, basePath, provider, depDetector)
		if payload == nil {
			continue
		}
		if len(podspecFiles) > 0 && len(payload.Licenses) == 0 {
			d.addPodspecLicense(payload, podspecFiles[0], currentPath, provider)
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

// processPodfile processes a single Podfile
func (d *Detector) processPodfile(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}
	return d.buildPayload(file, string(content), currentPath, basePath, depDetector,
		d.cocoapodsParser.ParsePodfile, "")
}

// processPodfileLock processes a single Podfile.lock
func (d *Detector) processPodfileLock(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}
	payload := d.buildPayload(file, string(content), currentPath, basePath, depDetector,
		d.cocoapodsParser.ParsePodfileLock, "")
	if payload != nil {
		// Attach the dependency graph (no-op unless the mode is on).
		components.AttachLockfileGraph(payload, currentPath, provider, cocoapodsGraphProducers)
	}
	return payload
}

// cocoapodsGraphProducers lists the CocoaPods lockfile. Podfile.lock PODS
// section states each pod's dependencies; DEPENDENCIES gives the direct pods.
var cocoapodsGraphProducers = []components.LockfileGraphProducer{
	{Lockfile: "Podfile.lock", Parse: parsers.ParsePodfileLockGraph},
}

// processPodspec processes a single .podspec file for dependencies and license
func (d *Detector) processPodspec(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}
	contentStr := string(content)
	return d.buildPayload(file, contentStr, currentPath, basePath, depDetector,
		d.cocoapodsParser.ParsePodspec, extractPodspecLicense(contentStr))
}

// buildPayload is the shared payload construction logic for all CocoaPods file types.
// parseFunc extracts dependencies from the file content; licenseStr is non-empty only for podspec.
func (d *Detector) buildPayload(
	file types.File,
	content, currentPath, basePath string,
	depDetector components.DependencyDetector,
	parseFunc func(string) []types.Dependency,
	licenseStr string,
) *types.Payload {
	dependencies := parseFunc(content)
	if len(dependencies) == 0 {
		return nil
	}

	depNames := make([]string, len(dependencies))
	for i, dep := range dependencies {
		depNames[i] = dep.Name
	}

	relativeFilePath := d.getRelativeFilePath(basePath, currentPath, file.Name)
	payload := types.NewPayloadWithPath("CocoaPods", relativeFilePath)
	payload.SetComponentType("cocoapods")
	payload.AddPrimaryTech("cocoapods")

	depDetector.ApplyMatchesToPayload(payload, depDetector.MatchDependencies(depNames, "cocoapods"))
	payload.Dependencies = append(payload.Dependencies, dependencies...)

	if licenseStr != "" {
		licensenormalizer.ProcessLicenseExpression(licenseStr, file.Name, payload)
	}

	return payload
}

// getRelativeFilePath returns relative file path for payload
func (d *Detector) getRelativeFilePath(basePath, currentPath, fileName string) string {
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, fileName))
	if relativeFilePath == "." {
		return "/"
	}
	return "/" + relativeFilePath
}

// addPodspecLicense extracts license information from a .podspec file and adds it
// to the payload. Podspec files use patterns like:
//
//	spec.license = { :type => 'MIT', :file => 'LICENSE' }
//	spec.license = 'MIT'
//	s.license = "Apache-2.0"
func (d *Detector) addPodspecLicense(payload *types.Payload, podspecFile types.File, currentPath string, provider types.Provider) {
	content, err := provider.ReadFile(filepath.Join(currentPath, podspecFile.Name))
	if err != nil {
		return
	}

	licenseStr := extractPodspecLicense(string(content))
	if licenseStr != "" {
		licensenormalizer.ProcessLicenseExpression(licenseStr, podspecFile.Name, payload)
	}
}

// extractPodspecLicense extracts the license type from .podspec content.
// Handles both hash format ({ :type => 'MIT' }) and simple string format ('MIT').
func extractPodspecLicense(content string) string {
	// Match hash format: spec.license = { :type => 'MIT', ... }
	hashRe := regexp.MustCompile(`(?m)\.\s*license\s*=\s*\{[^}]*:type\s*=>\s*['"]([^'"]+)['"]`)
	if match := hashRe.FindStringSubmatch(content); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	// Match simple string format: spec.license = 'MIT' or spec.license = "MIT"
	simpleRe := regexp.MustCompile(`(?m)\.\s*license\s*=\s*['"]([^'"]+)['"]`)
	if match := simpleRe.FindStringSubmatch(content); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	return ""
}

func init() {
	// Auto-register this detector
	components.Register(&Detector{})
}
