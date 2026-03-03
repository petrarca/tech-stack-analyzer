package parsers

import (
	"path/filepath"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// DotenvDetector handles .env.example file detection
type DotenvDetector struct {
	provider types.Provider
	rules    []types.Rule
}

// NewDotenvDetector creates a new dotenv detector
func NewDotenvDetector(provider types.Provider, rules []types.Rule) *DotenvDetector {
	return &DotenvDetector{
		provider: provider,
		rules:    rules,
	}
}

// DetectInDotEnv detects technologies from .env.example files
// Returns a virtual payload that gets merged into the parent
func (d *DotenvDetector) DetectInDotEnv(files []types.File, currentPath string, basePath string) *types.Payload {
	file := d.findDotenvFile(files)
	if file == nil {
		return nil
	}

	content, err := d.provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	relativeFilePath := d.getRelativeFilePath(basePath, currentPath, file.Name)
	payload := types.NewPayloadWithPath("virtual", relativeFilePath)

	d.scanEnvVariables(string(content), payload)
	return payload
}

func (d *DotenvDetector) findDotenvFile(files []types.File) *types.File {
	const dotenvFile = ".env.example"
	for _, file := range files {
		if file.Name == dotenvFile {
			return &file
		}
	}
	return nil
}

func (d *DotenvDetector) getRelativeFilePath(basePath, currentPath, fileName string) string {
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, fileName))
	if relativeFilePath == "." {
		return "/"
	}
	return "/" + relativeFilePath
}

func (d *DotenvDetector) scanEnvVariables(content string, payload *types.Payload) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		varName := d.extractVarName(line)
		if varName == "" {
			continue
		}
		d.matchVarAgainstRules(varName, payload)
	}
}

func (d *DotenvDetector) extractVarName(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}

	parts := strings.SplitN(line, "=", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func (d *DotenvDetector) matchVarAgainstRules(varName string, payload *types.Payload) {
	lowerVarName := strings.ToLower(varName)
	for _, rule := range d.rules {
		if d.matchesRule(lowerVarName, varName, rule, payload) {
			break
		}
	}
}

func (d *DotenvDetector) matchesRule(lowerVarName, varName string, rule types.Rule, payload *types.Payload) bool {
	if len(rule.DotEnv) == 0 {
		return false
	}

	for _, pattern := range rule.DotEnv {
		if strings.Contains(lowerVarName, strings.ToLower(pattern)) {
			payload.AddTech(rule.Tech, rule.Tech+" matched env: "+varName)
			return true
		}
	}
	return false
}
