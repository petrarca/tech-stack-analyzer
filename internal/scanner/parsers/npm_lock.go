package parsers

import (
	"encoding/json"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// PackageLockJSON represents the structure of package-lock.json
// Enhanced with deps.dev patterns for comprehensive dependency analysis
type PackageLockJSON struct {
	Name            string                 `json:"name"`
	Version         string                 `json:"version"`
	LockfileVersion int                    `json:"lockfileVersion"`
	Packages        map[string]PackageInfo `json:"packages"`
	Dependencies    map[string]PackageInfo `json:"dependencies,omitempty"` // v2 format
}

// PackageInfo represents a package in package-lock.json
// Enhanced with deps.dev patterns for better dependency classification
type PackageInfo struct {
	Version      string                 `json:"version"`
	Resolved     string                 `json:"resolved,omitempty"`
	Link         bool                   `json:"link,omitempty"`
	Dev          bool                   `json:"dev,omitempty"`
	Optional     bool                   `json:"optional,omitempty"`
	Bundled      bool                   `json:"bundled,omitempty"`
	Dependencies map[string]PackageInfo `json:"dependencies,omitempty"`
}

// ParsePackageLockOptions contains configuration options for ParsePackageLock
type ParsePackageLockOptions struct {
	IncludeTransitive bool // Include transitive dependencies (default: false for backward compatibility)
}

// ParsePackageLock parses package-lock.json content and returns comprehensive dependencies
// Enhanced with deps.dev patterns for transitive dependency analysis and scope detection
func ParsePackageLock(content []byte, packageJSON *PackageJSON) []types.Dependency {
	return ParsePackageLockWithOptions(content, packageJSON, nil, ParsePackageLockOptions{})
}

// ParsePackageLockWithOptions parses package-lock.json content with configurable options
// Enhanced with deps.dev patterns for transitive dependency analysis and scope detection
// packageJSONContent is the raw package.json bytes (optional, for peer/optional dependency detection)
func ParsePackageLockWithOptions(content []byte, packageJSON *PackageJSON, packageJSONContent []byte, options ParsePackageLockOptions) []types.Dependency {
	var lockfile PackageLockJSON
	if err := json.Unmarshal(content, &lockfile); err != nil {
		return nil
	}

	// Build dependency scope maps from package.json
	scopeMaps := buildDependencyScopeMaps(packageJSON, packageJSONContent)

	// Handle both v2 (dependencies) and v3+ (packages) lockfile formats
	if len(lockfile.Packages) > 0 {
		return parsePackagesV3(lockfile.Packages, options, scopeMaps)
	}

	if len(lockfile.Dependencies) > 0 {
		return parseDependenciesV2Format(lockfile.Dependencies, options, scopeMaps)
	}

	return nil
}

// buildDependencyScopeMaps builds maps of direct dependency names with their scopes from package.json
func buildDependencyScopeMaps(packageJSON *PackageJSON, content []byte) dependencyScopeMaps {
	maps := dependencyScopeMaps{
		prodDeps:     make(map[string]bool),
		devDeps:      make(map[string]bool),
		peerDeps:     make(map[string]bool),
		optionalDeps: make(map[string]bool),
	}

	if packageJSON == nil {
		return maps
	}

	for name := range packageJSON.Dependencies {
		maps.prodDeps[name] = true
	}
	for name := range packageJSON.DevDependencies {
		maps.devDeps[name] = true
	}

	// Try to detect peer and optional dependencies if enhanced struct is available
	if enhancedPkg, err := parseEnhancedPackageJSON(content); err == nil {
		for name := range enhancedPkg.PeerDependencies {
			maps.peerDeps[name] = true
		}
		for name := range enhancedPkg.OptionalDependencies {
			maps.optionalDeps[name] = true
		}
	}

	return maps
}

// dependencyScopeMaps holds the dependency scope mappings
type dependencyScopeMaps struct {
	prodDeps     map[string]bool
	devDeps      map[string]bool
	peerDeps     map[string]bool
	optionalDeps map[string]bool
}

// parsePackagesV3 parses v3+ format with packages field
func parsePackagesV3(packages map[string]PackageInfo, options ParsePackageLockOptions, maps dependencyScopeMaps) []types.Dependency {
	var dependencies []types.Dependency

	for path, pkg := range packages {
		if shouldSkipPackage(path, pkg, options) {
			continue
		}

		name := extractNameFromNodeModulesPath(path)
		if name == "" {
			continue
		}

		scope := determineScopeFromLockfile(name, pkg, maps.prodDeps, maps.devDeps, maps.peerDeps, maps.optionalDeps)
		isDirect := isDirectDependency(name, maps.prodDeps, maps.devDeps, maps.peerDeps, maps.optionalDeps)

		dependencies = append(dependencies, types.Dependency{
			Type:       "npm",
			Name:       name,
			Version:    pkg.Version,
			Scope:      scope,
			Direct:     isDirect,
			SourceFile: "package-lock.json",
			Metadata:   buildNPMMetadata(name, pkg, maps.peerDeps, maps.optionalDeps),
		})
	}

	return dependencies
}

// shouldSkipPackage determines if a package should be skipped during parsing
func shouldSkipPackage(path string, pkg PackageInfo, options ParsePackageLockOptions) bool {
	if path == "" {
		return true // Skip root package
	}

	if pkg.Bundled {
		return true // Skip bundled dependencies
	}

	// Filter transitive dependencies based on options
	if !options.IncludeTransitive && strings.Count(path, "node_modules/") != 1 {
		return true
	}

	return false
}

// parseDependenciesV2Format parses v2 format with dependencies field
func parseDependenciesV2Format(dependencies map[string]PackageInfo, options ParsePackageLockOptions, maps dependencyScopeMaps) []types.Dependency {
	if options.IncludeTransitive {
		return parseDependenciesV2(dependencies, "", maps.prodDeps, maps.devDeps, maps.peerDeps, maps.optionalDeps)
	}

	return parseTopLevelDependenciesV2(dependencies, maps)
}

// parseTopLevelDependenciesV2 parses only top-level dependencies from v2 format
func parseTopLevelDependenciesV2(dependencies map[string]PackageInfo, maps dependencyScopeMaps) []types.Dependency {
	var result []types.Dependency

	for name, dep := range dependencies {
		if dep.Bundled {
			continue
		}

		scope := determineScopeFromLockfile(name, PackageInfo{Dev: dep.Dev, Optional: dep.Optional}, maps.prodDeps, maps.devDeps, maps.peerDeps, maps.optionalDeps)
		isDirect := isDirectDependency(name, maps.prodDeps, maps.devDeps, maps.peerDeps, maps.optionalDeps)

		result = append(result, types.Dependency{
			Type:       "npm",
			Name:       name,
			Version:    dep.Version,
			Scope:      scope,
			Direct:     isDirect,
			SourceFile: "package-lock.json",
			Metadata:   buildNPMMetadata(name, dep, maps.peerDeps, maps.optionalDeps),
		})
	}

	return result
}

// extractNameFromNodeModulesPath extracts package name from package-lock.json path
// e.g., "node_modules/express" -> "express"
// e.g., "node_modules/@babel/core" -> "@babel/core"
// e.g., "node_modules/express/node_modules/accepts" -> "accepts"
// e.g., "express" -> "express" (handles paths without node_modules/ prefix)
func extractNameFromNodeModulesPath(path string) string {
	var packagePath string

	// Split by "node_modules/" to get all segments
	segments := strings.Split(path, "node_modules/")

	if len(segments) >= 2 {
		// Get the last segment (the actual package)
		packagePath = segments[len(segments)-1]
	} else {
		// No "node_modules/" in path, use the path as-is
		packagePath = path
	}

	packagePath = strings.TrimSpace(packagePath)

	if packagePath == "" {
		return ""
	}

	// Handle scoped packages like @babel/core
	if strings.HasPrefix(packagePath, "@") {
		parts := strings.Split(packagePath, "/")
		if len(parts) >= 2 {
			return strings.Join(parts[:2], "/")
		}
		return packagePath
	}

	// Handle regular packages - just return the first part
	parts := strings.Split(packagePath, "/")
	if len(parts) > 0 {
		return parts[0]
	}

	return packagePath
}

// parseEnhancedPackageJSON attempts to parse package.json with enhanced fields
func parseEnhancedPackageJSON(content []byte) (*PackageJSONEnhanced, error) {
	var enhancedPkg PackageJSONEnhanced
	if err := json.Unmarshal(content, &enhancedPkg); err != nil {
		return nil, err
	}
	return &enhancedPkg, nil
}

// parseDependenciesV2 recursively parses dependencies in v2 lockfile format
// Based on deps.dev patterns for recursive dependency tree traversal
func parseDependenciesV2(
	deps map[string]PackageInfo,
	path string,
	prodDeps, devDeps, peerDeps, optionalDeps map[string]bool,
) []types.Dependency {
	var dependencies []types.Dependency

	for name, dep := range deps {
		// Skip bundled dependencies (deps.dev pattern)
		if dep.Bundled {
			continue
		}

		// Determine scope
		scope := determineScopeFromLockfile(name, dep, prodDeps, devDeps, peerDeps, optionalDeps)
		isDirect := isDirectDependency(name, prodDeps, devDeps, peerDeps, optionalDeps)

		dependencies = append(dependencies, types.Dependency{
			Type:       "npm",
			Name:       name,
			Version:    dep.Version,
			Scope:      scope,
			Direct:     isDirect,
			SourceFile: "package-lock.json",
			Metadata:   buildNPMMetadata(name, dep, peerDeps, optionalDeps),
		})

		// Recursively parse nested dependencies
		if len(dep.Dependencies) > 0 {
			nestedPath := path + "node_modules/" + name + "/"
			nestedDeps := parseDependenciesV2(dep.Dependencies, nestedPath,
				prodDeps, devDeps, peerDeps, optionalDeps)
			dependencies = append(dependencies, nestedDeps...)
		}
	}

	return dependencies
}

// isDirectDependency checks if a dependency is declared in package.json (direct) or pulled in transitively
func isDirectDependency(name string, prodDeps, devDeps, peerDeps, optionalDeps map[string]bool) bool {
	return prodDeps[name] || devDeps[name] || peerDeps[name] || optionalDeps[name]
}

// buildNPMMetadata creates metadata map for NPM dependencies with peer, optional, and bundled flags
func buildNPMMetadata(name string, pkg PackageInfo, peerDeps, optionalDeps map[string]bool) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Add peer flag if true
	if peerDeps[name] {
		metadata["peer"] = true
	}

	// Add optional flag if true
	if optionalDeps[name] || pkg.Optional {
		metadata["optional"] = true
	}

	// Add bundled flag if true
	if pkg.Bundled {
		metadata["bundled"] = true
	}

	// Return nil if no metadata to add
	if len(metadata) == 0 {
		return nil
	}

	return metadata
}

// determineScopeFromLockfile determines the dependency scope based on package.json and lockfile metadata
// Enhanced with deps.dev patterns for accurate scope classification
func determineScopeFromLockfile(
	name string,
	pkg PackageInfo,
	prodDeps, devDeps, peerDeps, optionalDeps map[string]bool,
) string {
	// Check if it's a peer dependency
	if peerDeps[name] {
		return types.ScopePeer
	}

	// Check if it's an optional dependency
	if optionalDeps[name] || pkg.Optional {
		return types.ScopeOptional
	}

	// Check if it's a development dependency
	if devDeps[name] || pkg.Dev {
		return types.ScopeDev
	}

	// Check if it's a production dependency
	if prodDeps[name] {
		return types.ScopeProd
	}

	// Default to production if not explicitly classified
	// This is a reasonable default for transitive dependencies
	return types.ScopeProd
}

// GetLockfileVersion detects the package-lock.json version format
func GetLockfileVersion(content []byte) int {
	var lockfile struct {
		LockfileVersion int `json:"lockfileVersion"`
	}

	if err := json.Unmarshal(content, &lockfile); err != nil {
		return 1 // Default to v1 if parsing fails
	}

	return lockfile.LockfileVersion
}
