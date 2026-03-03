package parsers

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Compile Dockerfile parsing regexes once at package level for performance
var (
	dockerfileFromRegex   = regexp.MustCompile(`(?i)^FROM\s+([^\s]+)(?:\s+AS\s+([^\s]+))?`)
	dockerfileExposeRegex = regexp.MustCompile(`(?i)^EXPOSE\s+(.+)`)
	dockerfilePortRegex   = regexp.MustCompile(`\d+`)
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

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse FROM statements
		if matches := dockerfileFromRegex.FindStringSubmatch(line); matches != nil {
			image := matches[1]
			info.BaseImages = append(info.BaseImages, image)

			// Check for multi-stage build (AS keyword)
			if len(matches) > 2 && matches[2] != "" {
				stageName := matches[2]
				info.Stages = append(info.Stages, stageName)
				info.MultiStage = true
			}
		}

		// Parse EXPOSE statements
		if matches := dockerfileExposeRegex.FindStringSubmatch(line); matches != nil {
			portsStr := matches[1]
			// Extract all port numbers from the line
			portMatches := dockerfilePortRegex.FindAllString(portsStr, -1)
			for _, portStr := range portMatches {
				if port, err := strconv.Atoi(portStr); err == nil {
					info.ExposedPorts = append(info.ExposedPorts, port)
				}
			}
		}
	}

	// Return nil if no useful information was found
	if len(info.BaseImages) == 0 && len(info.ExposedPorts) == 0 {
		return nil
	}

	return info
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
