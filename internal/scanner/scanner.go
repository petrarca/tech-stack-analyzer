package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/petrarca/tech-stack-analyzer/internal/config"
	"github.com/petrarca/tech-stack-analyzer/internal/git"
	"github.com/petrarca/tech-stack-analyzer/internal/metadata"
	"github.com/petrarca/tech-stack-analyzer/internal/progress"
	"github.com/petrarca/tech-stack-analyzer/internal/provider"
	"github.com/petrarca/tech-stack-analyzer/internal/rules"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/matchers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/spec"
	"github.com/sirupsen/logrus"

	// Import component detectors to trigger init() registration
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/cocoapods"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/delphi"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/deno"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/docker"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/dotnet"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/golang"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/java"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/nodejs"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/php"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/python"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/ruby"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/rust"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/terraform"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Scanner handles the scanning logic (like TypeScript's Payload.recurse)
type Scanner struct {
	provider        types.Provider
	rules           []types.Rule
	depDetector     *DependencyDetector
	compDetector    *ComponentDetector
	dotenvDetector  *parsers.DotenvDetector
	licenseDetector *LicenseDetector
	langDetector    *LanguageDetector
	contentMatcher  *matchers.ContentMatcherRegistry
	excludeDirs     []string
	progress        *progress.Progress
	codeStats       CodeStatsAnalyzer
}

// CodeStatsAnalyzer interface for code statistics collection
type CodeStatsAnalyzer interface {
	// ProcessFile analyzes a file
	// language is the go-enry detected language name (used for grouping)
	// If content is provided it will be used, otherwise file is read
	ProcessFile(filename string, language string, content []byte)
	IsEnabled() bool
	GetStats() interface{} // Returns the aggregated stats
}

// defaultIgnorePatterns holds the loaded ignore patterns from ignore.yaml
var defaultIgnorePatterns []string

// NewScanner creates a new scanner (mirroring TypeScript's analyser function)
func NewScanner(path string) (*Scanner, error) {
	return NewScannerWithOptions(path, nil, false, false, false, false, nil)
}

// NewScannerWithExcludes creates a new scanner with directory exclusions
func NewScannerWithExcludes(path string, excludeDirs []string, verbose bool, useTreeView bool, traceTimings bool, traceRules bool) (*Scanner, error) {
	return NewScannerWithOptions(path, excludeDirs, verbose, useTreeView, traceTimings, traceRules, nil)
}

// NewScannerWithOptions creates a new scanner with all options including code stats
func NewScannerWithOptions(path string, excludeDirs []string, verbose bool, useTreeView bool, traceTimings bool, traceRules bool, codeStats CodeStatsAnalyzer) (*Scanner, error) {
	return NewScannerWithOptionsAndLogger(path, excludeDirs, verbose, useTreeView, traceTimings, traceRules, codeStats, nil)
}

// NewScannerWithOptionsAndLogger creates a new scanner with all options including logger
func NewScannerWithOptionsAndLogger(path string, excludeDirs []string, verbose bool, useTreeView bool, traceTimings bool, traceRules bool, codeStats CodeStatsAnalyzer, logger *logrus.Logger) (*Scanner, error) {
	// Create provider for the target path (like TypeScript's FSProvider)
	provider := provider.NewFSProvider(path)

	// Initialize all scanner components
	components, err := initializeScannerComponents(provider, path)
	if err != nil {
		return nil, err
	}

	// Create progress reporter
	var prog *progress.Progress
	if verbose || useTreeView {
		if useTreeView {
			prog = progress.New(true, progress.NewTreeHandler(os.Stderr))
		} else {
			prog = progress.New(true, progress.NewSimpleHandler(os.Stderr))
		}
	} else {
		prog = progress.New(false, progress.NewNullHandler())
	}

	// Load ignore patterns from .gitignore (only once) with progress reporting
	if len(defaultIgnorePatterns) == 0 {
		gitignoreLoader := git.NewGitignoreLoaderWithLogger(prog, logger)
		ignorePatterns, err := gitignoreLoader.LoadPatterns(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load ignore patterns: %w", err)
		}
		defaultIgnorePatterns = ignorePatterns
	}

	// Enable tracing if requested
	if traceTimings {
		prog.EnableTimings()
	}
	if traceRules {
		prog.EnableRuleTracing()
	}

	return &Scanner{
		provider:        provider,
		rules:           components.rules,
		depDetector:     components.depDetector,
		compDetector:    components.compDetector,
		dotenvDetector:  components.dotenvDetector,
		licenseDetector: components.licenseDetector,
		langDetector:    NewLanguageDetector(),
		contentMatcher:  components.contentMatcher,
		excludeDirs:     excludeDirs,
		progress:        prog,
		codeStats:       codeStats,
	}, nil
}

