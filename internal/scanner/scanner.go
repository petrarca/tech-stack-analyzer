package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log/slog"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/petrarca/tech-stack-analyzer/internal/config"
	"github.com/petrarca/tech-stack-analyzer/internal/git"
	"github.com/petrarca/tech-stack-analyzer/internal/license"
	"github.com/petrarca/tech-stack-analyzer/internal/metadata"
	"github.com/petrarca/tech-stack-analyzer/internal/progress"
	"github.com/petrarca/tech-stack-analyzer/internal/provider"
	"github.com/petrarca/tech-stack-analyzer/internal/rules"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/matchers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/spec"
	"github.com/petrarca/tech-stack-analyzer/internal/types"

	// Import component detectors to trigger init() registration
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/cocoapods"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/cplusplus"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/delphi"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/deno"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/docker"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/dotnet"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/githubactions"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/golang"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/java"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/nodejs"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/php"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/python"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/ruby"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/rust"
	_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/terraform"
)

// Scanner handles the scanning logic (like TypeScript's Payload.recurse)
type Scanner struct {
	provider        types.Provider
	rules           []types.Rule
	depDetector     *DependencyDetector
	dotenvDetector  *parsers.DotenvDetector
	licenseDetector *license.LicenseDetector
	langDetector    *LanguageDetector
	contentMatcher  *matchers.ContentMatcherRegistry
	excludePatterns []string
	progress        *progress.Progress
	codeStats       CodeStatsAnalyzer
	gitignoreStack  *git.StackBasedLoader
	gitCache        map[string]*git.GitInfo // Cache git info by repo root path
	gitRootCache    map[string]string       // Cache path -> repo root mapping
	rootID          string                  // Override root ID for deterministic scans
	config          *config.ScanConfig      // Merged configuration for metadata properties
	useLockFiles    bool                    // Use lock files for dependency resolution
}

// CodeStatsAnalyzer interface for code statistics collection
type CodeStatsAnalyzer interface {
	// ProcessFile analyzes a file
	// language is the go-enry detected language name (used for grouping)
	// If content is provided it will be used, otherwise file is read
	ProcessFile(filename string, language string, content []byte)

	// ProcessFileForComponent analyzes a file for a specific component
	// componentID is used to group stats by component
	// language is the go-enry detected language name (used for grouping)
	// If content is provided it will be used, otherwise the file will be read
	ProcessFileForComponent(filename string, language string, content []byte, componentID string)

	// GetStats returns the aggregated statistics
	GetStats() interface{}

	// GetComponentStats returns statistics for a specific component
	GetComponentStats(componentID string) interface{}

	// IsEnabled returns whether code stats collection is enabled
	IsEnabled() bool

	// IsPerComponentEnabled returns whether per-component code stats collection is enabled
	IsPerComponentEnabled() bool
}

// NewScanner creates a new scanner (mirroring TypeScript's analyser function)
func NewScanner(path string) (*Scanner, error) {
	return NewScannerWithOptions(path, nil, false, false, false, false, nil)
}

// NewScannerWithExcludes creates a new scanner with exclusion patterns
func NewScannerWithExcludes(path string, excludePatterns []string, verbose bool, useTreeView bool, traceTimings bool, traceRules bool) (*Scanner, error) {
	return NewScannerWithOptions(path, excludePatterns, verbose, useTreeView, traceTimings, traceRules, nil)
}

// NewScannerWithOptions creates a new scanner with all options including code stats
func NewScannerWithOptions(path string, excludePatterns []string, verbose bool, useTreeView bool, traceTimings bool, traceRules bool, codeStats CodeStatsAnalyzer) (*Scanner, error) {
	return NewScannerWithOptionsAndLogger(path, excludePatterns, verbose, useTreeView, traceTimings, traceRules, codeStats, nil, "", nil)
}

// NewScannerWithOptionsAndRootID creates a new scanner with root ID override
func NewScannerWithOptionsAndRootID(path string, excludePatterns []string, verbose bool, useTreeView bool, traceTimings bool, traceRules bool, codeStats CodeStatsAnalyzer, rootID string) (*Scanner, error) {
	return NewScannerWithOptionsAndLogger(path, excludePatterns, verbose, useTreeView, traceTimings, traceRules, codeStats, nil, rootID, nil)
}

