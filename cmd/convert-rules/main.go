package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Rule represents a converted rule
type Rule struct {
	Tech         string        `yaml:"tech"`
	Name         string        `yaml:"name"`
	Type         string        `yaml:"type"`
	DotEnv       []string      `yaml:"dotenv,omitempty"`
	Dependencies []Dependency  `yaml:"dependencies,omitempty"`
	Files        []string      `yaml:"files,omitempty"`
	Extensions   []string      `yaml:"extensions,omitempty"`
	Detect       *DetectConfig `yaml:"detect,omitempty"`
}

// Dependency represents a dependency pattern as [type, name, version] array
type Dependency [3]string

// DetectConfig represents custom detection configuration
type DetectConfig struct {
	Type    string `yaml:"type"`
	File    string `yaml:"file,omitempty"`
	Pattern string `yaml:"pattern,omitempty"`
	Schema  string `yaml:"schema,omitempty"`
	Extract bool   `yaml:"extract,omitempty"`
}

// Converter handles the conversion process
type Converter struct {
	sourceDir   string
	targetDir   string
	typeMapping map[string]string
	stats       map[string]int
}

// NewConverter creates a new converter instance
func NewConverter(sourceDir, targetDir string) *Converter {
	return &Converter{
		sourceDir:   sourceDir,
		targetDir:   targetDir,
		typeMapping: getTypeMapping(),
		stats:       make(map[string]int),
	}
}

// getTypeMapping returns the mapping from TypeScript types to Go categories
func getTypeMapping() map[string]string {
	return map[string]string{
		"ai":              "ai",
		"analytics":       "analytics",
		"app":             "application",
		"auth":            "security",
		"automation":      "automation",
		"builder":         "build",
		"ci":              "ci",
		"cloud":           "cloud",
		"cms":             "cms",
		"collaboration":   "collaboration",
		"communication":   "communication",
		"crm":             "crm",
		"db":              "database",
		"etl":             "etl",
		"framework":       "framework",
		"hosting":         "hosting",
		"iac":             "infrastructure",
		"iconset":         "ui",
		"language":        "language",
		"linter":          "tool",
		"maps":            "tool",
		"monitoring":      "monitoring",
		"network":         "network",
		"notification":    "notification",
		"orm":             "database",
		"package_manager": "tool",
		"payment":         "payment",
		"queue":           "queue",
		"runtime":         "runtime",
		"saas":            "saas",
		"security":        "security",
		"ssg":             "ssg",
		"storage":         "storage",
		"test":            "test",
		"tool":            "tool",
		"ui":              "ui",
		"ui_framework":    "ui",
		"validation":      "validation",
	}
}

// ConvertAll converts all TypeScript rules to YAML format
func (c *Converter) ConvertAll(limit int, dryRun bool) error {
	// Find all TypeScript files
	tsFiles, err := c.findTypeScriptFiles()
	if err != nil {
		return fmt.Errorf("failed to find TypeScript files: %w", err)
	}

	// Filter out index and type files (except in spec directory)
	var ruleFiles []string
	for _, file := range tsFiles {
		base := filepath.Base(file)
		// Allow index.ts files in spec directory (for complex component rules)
		if base != "index.ts" && base != "types.ts" {
			ruleFiles = append(ruleFiles, file)
		} else if base == "index.ts" && strings.Contains(file, "/spec/") {
			ruleFiles = append(ruleFiles, file)
		}
	}

	if limit > 0 && limit < len(ruleFiles) {
		ruleFiles = ruleFiles[:limit]
	}

	log.Printf("Found %d TypeScript rule files", len(ruleFiles))

	// Create target directories
	if err := c.createTargetDirectories(); err != nil {
		return fmt.Errorf("failed to create target directories: %w", err)
	}

	// Convert each file
	converted := 0
	errors := 0

	for _, file := range ruleFiles {
		rule, err := c.extractRuleFromFile(file)
		if err != nil {
			log.Printf("✗ Error extracting from %s: %v", filepath.Base(file), err)
			errors++
			continue
		}

		if rule == nil {
			log.Printf("- Skipped %s (no rule found)", filepath.Base(file))
			continue
		}

		if !dryRun {
			if err := c.writeRuleToFile(rule, file); err != nil {
				log.Printf("✗ Error writing rule from %s: %v", filepath.Base(file), err)
				errors++
				continue
			}
		}

		category := c.getCategoryForType(rule.Type)
		c.stats[category]++
		converted++
		log.Printf("✓ %s -> %s/%s.yaml", filepath.Base(file), category, rule.Tech)
	}

	log.Printf("Conversion complete: %d converted, %d errors, target: %s", converted, errors, c.targetDir)

	return nil
}