// scannerComponents holds all initialized scanner components
type scannerComponents struct {
	rules           []types.Rule
	depDetector     *DependencyDetector
	compDetector    *ComponentDetector
	dotenvDetector  *parsers.DotenvDetector
	licenseDetector *LicenseDetector
	contentMatcher  *matchers.ContentMatcherRegistry
}

// initializeScannerComponents handles common initialization logic
func initializeScannerComponents(provider types.Provider, path string) (*scannerComponents, error) {
	// Load rules (simple, not lazy loaded - like TypeScript's loadAllRules)
	loadedRules, err := rules.LoadEmbeddedRules()
	if err != nil {
		return nil, fmt.Errorf("failed to load rules: %w", err)
	}

	// Load types configuration
	categoriesConfig, err := config.LoadCategoriesConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load categories config: %w", err)
	}
	SetCategoriesConfig(categoriesConfig)

	// Initialize all detectors
	depDetector := NewDependencyDetector(loadedRules)
	compDetector := NewComponentDetector(depDetector, provider, loadedRules)
	dotenvDetector := parsers.NewDotenvDetector(provider, loadedRules)
	licenseDetector := NewLicenseDetector()

	// Build matchers from rules (like TypeScript's loadAllRules)
	matchers.BuildFileMatchersFromRules(loadedRules)

	// Build content matchers from rules
	contentMatcher := matchers.NewContentMatcherRegistry()
	if err := contentMatcher.BuildFromRules(loadedRules); err != nil {
		return nil, fmt.Errorf("failed to build content matchers: %w", err)
	}

	return &scannerComponents{
		rules:           loadedRules,
		depDetector:     depDetector,
		compDetector:    compDetector,
		dotenvDetector:  dotenvDetector,
		licenseDetector: licenseDetector,
		contentMatcher:  contentMatcher,
	}, nil
}

