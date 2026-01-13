package cocoapods

import (
	"path/filepath"

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

// Detect scans for CocoaPods projects (Podfile, Podfile.lock)
func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var payloads []*types.Payload

	// Initialize parser if not already done
	if d.cocoapodsParser == nil {
		d.cocoapodsParser = parsers.NewCocoaPodsParser()
	}

	// Process Podfile files first (higher priority for project name)
	for _, file := range files {
		if file.Name != "Podfile" {
			continue
		}

		payload := d.processPodfile(file, currentPath, basePath, provider, depDetector)
		if payload != nil {
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
	matchedTechs := depDetector.MatchDependencies(depNames, "cocoapods")
	for tech, reasons := range matchedTechs {
		for _, reason := range reasons {
			payload.AddTech(tech, reason)
		}
		depDetector.AddPrimaryTechIfNeeded(payload, tech)
	}

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
	matchedTechs := depDetector.MatchDependencies(depNames, "cocoapods")
	for tech, reasons := range matchedTechs {
		for _, reason := range reasons {
			payload.AddTech(tech, reason)
		}
		depDetector.AddPrimaryTechIfNeeded(payload, tech)
	}

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

func init() {
	// Auto-register this detector
	components.Register(&Detector{})
}
