package parsers

import (
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ParseYarnLock parses yarn.lock content and returns direct dependencies only
// Enhanced with deps.dev patterns for semantic version preservation and workspace support
func ParseYarnLock(lockContent []byte, packageJSON *PackageJSON) []types.Dependency {
	return ParseYarnLockWithOptions(lockContent, packageJSON, NPMLockFileOptions{})
}

// ParseYarnLockWithOptions parses yarn.lock content with configurable options
func ParseYarnLockWithOptions(lockContent []byte, packageJSON *PackageJSON, options NPMLockFileOptions) []types.Dependency {
	if packageJSON == nil {
		return nil
	}

	// Detect yarn.lock version format
	yarnVersion := DetectYarnVersion(lockContent)

	if yarnVersion == "berry" {
		return parseYarnLockBerryWithOptions(lockContent, packageJSON, options)
	} else {
		return parseYarnLockClassicWithOptions(lockContent, packageJSON, options)
	}
}

// parseYarnLockBerryWithOptions parses yarn.lock v3+ format (Berry) with options
// Enhanced with deps.dev patterns for workspace, git, and patch dependencies
func parseYarnLockBerryWithOptions(lockContent []byte, packageJSON *PackageJSON, options NPMLockFileOptions) []types.Dependency {
	var dependencies []types.Dependency
	filter := NewDependencyFilter(options)

	// Build maps of direct dependency names with their scopes from package.json
	prodDeps := make(map[string]bool)
	devDeps := make(map[string]bool)
	peerDeps := make(map[string]bool)
	optionalDeps := make(map[string]bool)

	for name := range packageJSON.Dependencies {
		prodDeps[name] = true
	}
	for name := range packageJSON.DevDependencies {
		devDeps[name] = true
	}

	// Try to detect peer and optional dependencies if enhanced struct is available
	if enhancedPkg, err := parseEnhancedPackageJSON(lockContent); err == nil {
		for name := range enhancedPkg.PeerDependencies {
			peerDeps[name] = true
		}
		for name := range enhancedPkg.OptionalDependencies {
			optionalDeps[name] = true
		}
	}

	// Add direct dependencies to filter
	filter.AddDirectDependenciesFromMaps(prodDeps, devDeps, peerDeps, optionalDeps)

	content := string(lockContent)

	// Enhanced regex patterns for yarn.lock v3+ format (Berry)
	// Format: "package@npm:^version", "package@workspace:.", "package@patch:..."
	packagePattern := regexp.MustCompile(`^"((?:@[^/]+/)?[^@]+)@([^:]+):([^"]+)"`)
	versionPattern := regexp.MustCompile(`^\s+version:\s+"?([^"\s]+)"?`)
	resolutionPattern := regexp.MustCompile(`^\s+resolution:\s+"([^"]+)"`)

	lines := strings.Split(content, "\n")
	var currentPackage string
	var currentSpecType string
	var currentResolution string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for package declaration with enhanced patterns
		if matches := packagePattern.FindStringSubmatch(line); len(matches) > 3 {
			currentPackage = matches[1]
			currentSpecType = matches[2]
			currentResolution = ""
			continue
		}

		// Check for version line
		if currentPackage != "" {
			if matches := versionPattern.FindStringSubmatch(line); len(matches) > 1 {
				version := parseYarnVersion(matches[1], currentSpecType, currentResolution)

				// Use common filtering to create dependency
				filter.CreateAndAppendDependency("npm", currentPackage, version, "yarn.lock", &dependencies)

				currentPackage = ""
				continue
			}
		}

		// Check for resolution line (for workspace and git dependencies)
		if currentPackage != "" {
			if matches := resolutionPattern.FindStringSubmatch(line); len(matches) > 1 {
				currentResolution = matches[1]
			}
		}
	}

	return dependencies
}

// parseYarnLockClassicWithOptions parses yarn.lock v1/v2 format (Classic) with options
// Enhanced with deps.dev patterns for better dependency analysis
func parseYarnLockClassicWithOptions(lockContent []byte, packageJSON *PackageJSON, options NPMLockFileOptions) []types.Dependency {
	if packageJSON == nil {
		return nil
	}

	var dependencies []types.Dependency
	filter := NewDependencyFilter(options)

	// Build maps of direct dependency names with their scopes from package.json
	prodDeps := make(map[string]bool)
	devDeps := make(map[string]bool)

	for name := range packageJSON.Dependencies {
		prodDeps[name] = true
	}
	for name := range packageJSON.DevDependencies {
		devDeps[name] = true
	}

	// Add direct dependencies to filter
	filter.AddDirectDependenciesFromMaps(prodDeps, devDeps, nil, nil)

	content := string(lockContent)

	// Parse yarn.lock v1/v2 format (Classic)
	// v1 format: "package@npm:^version":\n  version: x.y.z
	// v2 format: "package@^version":\n  version: x.y.z
	packagePattern := regexp.MustCompile(`^"((?:@[^/]+/)?[^@"]+)@[^"]*":`)
	versionPattern := regexp.MustCompile(`^version:\s+"?([^"\s]+)"?`)

	lines := strings.Split(content, "\n")
	var currentPackage string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for package declaration
		if matches := packagePattern.FindStringSubmatch(line); len(matches) > 1 {
			currentPackage = matches[1]
			continue
		}

		// Check for version line
		if currentPackage != "" {
			if matches := versionPattern.FindStringSubmatch(line); len(matches) > 1 {
				version := matches[1]

				// Use common filtering to create dependency
				filter.CreateAndAppendDependency("npm", currentPackage, version, "yarn.lock", &dependencies)

				currentPackage = ""
			}
		}
	}

	return dependencies
}

// parseYarnVersion parses yarn version with semantic version preservation
// Enhanced with deps.dev patterns for workspace, git, and patch dependencies
func parseYarnVersion(version, specType, resolution string) string {
	version = strings.TrimSpace(version)

	// Handle workspace dependencies
	if specType == "workspace" {
		if strings.Contains(resolution, "workspace:") {
			return "workspace"
		}
		return "workspace"
	}

	// Handle patch dependencies
	if specType == "patch" {
		return "patch"
	}

	// Handle npm dependencies with semantic version constraints
	if specType == "npm" {
		if version == "" || version == "*" {
			return "latest"
		}
		// Preserve semantic version constraints (^, ~, >=, <=, ||, -, etc.)
		return version
	}

	// Handle git dependencies
	if specType == "git" {
		if resolution != "" {
			return "git:" + resolution
		}
		return "git"
	}

	// Handle file dependencies
	if specType == "file" {
		return "local"
	}

	// Handle tarball dependencies
	if specType == "tarball" {
		if strings.HasPrefix(resolution, "file:") {
			return "local"
		}
		return "tarball"
	}

	// Default handling
	if version == "" || version == "*" {
		return "latest"
	}

	return version
}

// DetectYarnVersion detects the yarn.lock version format
func DetectYarnVersion(content []byte) string {
	contentStr := string(content)

	// Check for Yarn v1 format (Classic)
	// v1 format: "package@npm:version":\n  version: x.y.z
	if strings.Contains(contentStr, `"@npm:`) && !strings.Contains(contentStr, "__metadata:") {
		return "classic"
	}

	// Check for Yarn Berry (v3+) markers
	if strings.Contains(contentStr, "__metadata:") ||
		strings.Contains(contentStr, "specifiers") ||
		strings.Contains(contentStr, "@workspace:") ||
		strings.Contains(contentStr, "@patch:") {
		return "berry"
	}

	// Default to Yarn Classic (v2/v1)
	return "classic"
}