// NewScannerWithOptionsAndLogger creates a new scanner with all options including logger
func NewScannerWithOptionsAndLogger(path string, excludePatterns []string, verbose bool, useTreeView bool, traceTimings bool, traceRules bool, codeStats CodeStatsAnalyzer, logger *slog.Logger, rootID string, mergedConfig *config.ScanConfig) (*Scanner, error) {
	// Create provider for the target path (like TypeScript's FSProvider)
	tInit := time.Now()
	provider := provider.NewFSProvider(path)

	// Initialize all scanner components
	components, err := initializeScannerComponents(provider, path, logger)
	if err != nil {
		return nil, err
	}
	if logger != nil {
		logger.Debug("Scanner initialization completed", "duration", time.Since(tInit))
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

	// Initialize stack-based gitignore loader
	gitignoreStack := git.NewStackBasedLoaderWithLogger(prog, logger)

	// Load config excludes to pass to gitignore stack
	var configExcludes []string
	var cfg *config.ScanConfig

	// Use merged config if provided, otherwise load project config
	if mergedConfig != nil {
		cfg = mergedConfig
	} else {
		cfg, err = config.LoadConfig(path)
		if err != nil {
			cfg = &config.ScanConfig{} // Use empty config if load fails
		}
	}

	if len(cfg.Exclude) > 0 {
		configExcludes = cfg.Exclude
	}

	// Initialize with top-level excludes (config and CLI patterns)
	if err := gitignoreStack.InitializeWithTopLevelExcludes(path, excludePatterns, configExcludes); err != nil {
		return nil, fmt.Errorf("failed to initialize top-level excludes: %w", err)
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
		dotenvDetector:  components.dotenvDetector,
		licenseDetector: components.licenseDetector,
		langDetector:    NewLanguageDetector(),
		contentMatcher:  components.contentMatcher,
		excludePatterns: excludePatterns,
		progress:        prog,
		codeStats:       codeStats,
		gitignoreStack:  gitignoreStack,
		gitCache:        make(map[string]*git.GitInfo),
		gitRootCache:    make(map[string]string),
		rootID:          rootID,
		config:          cfg,
		useLockFiles:    true, // Default to true
	}, nil
}

// NewScannerWithSettings creates a new scanner with full settings support
func NewScannerWithSettings(path string, settings *config.Settings, mergedConfig *config.ScanConfig, logger *slog.Logger) (*Scanner, error) {
	scanner, err := NewScannerWithOptionsAndLogger(
		path,
		settings.ExcludePatterns,
		settings.Verbose,
		false, // useTreeView - not in settings
		settings.TraceTimings,
		settings.TraceRules,
		nil, // codeStats - handled separately
		logger,
		settings.RootID,
		mergedConfig,
	)
	if err != nil {
		return nil, err
	}
	scanner.useLockFiles = settings.UseLockFiles
	return scanner, nil
}

// UseLockFiles returns whether lock files should be used for dependency resolution
func (s *Scanner) UseLockFiles() bool {
	return s.useLockFiles
}

// SetUseLockFiles sets whether lock files should be used for dependency resolution
func (s *Scanner) SetUseLockFiles(use bool) {
	s.useLockFiles = use
}

// scannerComponents holds all initialized scanner components
type scannerComponents struct {
	rules           []types.Rule
	depDetector     *DependencyDetector
	dotenvDetector  *parsers.DotenvDetector
	licenseDetector *license.LicenseDetector
	contentMatcher  *matchers.ContentMatcherRegistry
}

// initializeScannerComponents handles common initialization logic
func initializeScannerComponents(provider types.Provider, path string, logger *slog.Logger) (*scannerComponents, error) {
	// Load rules (simple, not lazy loaded - like TypeScript's loadAllRules)
	t1 := time.Now()
	loadedRules, err := rules.LoadEmbeddedRules()
	if err != nil {
		return nil, fmt.Errorf("failed to load rules: %w", err)
	}
	if logger != nil {
		logger.Debug("Loaded embedded rules", "count", len(loadedRules), "duration", time.Since(t1))
	}

	// Load types configuration
	t2 := time.Now()
	categoriesConfig, err := config.LoadCategoriesConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load categories config: %w", err)
	}
	SetCategoriesConfig(categoriesConfig)
	if logger != nil {
		logger.Debug("Loaded categories config", "duration", time.Since(t2))
	}

	// Initialize all detectors
	t3 := time.Now()
	depDetector := NewDependencyDetector(loadedRules)
	dotenvDetector := parsers.NewDotenvDetector(provider, loadedRules)
	licenseDetector := license.NewLicenseDetector()
	if logger != nil {
		logger.Debug("Initialized detectors", "duration", time.Since(t3))
	}

	// Build matchers from rules (like TypeScript's loadAllRules)
	t4 := time.Now()
	matchers.BuildFileMatchersFromRules(loadedRules)
	if logger != nil {
		logger.Debug("Built file matchers", "duration", time.Since(t4))
	}

	// Build content matchers from rules
	t5 := time.Now()
	contentMatcher := matchers.NewContentMatcherRegistry()
	if err := contentMatcher.BuildFromRules(loadedRules); err != nil {
		return nil, fmt.Errorf("failed to build content matchers: %w", err)
	}
	if logger != nil {
		logger.Debug("Built content matchers", "duration", time.Since(t5))
	}

	return &scannerComponents{
		rules:           loadedRules,
		depDetector:     depDetector,
		dotenvDetector:  dotenvDetector,
		licenseDetector: licenseDetector,
		contentMatcher:  contentMatcher,
	}, nil
}

