package docker

import (
	"path/filepath"
	"regexp"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Compile regex once at package level for performance
var imageVersionRegex = regexp.MustCompile(`:(.*)`)

type Detector struct{}

func (d *Detector) Name() string {
	return "docker"
}

func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var results []*types.Payload

	// Check for docker-compose.yml or docker-compose.yaml
	dockerComposeRegex := regexp.MustCompile(`^docker-compose(.*)?\.y(a)?ml$`)
	for _, file := range files {
		if dockerComposeRegex.MatchString(file.Name) {
			payload := d.detectDockerCompose(file, currentPath, basePath, provider, depDetector)
			if payload != nil {
				results = append(results, payload)
			}
		}
	}

	// Check for Dockerfile
	for _, file := range files {
		if file.Name == "Dockerfile" || regexp.MustCompile(`^Dockerfile\..+$`).MatchString(file.Name) {
			payload := d.detectDockerfile(file, currentPath, basePath, provider, depDetector)
			if payload != nil {
				results = append(results, payload)
			}
		}
	}

	return results
}

func (d *Detector) detectDockerCompose(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Parse docker-compose file using parser
	composeParser := parsers.NewDockerComposeParser()
	services := composeParser.ParseDockerCompose(string(content))

	if len(services) == 0 {
		return nil
	}

	// Create virtual payload
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}
	payload := types.NewPayloadWithPath("virtual", relativeFilePath)

	// Create child components for each service
	for _, service := range services {
		// Skip images starting with $ (environment variables)
		if len(service.Image) > 0 && service.Image[0] == '$' {
			continue
		}

		// Create dependency using parser
		dependency := composeParser.CreateDependency(service)
		if dependency.Name == "" {
			continue
		}

		// Use container_name if available, otherwise service name
		serviceName := service.ContainerName
		if serviceName == "" {
			serviceName = service.Name
		}

		// Match image name against dependency rules
		matchedTechs := depDetector.MatchDependencies([]string{dependency.Name}, "docker")

		// Determine tech and reasons
		var tech string
		var reasons []string
		for t, r := range matchedTechs {
			tech = t
			reasons = r
			break // Take first match
		}

		if tech == "" {
			tech = "docker"
		}
		if len(reasons) == 0 {
			reasons = []string{"matched: " + dependency.Name}
		}

		// Create child component
		childPayload := types.NewPayloadWithPath(serviceName, relativeFilePath)
		childPayload.AddPrimaryTech(tech)
		childPayload.Dependencies = []types.Dependency{dependency}

		// Add techs and reasons to child
		for _, reason := range reasons {
			childPayload.AddTech(tech, reason)
		}

		// Add child to parent payload
		payload.AddChild(childPayload)
	}

	return payload
}

func (d *Detector) detectDockerfile(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Parse Dockerfile using parser
	dockerfileParser := parsers.NewDockerfileParser()
	dockerfileInfo := dockerfileParser.ParseDockerfile(string(content))

	if dockerfileInfo == nil {
		return nil
	}

	// Create virtual payload
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}
	payload := types.NewPayloadWithPath("virtual", relativeFilePath)

	// Set the file path in dockerfileInfo
	dockerfileInfo.File = relativeFilePath

	// Create dependencies using parser
	payload.Dependencies = parsers.NewDockerfileParser().CreateDependencies(dockerfileInfo)

	// Add Dockerfile info to properties as array (Properties already initialized by NewPayloadWithPath)
	payload.Properties["docker"] = []interface{}{dockerfileInfo}

	// Add reason
	payload.AddTech("docker", "matched file: "+file.Name)
	for _, baseImage := range dockerfileInfo.BaseImages {
		payload.AddDockerReason("base image: " + baseImage)
	}

	return payload
}

// parseImage splits image name and version
func (d *Detector) parseImage(image string) (string, string) {
	// Split on colon to separate name and version
	parts := imageVersionRegex.Split(image, 2)
	if len(parts) == 1 {
		// No version specified
		return parts[0], "latest"
	}
	return parts[0], parts[1]
}

func init() {
	components.Register(&Detector{})
}