// Scan performs analysis following the original TypeScript pattern
func (s *Scanner) Scan() (*types.Payload, error) {
	basePath := s.provider.GetBasePath()

	// Report scan start
	s.progress.ScanStart(basePath, s.excludeDirs)

	// Load configuration from .stack-analyzer.yml if it exists
	cfg, err := config.LoadConfig(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create scan metadata
	scanMeta := metadata.NewScanMetadata(basePath, spec.Version, s.excludeDirs)
	startTime := time.Now()

	// Create main payload like in TypeScript: new Payload({ name: 'main', folderPath: '/' })
	payload := types.NewPayloadWithPath("main", "/")

	// Add configured techs to payload
	for _, tech := range cfg.Techs {
		payload.AddTech(tech.Tech, tech.Reason)
	}

	// Start recursion from base path (like TypeScript's payload.recurse(provider, provider.basePath))
	err = s.recurse(payload, basePath)
	if err != nil {
		return nil, err
	}

	// Set scan duration
	scanMeta.SetDuration(time.Since(startTime))

	// Count files and components in the payload tree
	fileCount, componentCount := s.countFilesAndComponents(payload)
	scanMeta.SetFileCounts(fileCount, componentCount)

	// Count languages, primary techs, and all techs
	languageCount := s.countLanguages(payload)
	techCount, techsCount := s.countTechs(payload)
	scanMeta.SetLanguageCount(languageCount)
	scanMeta.SetTechCounts(techCount, techsCount)

	// Set custom properties from config
	scanMeta.SetProperties(cfg.Properties)

	// Set git information directly on payload
	payload.Git = git.GetGitInfo(basePath)

	// Attach metadata to root payload
	payload.Metadata = scanMeta

	// Report scan complete
	s.progress.ScanComplete(fileCount, componentCount, time.Since(startTime))

	return payload, nil
}

// countFilesAndComponents recursively counts files and components in the payload tree
func (s *Scanner) countFilesAndComponents(payload *types.Payload) (int, int) {
	fileCount := 0

	// Sum actual file counts from languages map
	for _, count := range payload.Languages {
		fileCount += count
	}

	componentCount := 1 // Current component (payload node)

	for _, child := range payload.Childs {
		childFiles, childComponents := s.countFilesAndComponents(child)
		fileCount += childFiles
		componentCount += childComponents
	}

	return fileCount, componentCount
}

// countLanguages recursively counts distinct programming languages in the payload tree
func (s *Scanner) countLanguages(payload *types.Payload) int {
	languages := make(map[string]bool)

	// Collect languages from current payload
	for lang := range payload.Languages {
		languages[lang] = true
	}

	// Recursively collect from child components
	for _, child := range payload.Childs {
		// We need to get the actual language names from the child
		s.collectLanguages(child, languages)
	}

	return len(languages)
}

// collectLanguages helper to recursively collect language names
func (s *Scanner) collectLanguages(payload *types.Payload, languages map[string]bool) {
	for lang := range payload.Languages {
		languages[lang] = true
	}

	for _, child := range payload.Childs {
		s.collectLanguages(child, languages)
	}
}

// countTechs returns the count of primary techs and all detected techs
func (s *Scanner) countTechs(payload *types.Payload) (int, int) {
	primaryTechs := make(map[string]bool)
	allTechs := make(map[string]bool)

	// Collect from current payload
	for _, tech := range payload.Tech {
		primaryTechs[tech] = true
		allTechs[tech] = true
	}

	for _, tech := range payload.Techs {
		allTechs[tech] = true
	}

	// Recursively collect from child components
	for _, child := range payload.Childs {
		s.collectTechs(child, primaryTechs, allTechs)
	}

	return len(primaryTechs), len(allTechs)
}

// collectTechs helper to recursively collect tech names
func (s *Scanner) collectTechs(payload *types.Payload, primaryTechs, allTechs map[string]bool) {
	for _, tech := range payload.Tech {
		primaryTechs[tech] = true
		allTechs[tech] = true
	}

	for _, tech := range payload.Techs {
		allTechs[tech] = true
	}

	for _, child := range payload.Childs {
		s.collectTechs(child, primaryTechs, allTechs)
	}
}

// ScanFile performs analysis on a single file, treating it as a directory with just that file
func (s *Scanner) ScanFile(fileName string) (*types.Payload, error) {
	// Create main payload
	payload := types.NewPayloadWithPath("main", "/")

	// The provider's base path is already set to the directory containing the file
	// We just need to pass the file name
	basePath := s.provider.GetBasePath()

	// Create a virtual file list with just the single file
	files := []types.File{
		{
			Name:     fileName,
			Path:     fileName,
			Type:     "file",
			Size:     0, // Size not needed for detection
			Modified: 0, // Modified time not needed for detection
		},
	}

	// Apply rules to detect technologies on the single file
	// Pass the base path (directory) as the current path for component detection
	ctx := s.applyRules(payload, files, basePath)

	// Detect language from the file name with content analysis
	filePath := filepath.Join(basePath, fileName)
	content, err := s.provider.ReadFile(filePath)
	if err != nil {
		content = []byte{} // Empty content on error
	}
	if lang := s.langDetector.DetectLanguage(fileName, content); lang != "" {
		ctx.AddLanguage(lang)
	}

	// Collect code statistics if enabled (pass go-enry language for grouping)
	if s.codeStats != nil {
		s.codeStats.ProcessFile(filePath, s.langDetector.DetectLanguage(fileName, content), content)
	}

	// Add metadata for single file scan
	scanMeta := metadata.NewScanMetadata(basePath, spec.Version, s.excludeDirs)
	fileCount, componentCount := s.countFilesAndComponents(payload)
	scanMeta.SetFileCounts(fileCount, componentCount)
	languageCount := s.countLanguages(payload)
	techCount, techsCount := s.countTechs(payload)
	scanMeta.SetLanguageCount(languageCount)
	scanMeta.SetTechCounts(techCount, techsCount)

	// Set git information directly on payload
	payload.Git = git.GetGitInfo(basePath)

	payload.Metadata = scanMeta

	return payload, nil
}

// recurse follows the exact TypeScript implementation pattern
func (s *Scanner) recurse(payload *types.Payload, filePath string) error {
	// Report entering directory
	s.progress.EnterDirectory(filePath)
	defer s.progress.LeaveDirectory(filePath)

	// Get files in current directory (like TypeScript's provider.listDir)
	files, err := s.provider.ListDir(filePath)
	if err != nil {
		return err
	}

	// Apply rules to detect technologies (like TypeScript's ruleComponents loop)
	// This might return a different context if a component was detected
	ctx := s.applyRules(payload, files, filePath)

	// Check if this directory is a git repository and set git info
	// Only set git info if it's not already set (avoid overwriting parent repo info)
	if ctx.Git == nil {
		if gitInfo := git.GetGitInfo(filePath); gitInfo != nil {
			ctx.Git = gitInfo
		}
	}

	// Detect licenses from LICENSE files in this directory
	// This adds file-based license detection (MIT, Apache-2.0, etc.) from LICENSE files
	s.licenseDetector.AddLicensesToPayload(ctx, filePath)

	// Process each file/directory (exactly like TypeScript's loop)
	for _, file := range files {
		if file.Type == "file" {
			// Check if file should be excluded
			if s.shouldExcludeFile(file.Name, filePath) {
				continue
			}

			// Detect language from file name (like TypeScript's detectLang)
			// Languages go into the current context (might be a component)
			// Read content for accurate detection of ambiguous extensions
			fileFullPath := filepath.Join(filePath, file.Name)
			content, err := s.provider.ReadFile(fileFullPath)
			if err != nil {
				content = []byte{} // Empty content on error
			}
			lang := s.langDetector.DetectLanguage(file.Name, content)
			if lang != "" {
				ctx.AddLanguage(lang)
			}

			// Collect code statistics if enabled (pass go-enry language for grouping)
			if s.codeStats != nil {
				s.codeStats.ProcessFile(fileFullPath, lang, content)
			}
			continue
		}

		// Skip ignored directories (like TypeScript's IGNORED_DIVE_PATHS)
		if s.shouldIgnoreDirectory(file.Name) {
			s.progress.Skipped(filepath.Join(filePath, file.Name), "excluded")
			continue
		}

		// Recurse into subdirectory (like TypeScript's await ctx.recurse(provider, fp))
		// Important: We recurse with the CURRENT CONTEXT (ctx), not the original payload
		// This matches TypeScript's behavior where ctx might be a component
		childPath := filepath.Join(filePath, file.Name)
		err := s.recurse(ctx, childPath)
		if err != nil {
			return err
		}
	}

	// Note: Do NOT combine ctx back to payload
	// Components should remain separate with their own dependencies
	// Extension reasons are handled separately by the AddTech fix

	return nil
}

// applyRules applies rules to detect technologies (following TypeScript's pattern exactly)
func (s *Scanner) applyRules(payload *types.Payload, files []types.File, currentPath string) *types.Payload {
	ctx := payload

	// 1. Component-based detection
	ctx = s.detectComponents(payload, ctx, files, currentPath)

	// 2. GitHub Actions detection
	s.detectGitHubActions(payload, files, currentPath)

	// 3. Dotenv detection
	s.detectDotenv(ctx, files, currentPath)

	// 4. File and extension-based detection (includes JSON schema via content matchers)
	matchedTechs := s.detectByFilesAndExtensions(ctx, files, currentPath)

	// 6. Legacy file-based detection
	s.detectLegacyFiles(ctx, files, matchedTechs)

	return ctx
}

func (s *Scanner) detectComponents(payload, ctx *types.Payload, files []types.File, currentPath string) *types.Payload {
	var namedComponents []*types.Payload
	var virtualComponents []*types.Payload

	// Collect all components from all detectors
	for _, detector := range components.GetDetectors() {
		detectedComponents := detector.Detect(files, currentPath, s.provider.GetBasePath(), s.provider, s.depDetector)
		for _, component := range detectedComponents {
			// Add git information to newly created components
			// Only set if not already present (allows detectors to override if needed)
			if component.Git == nil {
				if gitInfo := git.GetGitInfo(currentPath); gitInfo != nil {
					component.Git = gitInfo
				}
			}

			if component.Name == "virtual" {
				virtualComponents = append(virtualComponents, component)
			} else {
				namedComponents = append(namedComponents, component)
			}
		}
	}

	// Merge virtual components first
	for _, virtual := range virtualComponents {
		s.mergeVirtualPayload(payload, virtual, currentPath)
	}

	// Handle named components
	if len(namedComponents) == 0 {
		return ctx
	} else if len(namedComponents) == 1 {
		// Single component - add it normally
		ctx = s.addNamedComponent(payload, namedComponents[0], currentPath)
	} else {
		// Multiple components in same directory - merge them
		merged := s.mergeComponents(namedComponents)
		ctx = s.addNamedComponent(payload, merged, currentPath)
	}

	return ctx
}

func (s *Scanner) mergeComponents(components []*types.Payload) *types.Payload {
	if len(components) == 0 {
		return nil
	}

	// Use first component as base
	base := components[0]

	// Merge all other components into the base
	for i := 1; i < len(components); i++ {
		comp := components[i]

		// Merge primary techs
		for _, tech := range comp.Tech {
			base.AddPrimaryTech(tech)
		}

		// Merge all techs
		for _, tech := range comp.Techs {
			base.AddTech(tech, "merged from multiple detectors")
		}

		// Merge dependencies
		for _, dep := range comp.Dependencies {
			base.AddDependency(dep)
		}

		// Merge paths
		for _, path := range comp.Path {
			base.AddPath(path)
		}

		// Merge licenses
		for _, license := range comp.Licenses {
			base.AddLicense(license)
		}

		// Copy component reasons to base payload for visibility
		if len(comp.Reason) > 0 {
			// Only copy the "_" (non-tech) reasons, not tech-specific ones
			if nonTechReasons, exists := comp.Reason["_"]; exists {
				for _, reason := range nonTechReasons {
					base.AddReason(reason)
				}
			}
		}
	}

	return base
}

func (s *Scanner) mergeVirtualPayload(target, virtual *types.Payload, currentPath string) {
	for _, child := range virtual.Childs {
		target.AddChild(child)
	}
	target.Combine(virtual)
	for _, tech := range virtual.Techs {
		s.findImplicitComponentByTech(target, tech, currentPath, false)
	}
}

func (s *Scanner) addNamedComponent(payload, component *types.Payload, currentPath string) *types.Payload {
	payload.AddChild(component)

	// Report component detection
	if len(component.Tech) > 0 {
		s.progress.ComponentDetected(component.Name, component.Tech[0], currentPath)
	}

	for _, tech := range component.Techs {
		s.findImplicitComponentByTech(component, tech, currentPath, true)
	}
	return component
}

func (s *Scanner) detectGitHubActions(payload *types.Payload, files []types.File, currentPath string) {
	githubActionsComponents := s.compDetector.DetectGitHubActionsComponent(files, currentPath, s.provider.GetBasePath())
	s.processDetectedComponent(payload, githubActionsComponents, currentPath)
}

func (s *Scanner) detectDotenv(ctx *types.Payload, files []types.File, currentPath string) {
	dotenvPayload := s.dotenvDetector.DetectInDotEnv(files, currentPath, s.provider.GetBasePath())
	s.processDetectedComponent(ctx, dotenvPayload, currentPath)
}

// processDetectedComponent handles the common pattern of processing detected components
func (s *Scanner) processDetectedComponent(target *types.Payload, component *types.Payload, currentPath string) {
	if component == nil {
		return
	}

	if component.Name == "virtual" {
		s.mergeVirtualPayload(target, component, currentPath)
	} else {
		target.AddChild(component)
	}
}

func (s *Scanner) detectByFilesAndExtensions(ctx *types.Payload, files []types.File, currentPath string) map[string]bool {
	matchedTechs := make(map[string]bool)

	// File-based detection
	fileMatches := matchers.MatchFiles(files, currentPath, s.provider.GetBasePath())
	s.processTechMatches(ctx, fileMatches, matchedTechs, currentPath, true)

	// Extension-based detection (only for rules without content requirements)
	extensionMatches := matchers.MatchExtensions(files)
	s.processTechMatches(ctx, extensionMatches, matchedTechs, currentPath, false)

	// Content-based detection (for rules WITH content requirements)
	// These rules require BOTH extension AND content to match
	s.detectByContent(ctx, files, currentPath, matchedTechs)

	return matchedTechs
}

func (s *Scanner) detectByContent(ctx *types.Payload, files []types.File, currentPath string, matchedTechs map[string]bool) {
	// Content-based detection: for extensions or filenames that have content matchers,
	// check if the file content matches the patterns
	// This is ADDITIVE - it can detect techs that weren't matched by extension alone

	for _, file := range files {
		if file.Type != "file" {
			continue
		}

		if !s.shouldCheckFileContent(file) {
			continue
		}

		filePath := filepath.Join(currentPath, file.Name)
		content, err := s.provider.ReadFile(filePath)
		if err != nil {
			continue
		}

		contentMatches := s.matchFileContent(file, string(content))
		s.processContentMatches(ctx, contentMatches, matchedTechs, filePath, currentPath)
	}
}

func (s *Scanner) shouldCheckFileContent(file types.File) bool {
	hasFileMatchers := s.contentMatcher.HasFileMatchers(file.Name)
	ext := filepath.Ext(file.Name)
	hasExtMatchers := ext != "" && s.contentMatcher.HasContentMatchers(ext)
	return hasFileMatchers || hasExtMatchers
}

func (s *Scanner) matchFileContent(file types.File, content string) map[string][]string {
	hasFileMatchers := s.contentMatcher.HasFileMatchers(file.Name)
	ext := filepath.Ext(file.Name)
	hasExtMatchers := ext != "" && s.contentMatcher.HasContentMatchers(ext)

	var contentMatches map[string][]string
	if hasFileMatchers {
		contentMatches = s.contentMatcher.MatchFileContent(file.Name, content)
	}

	if hasExtMatchers {
		extMatches := s.contentMatcher.MatchContent(ext, content)
		if contentMatches == nil {
			contentMatches = extMatches
		} else {
			for tech, reasons := range extMatches {
				contentMatches[tech] = append(contentMatches[tech], reasons...)
			}
		}
	}

	return contentMatches
}

func (s *Scanner) processContentMatches(ctx *types.Payload, contentMatches map[string][]string, matchedTechs map[string]bool, filePath, currentPath string) {
	for tech, reasons := range contentMatches {
		if !matchedTechs[tech] && len(reasons) > 0 {
			relPath, _ := filepath.Rel(s.provider.GetBasePath(), filePath)
			s.progress.RuleResultWithPath(tech, true, reasons[0], relPath)
		}

		for _, reason := range reasons {
			ctx.AddTech(tech, reason)
		}

		if !matchedTechs[tech] {
			matchedTechs[tech] = true
			s.findImplicitComponentByTech(ctx, tech, currentPath, false)
		}
	}
}

func (s *Scanner) processTechMatches(ctx *types.Payload, matches map[string][]string, matchedTechs map[string]bool, currentPath string, addEdges bool) {
	for tech, reasons := range matches {
		if matchedTechs[tech] {
			continue
		}
		// Report rule match for tracing
		if len(reasons) > 0 {
			relPath, _ := filepath.Rel(s.provider.GetBasePath(), currentPath)
			if relPath == "" {
				relPath = "."
			}
			s.progress.RuleResultWithPath(tech, true, reasons[0], relPath)
		}

		for _, reason := range reasons {
			s.addTechWithPrimaryCheck(ctx, tech, reason, currentPath)
		}
		matchedTechs[tech] = true
		s.findImplicitComponentByTech(ctx, tech, currentPath, addEdges)
	}
}

func (s *Scanner) detectLegacyFiles(ctx *types.Payload, files []types.File, matchedTechs map[string]bool) {
	for _, rule := range s.rules {
		if len(rule.Files) == 0 || matchedTechs[rule.Tech] {
			continue
		}
		if s.matchRuleFiles(rule, files) {
			reason := fmt.Sprintf("matched file: %s", rule.Files[0])
			// Report rule match for tracing
			s.progress.RuleResult(rule.Tech, true, reason)
			s.addTechWithPrimaryCheck(ctx, rule.Tech, reason, "")
			matchedTechs[rule.Tech] = true
		}
	}
}

func (s *Scanner) matchRuleFiles(rule types.Rule, files []types.File) bool {
	for _, requiredFile := range rule.Files {
		for _, file := range files {
			if file.Name == requiredFile {
				return true
			}
		}
	}
	return false
}

// findImplicitComponentByTech finds the rule for a tech and creates an implicit component
func (s *Scanner) findImplicitComponentByTech(payload *types.Payload, tech string, currentPath string, addEdges bool) {
	// Find the rule for this tech
	for _, rule := range s.rules {
		if rule.Tech == tech {
			s.findImplicitComponent(payload, rule, currentPath, addEdges)
			return
		}
	}
}

// findImplicitComponent creates a child component for technologies that are not in the notAComponent set
// This replicates the TypeScript findImplicitComponent logic
func (s *Scanner) findImplicitComponent(payload *types.Payload, rule types.Rule, currentPath string, addEdges bool) {
	// Check if this rule should create a component
	// Uses is_component field if set, otherwise uses type-based logic
	if !ShouldCreateComponent(rule) {
		return
	}

	// Create a new child component (like TypeScript lines 47-54)
	// CRITICAL FIX: Use parent's path, not currentPath (like TypeScript: folderPath: pl.path)
	component := types.NewPayload(rule.Name, payload.Path)

	// NEW: Check is_primary_tech field to determine if we should add primary tech
	if ShouldAddPrimaryTech(rule) {
		component.AddPrimaryTech(rule.Tech)
	} else {
		component.AddTech(rule.Tech, fmt.Sprintf("matched file: %s", currentPath))
	}

	component.AddReason(fmt.Sprintf("matched file: %s", currentPath))

	// Add the component as a child
	payload.AddChild(component)

	// Add edges for non-hosting/cloud types if requested (like TypeScript: if (ref.type !== 'hosting' && ref.type !== 'cloud'))
	if addEdges && rule.Type != "hosting" && rule.Type != "cloud" {
		payload.AddEdges(component)
	}
}

// addTechWithPrimaryCheck adds technology and checks if it should be primary tech
func (s *Scanner) addTechWithPrimaryCheck(payload *types.Payload, tech string, reason string, currentPath string) {
	// Always add to techs array
	payload.AddTech(tech, reason)

	// Check if this tech should be primary tech even without component
	for _, rule := range s.rules {
		if rule.Tech == tech && ShouldAddPrimaryTech(rule) && !ShouldCreateComponent(rule) {
			// This rule wants to be primary tech but doesn't create components
			// Add to root payload's primary tech array directly
			payload.AddPrimaryTech(tech)
			break
		}
	}
}
func (s *Scanner) shouldExcludeFile(fileName, currentPath string) bool {
	if len(s.excludeDirs) == 0 {
		return false
	}

	// Get relative path from base path
	basePath := s.provider.GetBasePath()
	fullPath := filepath.Join(currentPath, fileName)
	relPath, err := filepath.Rel(basePath, fullPath)
	if err != nil {
		relPath = fileName // Fallback to just filename
	}

	// Check against exclude patterns
	for _, pattern := range s.excludeDirs {
		// Try glob match against relative path
		matched, err := doublestar.Match(pattern, relPath)
		if err == nil && matched {
			return true
		}

		// Also try matching just the filename
		matched, err = doublestar.Match(pattern, fileName)
		if err == nil && matched {
			return true
		}
	}

	return false
}

// shouldIgnoreDirectory checks if a directory should be ignored during scanning
// Uses modular ignore patterns defined in ignore_patterns.go
func (s *Scanner) shouldIgnoreDirectory(name string) bool {
	// Check user-specified exclude patterns first (supports glob patterns)
	if len(s.excludeDirs) > 0 {
		for _, pattern := range s.excludeDirs {
			// Try glob match first
			matched, err := doublestar.Match(pattern, name)
			if err == nil && matched {
				return true
			}

			// Fallback to simple name match for backward compatibility
			if strings.EqualFold(name, pattern) {
				return true
			}
		}
	}

	// Get all ignore patterns from configuration
	for _, pattern := range defaultIgnorePatterns {
		// Try glob match first (same as CLI excludes)
		matched, err := doublestar.Match(pattern, name)
		if err == nil && matched {
			return true
		}

		// Fallback to simple name match for backward compatibility
		if strings.EqualFold(name, pattern) {
			return true
		}
	}

	return false
}