// Scan performs analysis following the original TypeScript pattern
func (s *Scanner) Scan() (*types.Payload, error) {
	basePath := s.provider.GetBasePath()

	// Report scan start
	s.progress.ScanStart(basePath, s.excludePatterns)

	// Use the scanner's stored config (already merged with project config)
	cfg := s.config
	if cfg == nil {
		cfg = &config.ScanConfig{} // Use empty config if not set
	}

	// Create scan metadata
	scanMeta := metadata.NewScanMetadata(basePath, spec.Version)
	startTime := time.Now()

	// Create main payload like in TypeScript: new Payload({ name: 'main', folderPath: '/' })
	payload := types.NewPayloadWithPath("main", "/")

	// Configured techs are handled in cmd with validation
	// Do not add them here to avoid duplication

	// Set git information on payload BEFORE recursion starts
	// This ensures child components can check against parent git info
	t1 := time.Now()
	payload.Git = git.GetGitInfo(basePath)
	slog.Debug("Retrieved git info", "duration", time.Since(t1))

	// Start recursion from base path (like TypeScript's payload.recurse(provider, provider.basePath))
	slog.Debug("Starting directory recursion", "path", basePath)
	err := s.recurse(payload, basePath)
	if err != nil {
		return nil, err
	}
	slog.Debug("Completed directory recursion")

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

	// Set output format
	scanMeta.SetFormat("full")

	// Set git information directly on payload
	payload.Git = git.GetGitInfo(basePath)

	// Attach metadata to root payload
	payload.Metadata = scanMeta

	// Assign unique IDs to the entire payload tree
	payload.AssignIDs(s.resolveRootID(basePath))

	// Resolve inter-component references
	s.resolveComponentRefs(payload)

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
	// The provider's base path is already set to the directory containing the file
	basePath := s.provider.GetBasePath()

	// Create main payload (ID will be assigned at the end)
	payload := types.NewPayloadWithPath("main", "/")

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
		lang := s.langDetector.DetectLanguage(fileName, content)
		if s.codeStats.IsPerComponentEnabled() {
			// For single file scans, use the payload ID as component ID
			s.codeStats.ProcessFileForComponent(filePath, lang, content, payload.ID)
		} else {
			s.codeStats.ProcessFile(filePath, lang, content)
		}
	}

	// Add metadata for single file scan
	scanMeta := metadata.NewScanMetadata(basePath, spec.Version)
	fileCount, componentCount := s.countFilesAndComponents(payload)
	scanMeta.SetFileCounts(fileCount, componentCount)
	languageCount := s.countLanguages(payload)
	techCount, techsCount := s.countTechs(payload)
	scanMeta.SetLanguageCount(languageCount)
	scanMeta.SetTechCounts(techCount, techsCount)

	// Attach metadata to root payload
	payload.Metadata = scanMeta

	// Assign unique IDs to the payload tree
	payload.AssignIDs(s.resolveRootID(basePath))

	return payload, nil
}

// resolveRootID determines the root component ID using priority system:
// 1. CLI/config override (s.rootID)
// 2. Git remote URL (deterministic)
// 3. Absolute path (deterministic when no git)
func (s *Scanner) resolveRootID(basePath string) string {
	if s.rootID != "" {
		return s.rootID // CLI or config override
	}

	gitRootID := git.GenerateRootIDFromGit(basePath)
	if gitRootID != "" {
		return gitRootID // Git-based deterministic ID
	}

	// Use absolute path for deterministic ID when no git repository
	return git.GenerateRootIDFromPath(basePath)
}

// processFile handles language detection and code statistics for a single file
func (s *Scanner) processFile(ctx *types.Payload, dirPath string, fileName string) {
	fileFullPath := filepath.Join(dirPath, fileName)
	content, err := s.provider.ReadFile(fileFullPath)
	if err != nil {
		content = []byte{} // Empty content on error
	}

	// Detect language from file name
	lang := s.langDetector.DetectLanguage(fileName, content)
	if lang != "" {
		ctx.AddLanguage(lang)
	}

	// Collect code statistics if enabled
	if s.codeStats != nil {
		if s.codeStats.IsPerComponentEnabled() {
			s.codeStats.ProcessFileForComponent(fileFullPath, lang, content, ctx.ID)
		} else {
			s.codeStats.ProcessFile(fileFullPath, lang, content)
		}
	}
}

