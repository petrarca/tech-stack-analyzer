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

	// Find .podspec file for license extraction
	var podspecFile *types.File
	for i, file := range files {
		if strings.HasSuffix(file.Name, ".podspec") {
			podspecFile = &files[i]
			break
		}
	}

	// Process Podfile files first (higher priority for project name)
	for _, file := range files {
		if file.Name != "Podfile" {
			continue
		}

		payload := d.processPodfile(file, currentPath, basePath, provider, depDetector)
		if payload != nil {
			if podspecFile != nil {
				d.addPodspecLicense(payload, *podspecFile, currentPath, provider)
			}
			payloads = append(payloads, payload)
		}
	}

	// Process Podfile.lock files
	for _, file := range files {
		if file.Name != "Podfile.lock" {
			continue
		}

		payload := d.processPodfileLock(file, currentPath, basePath, provider, depDetector)
		if payload != nil {
			if podspecFile != nil && len(payload.Licenses) == 0 {
				d.addPodspecLicense(payload, *podspecFile, currentPath, provider)
			}
			payloads = append(payloads, payload)
		}
	}

	return payloads
}

// processPodfile processes a single Podfile
func (d *Detector) processPodfile(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	// Read Podfile
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Parse Podfile using CocoaPodsParser
	dependencies := d.cocoapodsParser.ParsePodfile(string(content))
	if len(dependencies) == 0 {
		return nil
	}

	// Extract dependency names for matching
	depNames := make([]string, len(dependencies))
	for i, dep := range dependencies {
		depNames[i] = dep.Name
	}

	// Create payload with relative path
	relativeFilePath := d.getRelativeFilePath(basePath, currentPath, file.Name)
	payload := types.NewPayloadWithPath("CocoaPods", relativeFilePath)

	// Set tech field to cocoapods
	payload.AddPrimaryTech("cocoapods")

	// Match dependencies against rules
	depDetector.ApplyMatchesToPayload(payload, depDetector.MatchDependencies(depNames, "cocoapods"))

	// Add dependencies to payload
	payload.Dependencies = append(payload.Dependencies, dependencies...)

	return payload
}

// processPodfileLock processes a single Podfile.lock
func (d *Detector) processPodfileLock(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	// Read Podfile.lock
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Parse Podfile.lock using CocoaPodsParser
	dependencies := d.cocoapodsParser.ParsePodfileLock(string(content))
	if len(dependencies) == 0 {
		return nil
	}

	// Extract dependency names for matching
	depNames := make([]string, len(dependencies))
	for i, dep := range dependencies {
		depNames[i] = dep.Name
	}

	// Create payload with relative path
	relativeFilePath := d.getRelativeFilePath(basePath, currentPath, file.Name)
	payload := types.NewPayloadWithPath("CocoaPods", relativeFilePath)

	// Set tech field to cocoapods
	payload.AddPrimaryTech("cocoapods")

	// Match dependencies against rules
	depDetector.ApplyMatchesToPayload(payload, depDetector.MatchDependencies(depNames, "cocoapods"))

	// Add dependencies to payload
	payload.Dependencies = append(payload.Dependencies, dependencies...)

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
