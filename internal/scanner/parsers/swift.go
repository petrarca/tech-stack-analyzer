package parsers

import (
	"encoding/json"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// SwiftParser handles Swift Package Manager Package.resolved parsing.
type SwiftParser struct{}

// NewSwiftParser creates a new Swift parser.
func NewSwiftParser() *SwiftParser {
	return &SwiftParser{}
}

// packageResolved is the Package.resolved lockfile. v1 nests pins under
// "object.pins" with "package"/"repositoryURL"; v2/v3 use top-level "pins" with
// "identity"/"location". The graph is a flat pin list (no edges).
type packageResolved struct {
	Version int          `json:"version"`
	Pins    []swiftPin   `json:"pins"`   // v2/v3
	Object  *swiftObject `json:"object"` // v1
}

type swiftObject struct {
	Pins []swiftPin `json:"pins"`
}

type swiftPin struct {
	Identity      string        `json:"identity"`      // v2/v3
	Package       string        `json:"package"`       // v1
	Location      string        `json:"location"`      // v2/v3
	RepositoryURL string        `json:"repositoryURL"` // v1
	State         swiftPinState `json:"state"`
}

type swiftPinState struct {
	Version  string `json:"version"`
	Branch   string `json:"branch"`
	Revision string `json:"revision"`
}

// pins returns the pin list regardless of lockfile version.
func (r packageResolved) pins() []swiftPin {
	if r.Version <= 1 && r.Object != nil {
		return r.Object.Pins
	}
	return r.Pins
}

// name returns the package identity: the v2/v3 identity, else the v1 name
// derived from the repository URL (last path segment, ".git" stripped).
func (p swiftPin) name() string {
	if p.Identity != "" {
		return p.Identity
	}
	url := p.Package
	if p.RepositoryURL != "" {
		url = p.RepositoryURL
	}
	name := strings.TrimSuffix(url, ".git")
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	return strings.ToLower(name)
}

// version returns the resolved version, falling back to the branch when a pin
// is tracked by branch rather than a tagged release.
func (p swiftPin) version() string {
	if p.State.Version != "" {
		return p.State.Version
	}
	return p.State.Branch
}

// ParsePackageResolved parses Package.resolved into resolved dependencies.
func (p *SwiftParser) ParsePackageResolved(content string) []types.Dependency {
	var lock packageResolved
	if err := json.Unmarshal([]byte(content), &lock); err != nil {
		return nil
	}
	var deps []types.Dependency
	for _, pin := range lock.pins() {
		name, version := pin.name(), pin.version()
		if name == "" || version == "" {
			continue
		}
		deps = append(deps, types.Dependency{
			Type:       DependencyTypeSwift,
			Name:       name,
			Version:    version,
			SourceFile: "Package.resolved",
		})
	}
	return deps
}
