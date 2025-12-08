package parsers

import (
	"regexp"
	"strings"
)

// DockerComposeParser handles docker-compose.yml/yaml parsing
type DockerComposeParser struct{}

// NewDockerComposeParser creates a new Docker Compose parser
func NewDockerComposeParser() *DockerComposeParser {
	return &DockerComposeParser{}
}

// DockerService represents a service in docker-compose
type DockerService struct {
	Name          string
	Image         string
	ContainerName string
}

// ParseDockerCompose parses docker-compose.yml/yaml and extracts services
func (p *DockerComposeParser) ParseDockerCompose(content string) []DockerService {
	lines := strings.Split(content, "\n")

	parser := &dockerComposeState{
		services:           []DockerService{},
		inServices:         false,
		servicesIndent:     0,
		serviceRegex:       regexp.MustCompile(`^(\s*)([\w-]+):`), // Support hyphens in service names
		imageRegex:         regexp.MustCompile(`^(\s*)image:\s*(.+)`),
		containerNameRegex: regexp.MustCompile(`^(\s*)container_name:\s*(.+)`),
	}

	for _, line := range lines {
		parser.parseLine(line)
	}

	// Save last service if exists
	parser.saveCurrentService()

	return parser.services
}

// dockerComposeState holds the parsing state
type dockerComposeState struct {
	services           []DockerService
	inServices         bool
	servicesIndent     int
	currentService     DockerService
	currentIndent      int
	serviceRegex       *regexp.Regexp
	imageRegex         *regexp.Regexp
	containerNameRegex *regexp.Regexp
}

// parseLine processes a single line of docker-compose content
func (s *dockerComposeState) parseLine(line string) {
	trimmedLine := strings.TrimSpace(line)

	// Skip empty lines and comments
	if s.shouldSkipLine(trimmedLine) {
		return
	}

	// Check for services section
	if trimmedLine == "services:" {
		s.inServices = true
		s.servicesIndent = len(line) - len(trimmedLine)
		return
	}

	// Check if we're leaving services section
	if s.inServices && s.isLeavingServices(line, trimmedLine) {
		s.saveCurrentService()
		s.inServices = false
		return
	}

	if !s.inServices {
		return
	}

	// Parse service definition
	if s.parseServiceDefinition(line) {
		return
	}

	// Parse service properties
	s.parseServiceProperties(line)
}

// shouldSkipLine checks if a line should be skipped
func (s *dockerComposeState) shouldSkipLine(trimmedLine string) bool {
	return trimmedLine == "" || strings.HasPrefix(trimmedLine, "#")
}

// isLeavingServices checks if we're leaving the services section
func (s *dockerComposeState) isLeavingServices(line, trimmedLine string) bool {
	if !strings.Contains(trimmedLine, ":") {
		return false
	}

	lineIndent := len(line) - len(trimmedLine)
	return lineIndent <= s.servicesIndent && trimmedLine != "services:"
}

// parseServiceDefinition tries to parse a service definition
func (s *dockerComposeState) parseServiceDefinition(line string) bool {
	matches := s.serviceRegex.FindStringSubmatch(line)
	if len(matches) <= 2 {
		return false
	}

	indent := len(matches[1])
	if indent != s.servicesIndent+2 {
		return false
	}

	// Save previous service if exists
	s.saveCurrentService()

	// Start new service
	s.currentService = DockerService{Name: matches[2]}
	s.currentIndent = indent
	return true
}

// parseServiceProperties parses image and container_name properties
func (s *dockerComposeState) parseServiceProperties(line string) {
	if s.currentService.Name == "" {
		return
	}

	if matches := s.imageRegex.FindStringSubmatch(line); len(matches) > 2 {
		if s.isValidPropertyIndent(matches[1]) {
			image := strings.TrimSpace(matches[2])
			s.currentService.Image = s.trimQuotes(image)
		}
	} else if matches := s.containerNameRegex.FindStringSubmatch(line); len(matches) > 2 {
		if s.isValidPropertyIndent(matches[1]) {
			containerName := strings.TrimSpace(matches[2])
			s.currentService.ContainerName = s.trimQuotes(containerName)
		}
	}
}

// isValidPropertyIndent checks if property is properly indented
func (s *dockerComposeState) isValidPropertyIndent(indentStr string) bool {
	return len(indentStr) > s.currentIndent
}

// trimQuotes removes both single and double quotes from a string
func (s *dockerComposeState) trimQuotes(str string) string {
	// Trim double quotes
	str = strings.Trim(str, `"`)
	// Trim single quotes
	str = strings.Trim(str, `'`)
	return str
}

// saveCurrentService saves the current service if it has an image
func (s *dockerComposeState) saveCurrentService() {
	if s.currentService.Name != "" {
		s.services = append(s.services, s.currentService)
		s.currentService = DockerService{}
	}
}