// findTypeScriptFiles finds all .ts files in the source directory
func (c *Converter) findTypeScriptFiles() ([]string, error) {
	var files []string

	err := filepath.Walk(c.sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasSuffix(path, ".ts") && !info.IsDir() {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// createTargetDirectories creates all necessary target directories
func (c *Converter) createTargetDirectories() error {
	categories := c.getAllCategories()

	for _, category := range categories {
		dir := filepath.Join(c.targetDir, "core", category)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// getAllCategories returns all possible categories
func (c *Converter) getAllCategories() []string {
	categories := make(map[string]bool)
	for _, category := range c.typeMapping {
		categories[category] = true
	}

	// Add misc category
	categories["misc"] = true

	var result []string
	for category := range categories {
		result = append(result, category)
	}
	sort.Strings(result)

	return result
}

// extractRuleFromFile extracts rule information from a TypeScript file
func (c *Converter) extractRuleFromFile(filePath string) (*Rule, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	contentStr := string(content)

	// Look for register() call - find the start and extract content manually
	registerStart := strings.Index(contentStr, "register({")
	if registerStart == -1 {
		return nil, nil // No rule found
	}

	// Find the matching closing brace for register
	braceCount := 0
	pos := registerStart + len("register({") - 1 // Position at first {

	for pos < len(contentStr) {
		switch contentStr[pos] {
		case '{':
			braceCount++
		case '}':
			braceCount--
			if braceCount == 0 {
				// Found the matching closing brace
				ruleContent := contentStr[registerStart+len("register(") : pos]
				return c.parseRuleContent(ruleContent), nil
			}
		}
		pos++
	}

	return nil, nil // No matching closing brace found
}

// parseRuleContent parses the extracted rule content
func (c *Converter) parseRuleContent(content string) *Rule {
	// Extract basic fields
	rule := &Rule{}

	if err := c.extractBasicFields(content, rule); err != nil {
		return nil
	}

	// Extract optional fields
	c.extractDotEnv(content, rule)
	c.extractDependencies(content, rule)
	c.extractFiles(content, rule)
	c.extractExtensions(content, rule)
	c.extractCustomDetection(content, rule)

	return rule
}

// extractBasicFields extracts required fields (tech, name, type)
func (c *Converter) extractBasicFields(content string, rule *Rule) error {
	techRegex := regexp.MustCompile(`tech\s*:\s*["']([^"']+)["']`)
	nameRegex := regexp.MustCompile(`name\s*:\s*["']([^"']+)["']`)
	typeRegex := regexp.MustCompile(`type\s*:\s*["']([^"']+)["']`)

	techMatch := techRegex.FindStringSubmatch(content)
	nameMatch := nameRegex.FindStringSubmatch(content)
	typeMatch := typeRegex.FindStringSubmatch(content)

	if techMatch == nil || nameMatch == nil || typeMatch == nil {
		return fmt.Errorf("missing required fields")
	}

	rule.Tech = techMatch[1]
	rule.Name = nameMatch[1]
	rule.Type = typeMatch[1]

	return nil
}

// extractDotEnv extracts dotenv patterns
func (c *Converter) extractDotEnv(content string, rule *Rule) {
	dotenvRegex := regexp.MustCompile(`dotenv\s*:\s*\[([^\]]+)\]`)
	matches := dotenvRegex.FindStringSubmatch(content)

	if len(matches) < 2 {
		return
	}

	items := strings.Split(matches[1], ",")
	for _, item := range items {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, `"'`)
		if item != "" {
			rule.DotEnv = append(rule.DotEnv, item)
		}
	}
}

// extractDependencies extracts dependency patterns
func (c *Converter) extractDependencies(content string, rule *Rule) {
	depRegex := regexp.MustCompile(`(?s)\{\s*type\s*:\s*["']([^"']+)["'][^}]*name\s*:\s*["']([^"']+)["'][^}]*\}`)
	matches := depRegex.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		depType := match[1]
		depName := match[2]

		// Add example based on type and pattern
		example := c.generateExample(depName, depType)

		// Convert regex patterns to proper format
		if c.isRegexPattern(depName) {
			depName = "/" + depName + "/"
		}

		// Create dependency as array: [type, name, version]
		dependency := Dependency{depType, depName, example}

		rule.Dependencies = append(rule.Dependencies, dependency)
	}
}

// extractFiles extracts file patterns from TypeScript rules
func (c *Converter) extractFiles(content string, rule *Rule) {
	// Look for files array: files: ['package.json', 'requirements.txt']
	filesRegex := regexp.MustCompile(`files\s*:\s*\[([^\]]+)\]`)
	match := filesRegex.FindStringSubmatch(content)

	if len(match) > 1 {
		filesContent := match[1]
		// Split by comma and clean up each filename
		parts := strings.Split(filesContent, ",")
		for _, part := range parts {
			file := strings.TrimSpace(part)
			file = strings.Trim(file, `'"`) // Remove quotes
			if file != "" {
				rule.Files = append(rule.Files, file)
			}
		}
	}
}

func (c *Converter) extractExtensions(content string, rule *Rule) {
	// Look for extensions array: extensions: ['.js', '.mjs', '.cjs']
	extensionsRegex := regexp.MustCompile(`extensions\s*:\s*\[([^\]]+)\]`)
	match := extensionsRegex.FindStringSubmatch(content)

	if len(match) > 1 {
		extensionsContent := match[1]
		// Split by comma and clean up each extension
		parts := strings.Split(extensionsContent, ",")
		for _, part := range parts {
			ext := strings.TrimSpace(part)
			ext = strings.Trim(ext, `'"`) // Remove quotes
			if ext != "" {
				rule.Extensions = append(rule.Extensions, ext)
			}
		}
	}
}

// extractCustomDetection extracts custom detection patterns
func (c *Converter) extractCustomDetection(content string, rule *Rule) {
	// Debug: Check if this is NodeJS file
	if strings.Contains(content, "tech: 'nodejs'") {
		fmt.Printf("DEBUG: Processing NodeJS rule\n")
		fmt.Printf("DEBUG: Has package.json: %v\n", strings.Contains(content, "package.json"))
		fmt.Printf("DEBUG: Has detectNodeComponent: %v\n", strings.Contains(content, "detectNodeComponent"))
		fmt.Printf("DEBUG: Current rule.Detect: %v\n", rule.Detect)
	}

	if strings.Contains(content, "components.json") && strings.Contains(content, "schema.json") {
		rule.Detect = &DetectConfig{
			Type:   "json-schema",
			File:   "components.json",
			Schema: "https://ui.shadcn.com/schema.json",
		}
	} else if strings.Contains(content, "package.json") && (strings.Contains(content, "parsePackageJSON") || strings.Contains(content, "detectNodeComponent") || strings.Contains(content, "detect: detectNodeComponent")) {
		fmt.Printf("DEBUG: NodeJS condition matched! Setting Detect field\n")
		rule.Detect = &DetectConfig{
			Type:    "package-json",
			File:    "package.json",
			Extract: true,
		}
		fmt.Printf("DEBUG: Set rule.Detect to: %v\n", rule.Detect)
	} else if strings.Contains(content, "tech: 'docker'") && strings.Contains(content, "detectDockerComponent") {
		rule.Detect = &DetectConfig{
			Type:    "docker",
			File:    "Dockerfile",
			Pattern: "Dockerfile|docker-compose.yml|docker-compose.yaml|.dockerignore",
		}
	} else if (strings.Contains(content, "tech: 'docker'") || strings.Contains(content, `tech: "docker"`)) && strings.Contains(content, "detectDockerComponent") {
		rule.Detect = &DetectConfig{
			Type:    "docker",
			File:    "Dockerfile",
			Pattern: "Dockerfile|docker-compose.yml|docker-compose.yaml|.dockerignore",
		}
	} else if (strings.Contains(content, "tech: 'github.actions'") || strings.Contains(content, `tech: "github.actions"`)) && strings.Contains(content, "detectGithubActionsComponent") {
		rule.Detect = &DetectConfig{
			Type:    "github-actions",
			File:    ".github/workflows",
			Pattern: "\\.github\\/workflows\\/.+\\.y(a)?ml$",
		}
		// Also add to files for simple directory detection
		rule.Files = append(rule.Files, ".github/workflows")
	} else if strings.Contains(content, "docker-compose") {
		rule.Detect = &DetectConfig{
			Type: "docker-compose",
			File: "docker-compose.yml",
		}
	} else if strings.Contains(content, ".tf") || strings.Contains(content, "terraform") {
		rule.Detect = &DetectConfig{
			Type: "terraform",
			File: "*.tf",
		}
	}
}

// isRegexPattern checks if a string is a regex pattern
func (c *Converter) isRegexPattern(name string) bool {
	regexIndicators := []string{"*", "+", "?", "[", "]", "(", ")", "|", "^", "$"}
	for _, indicator := range regexIndicators {
		if strings.Contains(name, indicator) {
			return true
		}
	}
	return false
}

// generateExample generates an example for a dependency
func (c *Converter) generateExample(name, depType string) string {
	if !c.isRegexPattern(name) {
		return name
	}

	examples := map[string]map[string]string{
		"npm": {
			"@anthropic*": "@anthropic-ai/sdk",
			"@aws-sdk*":   "@aws-sdk/client-s3",
			"@next*":      "@next/font",
			"@radix-ui*":  "@radix-ui/react-dialog",
			"*":           "example-package",
		},
		"python": {
			"django*": "django==4.0.0",
			"flask*":  "flask==2.0.0",
			"*":       "example-package==1.0.0",
		},
		"golang": {
			"github.com/*": "github.com/gin-gonic/gin",
			"*":            "github.com/example/lib",
		},
	}

	if typeExamples, exists := examples[depType]; exists {
		for pattern, example := range typeExamples {
			if pattern == "*" || strings.Contains(name, strings.TrimSuffix(pattern, "*")) {
				return example
			}
		}
	}

	return "example"
}

// getCategoryForType maps rule type to category directory
func (c *Converter) getCategoryForType(ruleType string) string {
	if category, exists := c.typeMapping[ruleType]; exists {
		return category
	}
	return "misc"
}

// writeRuleToFile writes a rule to a YAML file
func (c *Converter) writeRuleToFile(rule *Rule, sourceFile string) error {
	category := c.getCategoryForType(rule.Type)
	targetFile := filepath.Join(c.targetDir, "core", category, rule.Tech+".yaml")

	file, err := os.Create(targetFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	// Write YAML content
	fmt.Fprintf(writer, "tech: %s\n", rule.Tech)
	fmt.Fprintf(writer, "name: %s\n", rule.Name)
	fmt.Fprintf(writer, "type: %s\n", rule.Type)

	if len(rule.DotEnv) > 0 {
		c.writeStringList(writer, "dotenv", rule.DotEnv)
	}

	if len(rule.Dependencies) > 0 {
		c.writeDependencies(writer, rule.Dependencies)
	}

	if len(rule.Files) > 0 {
		c.writeQuotedList(writer, "files", rule.Files)
		if strings.Contains(rule.Tech, "nodejs") {
			fmt.Printf("DEBUG: Finished writing files section for NodeJS\n")
		}
	}

	if len(rule.Extensions) > 0 {
		c.writeQuotedList(writer, "extensions", rule.Extensions)
	}

	// Debug: Check if we reach the detect condition
	if strings.Contains(rule.Tech, "nodejs") {
		fmt.Printf("DEBUG: About to check rule.Detect != nil. rule.Detect = %v\n", rule.Detect)
	}

	if rule.Detect != nil {
		c.writeDetect(writer, rule)
	}

	// Debug: Show complete content for NodeJS
	if strings.Contains(rule.Tech, "nodejs") {
		fmt.Printf("DEBUG: About to write NodeJS file. Content so far:\n")
		// Unfortunately we can't easily read from writer, so let's add a temp debug
		fmt.Printf("DEBUG: rule.Detect = %v\n", rule.Detect)
	}

	err = writer.Flush()
	if err != nil {
		return err
	}

	// Debug: Verify file content for NodeJS
	if strings.Contains(rule.Tech, "nodejs") {
		content, err := os.ReadFile(targetFile)
		if err == nil {
			fmt.Printf("DEBUG: File content after write:\n%s\n=== END FILE ===\n", string(content))
		}
	}

	return nil
}

// needsQuoting checks if a string needs to be quoted in YAML
func (c *Converter) needsQuoting(value string) bool {
	if value == "" {
		return false
	}

	// Check for characters that need quoting
	specialChars := []string{"@", "#", "$", "%", "^", "&", "*", "(", ")", "[", "]", "{", "}", "|", "\\", ":", ";", "\"", "'", "<", ">", "?", "!", "~", "`"}

	for _, char := range specialChars {
		if strings.Contains(value, char) {
			return true
		}
	}

	// Check if it starts with a number or has special patterns
	if strings.HasPrefix(value, "-") || strings.HasPrefix(value, "?") || strings.HasPrefix(value, ":") {
		return true
	}

	return false
}

// writeStringList writes a YAML list of strings
func (c *Converter) writeStringList(w *bufio.Writer, key string, items []string) {
	fmt.Fprintf(w, "%s:\n", key)
	for _, item := range items {
		fmt.Fprintf(w, "  - %s\n", item)
	}
}

// writeDependencies writes dependencies as array format
func (c *Converter) writeDependencies(w *bufio.Writer, deps []Dependency) {
	fmt.Fprintf(w, "dependencies:\n")
	for _, dep := range deps {
		if dep[2] != "" {
			fmt.Fprintf(w, "  - [\"%s\", \"%s\", \"%s\"]\n", dep[0], dep[1], dep[2])
		} else {
			fmt.Fprintf(w, "  - [\"%s\", \"%s\", \"\"]\n", dep[0], dep[1])
		}
	}
}

// writeQuotedList writes a YAML list with conditional quoting
func (c *Converter) writeQuotedList(w *bufio.Writer, key string, items []string) {
	fmt.Fprintf(w, "%s:\n", key)
	for _, item := range items {
		if c.needsQuoting(item) {
			fmt.Fprintf(w, "  - \"%s\"\n", item)
		} else {
			fmt.Fprintf(w, "  - %s\n", item)
		}
	}
}

// writeDetect writes the detect section
func (c *Converter) writeDetect(w *bufio.Writer, rule *Rule) {
	fmt.Fprintf(w, "detect:\n")
	fmt.Fprintf(w, "  type: %s\n", rule.Detect.Type)

	if strings.Contains(rule.Tech, "nodejs") {
		fmt.Printf("DEBUG: Writing detect field for NodeJS: type=%s, file=%s\n", rule.Detect.Type, rule.Detect.File)
	}

	if rule.Detect.File != "" {
		if c.needsQuoting(rule.Detect.File) {
			fmt.Fprintf(w, "  file: \"%s\"\n", rule.Detect.File)
		} else {
			fmt.Fprintf(w, "  file: %s\n", rule.Detect.File)
		}
	}
	if rule.Detect.Schema != "" {
		fmt.Fprintf(w, "  schema: %s\n", rule.Detect.Schema)
	}
	if rule.Detect.Pattern != "" {
		fmt.Fprintf(w, "  pattern: %s\n", rule.Detect.Pattern)
	}
	if rule.Detect.Extract {
		fmt.Fprintf(w, "  extract: true\n")
	}
}

// PrintStats prints conversion statistics
func (c *Converter) PrintStats() {
	if len(c.stats) == 0 {
		return
	}

	log.Printf("Statistics by category:")
	var categories []string
	for category := range c.stats {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	total := 0
	for _, category := range categories {
		count := c.stats[category]
		log.Printf("  %s: %d rules", category, count)
		total += count
	}
	log.Printf("  Total: %d rules", total)
}

func main() {
	var (
		sourceDir = flag.String("source", "../../stack-analyser/src/rules", "Source directory containing TypeScript rules")
		targetDir = flag.String("target", "../../internal/rules", "Target directory for YAML rules (will create core/ subdirectory)")
		limit     = flag.Int("limit", 0, "Limit number of rules to convert (for testing)")
		dryRun    = flag.Bool("dry-run", false, "Show what would be converted without writing files")
		stats     = flag.Bool("stats", false, "Show conversion statistics")
	)
	flag.Parse()

	log.Printf("Stack Analyser Rule Converter (Go)")
	log.Printf("Source: %s", *sourceDir)
	log.Printf("Target: %s", *targetDir)
	if *limit > 0 {
		log.Printf("Limit: %d rules", *limit)
	}
	if *dryRun {
		log.Printf("DRY RUN MODE - No files will be written")
	}

	// Create converter
	converter := NewConverter(*sourceDir, *targetDir)

	// Convert rules
	err := converter.ConvertAll(*limit, *dryRun)
	if err != nil {
		log.Fatalf("Conversion failed: %v", err)
	}

	// Print statistics if requested
	if *stats {
		converter.PrintStats()
	}

	log.Printf("Next steps:")
	log.Printf("1. Check the converted rules in internal/rules/core/")
	log.Printf("2. Test the Go scanner: go test ./... -v")
	log.Printf("3. Build the scanner: go build -o bin/scanner ./cmd/scanner")
	log.Printf("4. Run a scan: ./bin/scanner <path>")
}
