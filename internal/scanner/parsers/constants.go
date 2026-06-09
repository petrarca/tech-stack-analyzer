// Package parsers provides parsers for various dependency management files.
//
// This file contains constants for dependency types and metadata sources
// used across all parsers to ensure consistency and prevent typos.
package parsers

// Dependency type constants define the type field for dependencies.
// These constants ensure consistency across all parsers and prevent typos.
//
// Values follow the Package URL (PURL) type vocabulary
// (https://github.com/package-url/purl-spec) so that emitted dependency
// types map directly onto PURL types when producing an SBOM. The constant
// names stay language-oriented for readability; the string values are the
// canonical PURL types (e.g. Ruby -> "gem", Python -> "pypi", PHP ->
// "composer", Rust -> "cargo", Go -> "golang").
//
// Identifiers without a PURL type (deno, terraform, githubAction, delphi,
// node) keep their descriptive value and are not emitted as SBOM package
// components.
const (
	// JavaScript/TypeScript ecosystem
	DependencyTypeNpm  = "npm"
	DependencyTypeDeno = "deno"

	// Python ecosystem (PURL: pypi)
	DependencyTypePython = "pypi"

	// Ruby ecosystem (PURL: gem)
	DependencyTypeRuby = "gem"

	// Go ecosystem (PURL: golang)
	DependencyTypeGolang = "golang"

	// Rust ecosystem (PURL: cargo)
	DependencyTypeRust = "cargo"

	// JVM ecosystem (PURL: maven; Gradle artifacts use Maven coordinates)
	DependencyTypeMaven  = "maven"
	DependencyTypeGradle = "gradle"

	// PHP ecosystem (PURL: composer)
	DependencyTypePHP = "composer"

	// .NET ecosystem (PURL: nuget)
	DependencyTypeNuget = "nuget"

	// C/C++ ecosystem (PURL: conan)
	DependencyTypeConan = "conan"

	// iOS/macOS ecosystem (PURL: cocoapods)
	DependencyTypeCocoapods = "cocoapods"

	// Dart/Flutter ecosystem (PURL: pub)
	DependencyTypeDart = "pub"

	// Elixir/Erlang ecosystem (PURL: hex)
	DependencyTypeElixir = "hex"

	// Swift Package Manager ecosystem (PURL: swift)
	DependencyTypeSwift = "swift"

	// Perl/CPAN ecosystem (PURL: cpan)
	DependencyTypePerl = "cpan"

	// R ecosystem (PURL: cran)
	DependencyTypeR = "cran"

	// Infrastructure as Code (no PURL type)
	DependencyTypeTerraform = "terraform"

	// CI/CD (no PURL type)
	DependencyTypeGitHubAction = "githubAction"

	// Containers (PURL: docker)
	DependencyTypeDocker = "docker"

	// Other (no PURL type)
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
	MetadataSourceVcxproj     = ".vcxproj"

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
