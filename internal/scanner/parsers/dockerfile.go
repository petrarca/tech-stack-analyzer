package parsers

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Compile Dockerfile parsing regexes once at package level for performance
var (
	dockerfileFromRegex   = regexp.MustCompile(`(?i)^FROM\s+(?:--platform=\S+\s+)?([^\s]+)(?:\s+AS\s+([^\s]+))?`)
	dockerfileExposeRegex = regexp.MustCompile(`(?i)^EXPOSE\s+(.+)`)
	dockerfilePortRegex   = regexp.MustCompile(`\d+`)
	// ARG declarations contributing build-time defaults usable in a FROM, e.g.
	// "ARG BUILD_IMAGE=node:18-alpine". Only the form with a default value is
	// captured; an ARG with no default cannot resolve a FROM image statically.
	dockerfileArgRegex = regexp.MustCompile(`(?i)^ARG\s+([A-Za-z_][A-Za-z0-9_]*)=(\S+)`)
	// A FROM image that is solely a variable reference: $VAR or ${VAR}.
	dockerfileVarRefRegex = regexp.MustCompile(`^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$`)
)

// DockerfileParser handles Dockerfile parsing
type DockerfileParser struct{}

// DockerfileInfo represents parsed information from a Dockerfile
type DockerfileInfo struct {
	File         string   `json:"file,omitempty"`
	BaseImages   []string `json:"base_images,omitempty"`
	ExposedPorts []int    `json:"exposed_ports,omitempty"`
	MultiStage   bool     `json:"multi_stage,omitempty"`
	Stages       []string `json:"stages,omitempty"`
}

// NewDockerfileParser creates a new Dockerfile parser
func NewDockerfileParser() *DockerfileParser {
	return &DockerfileParser{}
}

// ParseDockerfile parses a Dockerfile and extracts base images, exposed ports, and multi-stage info
func (p *DockerfileParser) ParseDockerfile(content string) *DockerfileInfo {
	info := &DockerfileInfo{
		BaseImages:   []string{},
		ExposedPorts: []int{},
		Stages:       []string{},
	}

	lines := strings.Split(content, "\n")

	// Build-time ARG defaults usable to resolve a "FROM $VAR" image, and the
	// set of named build stages (so a "FROM <stage>" that references a prior
	// stage is not mistaken for a registry image).
	argDefaults := make(map[string]string)
	stageNames := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Record ARG defaults (for resolving variable base images).
		if m := dockerfileArgRegex.FindStringSubmatch(line); m != nil {
			argDefaults[m[1]] = strings.Trim(m[2], `"'`)
			continue
		}

		if matches := dockerfileFromRegex.FindStringSubmatch(line); matches != nil {
			parseDockerfileFrom(info, matches, argDefaults, stageNames)
		}
		if matches := dockerfileExposeRegex.FindStringSubmatch(line); matches != nil {
			info.ExposedPorts = append(info.ExposedPorts, parseDockerfileExpose(matches[1])...)
		}
	}

	// Return nil if no useful information was found.
	if len(info.BaseImages) == 0 && len(info.ExposedPorts) == 0 {
		return nil
	}
	return info
}

// parseDockerfileFrom records the stage name and resolvable base image from a
// matched FROM statement onto info.
func parseDockerfileFrom(info *DockerfileInfo, matches []string, argDefaults map[string]string, stageNames map[string]bool) {
	image := resolveDockerImageRef(matches[1], argDefaults, stageNames)

	// Register the stage name regardless of whether the image resolved, so
	// later "FROM <stage>" references are recognized.
	if len(matches) > 2 && matches[2] != "" {
		stageName := matches[2]
		info.Stages = append(info.Stages, stageName)
		info.MultiStage = true
		stageNames[strings.ToLower(stageName)] = true
	}

	// Only record a real, resolvable registry image as a base image. A
	// reference to a prior stage or an unresolvable variable is not a package
	// and must not become an SBOM component.
	if image != "" {
		info.BaseImages = append(info.BaseImages, image)
	}
}

// parseDockerfileExpose extracts the port numbers from an EXPOSE argument.
func parseDockerfileExpose(portsStr string) []int {
	var ports []int
	for _, portStr := range dockerfilePortRegex.FindAllString(portsStr, -1) {
		if port, err := strconv.Atoi(portStr); err == nil {
			ports = append(ports, port)
		}
	}
	return ports
}

// resolveDockerImageRef normalizes a FROM image reference into a concrete
// registry image, or "" when it is not a real base image:
//   - "$VAR" / "${VAR}" -> the ARG default if declared, else "" (unresolvable).
//   - a name matching a prior build stage -> "" (it references a local stage,
//     not a registry image).
//
// A plain image reference (with or without a tag/digest) is returned unchanged.
func resolveDockerImageRef(ref string, argDefaults map[string]string, stageNames map[string]bool) string {
	if m := dockerfileVarRefRegex.FindStringSubmatch(ref); m != nil {
		if def, ok := argDefaults[m[1]]; ok && def != "" {
			ref = def
		} else {
			return "" // unresolvable build-arg image
		}
	}
	if stageNames[strings.ToLower(ref)] {
		return "" // references a prior build stage, not an image
	}
	// A reference still containing an unresolved variable is not usable.
	if strings.Contains(ref, "$") {
		return ""
	}
	return ref
}

// CreateDependencies creates dependency objects from Dockerfile base images
func (p *DockerfileParser) CreateDependencies(info *DockerfileInfo) []types.Dependency {
	if info == nil || len(info.BaseImages) == 0 {
		return nil
	}

	dependencies := make([]types.Dependency, 0, len(info.BaseImages))
	for _, baseImage := range info.BaseImages {
		imageName, imageVersion := p.parseImage(baseImage)
		dependencies = append(dependencies, types.Dependency{
			Type:     DependencyTypeDocker,
			Name:     imageName,
			Version:  imageVersion,
			Scope:    types.ScopeBuild,
			Direct:   true,
			Metadata: types.NewMetadata(MetadataSourceDockerfile),
		})
	}
	return dependencies
}

// parseImage splits a Docker image reference into name and version
func (p *DockerfileParser) parseImage(image string) (string, string) {
	parts := strings.Split(image, ":")
	name := parts[0]
	version := "latest"
	if len(parts) > 1 {
		version = parts[1]
	}
	return name, version
}
