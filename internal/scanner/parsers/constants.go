// Package parsers provides parsers for various dependency management files.
//
// This file contains constants for dependency types and metadata sources
// used across all parsers to ensure consistency and prevent typos.
package parsers

// Dependency type constants define the type field for dependencies.
// These constants ensure consistency across all parsers and prevent typos.
const (
	// JavaScript/TypeScript ecosystem
	DependencyTypeNpm  = "npm"
	DependencyTypeDeno = "deno"
	DependencyTypeNode = "node"

	// Python ecosystem
	DependencyTypePython = "python"

	// Ruby ecosystem
	DependencyTypeRuby = "ruby"

	// Go ecosystem
	DependencyTypeGolang = "golang"

	// Rust ecosystem
	DependencyTypeRust = "cargo"

	// JVM ecosystem
	DependencyTypeMaven  = "maven"
	DependencyTypeGradle = "gradle"

	// PHP ecosystem
	DependencyTypePHP = "php"

	// .NET ecosystem
	DependencyTypeDotnet = "dotnet"

	// C/C++ ecosystem
	DependencyTypeConan = "conan"

	// iOS/macOS ecosystem
	DependencyTypeCocoapods = "cocoapods"

	// Infrastructure as Code
	DependencyTypeTerraform = "terraform"

	// CI/CD
	DependencyTypeGitHubAction = "githubAction"

	// Containers
	DependencyTypeDocker = "docker"

	// Other
	DependencyTypeDelphi = "delphi"
)

// Metadata source constants define the source file for dependency metadata.
// These constants ensure consistency across all parsers and prevent typos.
const (
	// JavaScript/TypeScript ecosystem
	MetadataSourcePackageJSON = "package.json"
	MetadataSourcePackageLock = "package-lock.json"
	MetadataSourceYarnLock    = "yarn.lock"
	MetadataSourcePnpmLock    = "pnpm-lock.yaml"
	MetadataSourceDenoJSON    = "deno.json"
	MetadataSourceDenoLock    = "deno.lock"

	// Python ecosystem
	MetadataSourceRequirementsTxt = "requirements.txt"
	MetadataSourcePipfile         = "Pipfile"
	MetadataSourcePoetryLock      = "poetry.lock"

	// Ruby ecosystem
	MetadataSourceGemfile     = "Gemfile"
	MetadataSourceGemfileLock = "Gemfile.lock"

	// Go ecosystem
	MetadataSourceGoMod = "go.mod"
	MetadataSourceGoSum = "go.sum"

	// Rust ecosystem
	MetadataSourceCargoToml = "Cargo.toml"
	MetadataSourceCargoLock = "Cargo.lock"

	// JVM ecosystem
	MetadataSourcePomXML      = "pom.xml"
	MetadataSourceBuildGradle = "build.gradle"

	// PHP ecosystem
	MetadataSourceComposerJSON = "composer.json"
	MetadataSourceComposerLock = "composer.lock"

	// .NET ecosystem
	MetadataSourceCsproj         = ".csproj"
	MetadataSourcePackagesConfig = "packages.config"

	// C/C++ ecosystem
	MetadataSourceConanfile   = "conanfile.txt"
	MetadataSourceConanfilePy = "conanfile.py"

	// Delphi ecosystem
	MetadataSourceDproj = ".dproj"

	// iOS/macOS ecosystem
	MetadataSourcePodfile     = "Podfile"
	MetadataSourcePodfileLock = "Podfile.lock"

	// Infrastructure as Code
	MetadataSourceTerraform = ".tf"

	// CI/CD
	MetadataSourceGitHubWorkflow = ".github/workflows"

	// Containers
	MetadataSourceDockerfile    = "Dockerfile"
	MetadataSourceDockerCompose = "docker-compose.yml"
)