// recurse follows the exact TypeScript implementation pattern
func (s *Scanner) recurse(payload *types.Payload, filePath string) error {
	tEnter := time.Now()
	// Report entering directory
	s.progress.EnterDirectory(filePath)
	defer s.progress.LeaveDirectory(filePath)

	// Load .gitignore patterns for this directory and push to stack
	// This implements proper gitignore hierarchy: patterns only apply to current dir and subdirs
	tGitignore := time.Now()
	hasGitignore := s.gitignoreStack.LoadAndPushGitignore(filePath)
	if time.Since(tGitignore) > 100*time.Millisecond {
		slog.Debug("Gitignore loading slow", "path", filePath, "duration", time.Since(tGitignore))
	}

	// Report gitignore context for debugging
	if hasGitignore {
		s.progress.GitIgnoreEnter(filePath)
	}

	// Pop patterns when leaving this directory (only if we had a .gitignore)
	if hasGitignore {
		defer func() {
			s.progress.GitIgnoreLeave(filePath)
			s.gitignoreStack.PopGitignore()
		}()
	}

	// Get files in current directory (like TypeScript's provider.listDir)
	t1 := time.Now()
	files, err := s.provider.ListDir(filePath)
	if err != nil {
		return err
	}
	slog.Debug("Listed directory", "path", filePath, "file_count", len(files), "duration", time.Since(t1))

	// Filter files to exclude those matching ignore patterns
	// This ensures rule matching doesn't see excluded files
	t2 := time.Now()
	filteredFiles := make([]types.File, 0, len(files))
	for _, file := range files {
		if file.Type == "file" && s.shouldExcludeFileStackBased(file.Name, filePath) {
			continue
		}
		filteredFiles = append(filteredFiles, file)
	}
	if len(files) != len(filteredFiles) {
		slog.Debug("Filtered files", "path", filePath, "before", len(files), "after", len(filteredFiles), "duration", time.Since(t2))
	}

	// Start timing for folder file processing
	s.progress.FolderFileProcessingStart(filePath)

	// Apply rules to detect technologies (like TypeScript's ruleComponents loop)
	// This might return a different context if a component was detected
	t3 := time.Now()
	ctx := s.applyRules(payload, filteredFiles, filePath)
	if time.Since(t3) > 100*time.Millisecond {
		slog.Debug("Applied rules (slow)", "path", filePath, "duration", time.Since(t3))
	}

	// Check if this directory is a git repository and set git info
	// Only set git info if it's in a different repository than the parent context
	tGit := time.Now()
	if ctx.Git == nil {
		if gitInfo := s.getGitInfo(filePath); gitInfo != nil {
			// Only add git info if this directory is in a different repository than the parent context
			// For root level, parent is the same as payload, so we check payload.Git
			var parentGit *git.GitInfo
			if payload != nil {
				parentGit = payload.Git
			}
			if parentGit == nil || parentGit.RemoteURL != gitInfo.RemoteURL {
				ctx.Git = gitInfo
			}
		}
	}
	if time.Since(tGit) > 100*time.Millisecond {
		slog.Debug("Git info retrieval slow", "path", filePath, "duration", time.Since(tGit))
	}

	// Detect licenses from LICENSE files in this directory
	// This adds file-based license detection (MIT, Apache-2.0, etc.) from LICENSE files
	tLicense := time.Now()
	s.licenseDetector.AddLicensesToPayload(ctx, filePath)
	if time.Since(tLicense) > 100*time.Millisecond {
		slog.Debug("License detection slow", "path", filePath, "duration", time.Since(tLicense))
	}

	// End timing for folder file processing
	s.progress.FolderFileProcessingEnd(filePath)

	if time.Since(tEnter) > 500*time.Millisecond {
		slog.Debug("Directory processing slow", "path", filePath, "total_duration", time.Since(tEnter))
	}

	// Process each file/directory (exactly like TypeScript's loop)
	for _, file := range filteredFiles {
		if file.Type == "file" {
			s.processFile(ctx, filePath, file.Name)
			continue
		}

		// Skip ignored directories (like TypeScript's IGNORED_DIVE_PATHS)
		// Use stack-based gitignore checking for proper hierarchy
		if s.shouldIgnoreDirectoryStackBased(file.Name, filePath) {
			continue
		}

		// Recurse into subdirectories
		subPath := filepath.Join(filePath, file.Name)
		if err := s.recurse(ctx, subPath); err != nil {
			// Continue processing other directories even if one fails
			continue
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

	// 1. Component-based detection (all plugin detectors including GitHub Actions)
	ctx = s.detectComponents(payload, ctx, files, currentPath)

	// 2. Dotenv detection
	s.detectDotenv(ctx, files, currentPath)

	// 4. File and extension-based detection (includes JSON schema via content matchers)
	matchedTechs := s.detectByFilesAndExtensions(ctx, files, currentPath)

	// 6. File-based rule detection
	s.detectByRuleFiles(ctx, files, matchedTechs)

	return ctx
}

func (s *Scanner) detectComponents(payload, ctx *types.Payload, files []types.File, currentPath string) *types.Payload {
	var namedComponents []*types.Payload
	var virtualComponents []*types.Payload

	// Collect all components from all detectors
	for _, detector := range components.GetDetectors() {
		detectedComponents := detector.Detect(files, currentPath, s.provider.GetBasePath(), s.provider, s.depDetector)
		for _, component := range detectedComponents {
			// Note: Components should NOT get git info by default
			// Git info is only added at directory level when component is in a different repository
			// This prevents redundant git info for components in the same repo as their parent

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

	// Handle named components - keep them separate to preserve granularity
	if len(namedComponents) == 0 {
		return ctx
	}

	// Add each component separately (don't merge)
	// This preserves architectural clarity and dependency tracking
	for _, component := range namedComponents {
		ctx = s.addNamedComponent(payload, component, currentPath)
	}

	return ctx
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

// detectByRuleFiles matches rules that have specific file requirements
func (s *Scanner) detectByRuleFiles(ctx *types.Payload, files []types.File, matchedTechs map[string]bool) {
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

	// Add edges if configured to do so
	if addEdges && ShouldCreateEdges(rule) {
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

// shouldExcludeFileStackBased checks if a file should be excluded using stack-based gitignore approach
// This implements proper gitignore hierarchy where patterns only apply to their directory and subdirectories
func (s *Scanner) shouldExcludeFileStackBased(fileName, currentPath string) bool {
	// Get relative path from base path
	basePath := s.provider.GetBasePath()
	fullPath := filepath.Join(currentPath, fileName)
	relPath, err := filepath.Rel(basePath, fullPath)
	if err != nil {
		relPath = fileName // Fallback to just filename
	}

	// Check against CLI exclude patterns first (these apply globally)
	for _, pattern := range s.excludePatterns {
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

	// Check against stack-based gitignore patterns (proper hierarchy)
	if s.gitignoreStack.ShouldExclude(fileName, relPath) {
		return true
	}

	return false
}

// shouldIgnoreDirectoryStackBased checks if a directory should be ignored using stack-based gitignore approach
func (s *Scanner) shouldIgnoreDirectoryStackBased(name, parentPath string) bool {
	// Check user-specified exclude patterns first (supports glob patterns)
	if len(s.excludePatterns) > 0 {
		for _, pattern := range s.excludePatterns {
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

	// Check against stack-based gitignore patterns
	fullDirPath := filepath.Join(parentPath, name)
	basePath := s.provider.GetBasePath()
	relPath, err := filepath.Rel(basePath, fullDirPath)
	if err != nil {
		relPath = name // Fallback to just directory name
	}

	if s.gitignoreStack.ShouldExclude(name, relPath) {
		return true
	}

	return false
}

// getGitInfo retrieves git info with caching by repository root
// This avoids expensive worktree.Status() calls for the same repo
func (s *Scanner) getGitInfo(path string) *git.GitInfo {
	// Check if we already know the repo root for this path
	repoRoot, rootKnown := s.gitRootCache[path]

	if !rootKnown {
		// Find repo root (walks up looking for .git)
		// Note: We can't use parent's repo root as optimization because
		// nested git repos (submodules) would be incorrectly assigned to parent
		repoRoot = git.FindRepoRoot(path)
		s.gitRootCache[path] = repoRoot // Cache even empty string (not a repo)
	}

	if repoRoot == "" {
		return nil
	}

	// Check git info cache BEFORE doing expensive work
	if cached, ok := s.gitCache[repoRoot]; ok {
		return cached
	}

	// Not in cache - do the expensive GetGitInfo call (only once per repo)
	gitInfo, _ := git.GetGitInfoWithRoot(path)
	if gitInfo == nil {
		return nil
	}

	// Cache and return
	s.gitCache[repoRoot] = gitInfo
	return gitInfo
}
