package git

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/petrarca/tech-stack-analyzer/internal/progress"
)

// loadPatternsFromGitignore loads patterns from a specific .gitignore file
func loadPatternsFromGitignore(gitignorePath string) ([]string, error) {
	file, err := os.Open(gitignorePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .gitignore: %w", err)
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Remove trailing slashes for consistency (dir/ -> dir)
		pattern := strings.TrimSuffix(line, "/")

		// Skip negation patterns for now (they start with !)
		// These are complex to handle properly in a glob matcher
		if strings.HasPrefix(pattern, "!") {
			continue
		}

		patterns = append(patterns, pattern)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading .gitignore: %w", err)
	}

	return patterns, nil
}

// gitignoreLogger provides common logging functionality for gitignore operations
type gitignoreLogger struct {
	progress *progress.Progress
	logger   *slog.Logger
}

func (gl *gitignoreLogger) logError(path string, err error) {
	if gl.progress != nil {
		gl.progress.Info(fmt.Sprintf("Warning: Failed to read %s: %v", path, err))
	}
	if gl.logger != nil {
		gl.logger.Error("Failed to read .gitignore file", "path", path, "error", err)
	}
}

func (gl *gitignoreLogger) logLoaded(path string, patterns []string) {
	if gl.logger != nil {
		gl.logger.Debug("Loaded patterns from file", "path", path, "patterns", patterns, "count", len(patterns))
	}
}

// GitignoreStack represents a stack of .gitignore pattern sets
type GitignoreStack struct {
	stack []*PatternSet
}

// PatternSet represents patterns from a single .gitignore file
type PatternSet struct {
	Directory string   // Directory where this .gitignore was found
	Patterns  []string // Patterns from this .gitignore
}

// NewGitignoreStack creates a new empty gitignore stack
func NewGitignoreStack() *GitignoreStack {
	return &GitignoreStack{
		stack: make([]*PatternSet, 0),
	}
}

// Push adds patterns from a .gitignore file to the stack
func (gs *GitignoreStack) Push(directory string, patterns []string) {
	if len(patterns) == 0 {
		return // Don't push empty pattern sets
	}

	patternSet := &PatternSet{
		Directory: directory,
		Patterns:  patterns,
	}
	gs.stack = append(gs.stack, patternSet)
}

// Pop removes the top pattern set from the stack
func (gs *GitignoreStack) Pop() {
	if len(gs.stack) > 0 {
		gs.stack = gs.stack[:len(gs.stack)-1]
	}
}

// GetAllPatterns returns all patterns from the entire stack (in order)
func (gs *GitignoreStack) GetAllPatterns() []string {
	var allPatterns []string
	for _, patternSet := range gs.stack {
		allPatterns = append(allPatterns, patternSet.Patterns...)
	}
	return allPatterns
}

// ShouldExclude checks if a file/directory should be excluded based on the current stack
func (gs *GitignoreStack) ShouldExclude(name, relativePath string) bool {
	patterns := gs.GetAllPatterns()

	for _, pattern := range patterns {
		// Try glob match against relative path
		matched, err := doublestar.Match(pattern, relativePath)
		if err == nil && matched {
			return true
		}

		// Also try matching just the filename
		matched, err = doublestar.Match(pattern, name)
		if err == nil && matched {
			return true
		}
	}

	return false
}

// GetStackDepth returns the current depth of the stack
func (gs *GitignoreStack) GetStackDepth() int {
	return len(gs.stack)
}

// StackBasedLoader handles loading ignore patterns in a stack-based approach
type StackBasedLoader struct {
	progress *progress.Progress
	logger   *slog.Logger
	stack    *GitignoreStack
	basePath string // Store base path for .git/info/exclude
}

// NewStackBasedLoader creates a new stack-based gitignore loader
func NewStackBasedLoader() *StackBasedLoader {
	return &StackBasedLoader{
		stack: NewGitignoreStack(),
	}
}

// NewStackBasedLoaderWithProgress creates a new stack-based gitignore loader with progress reporting
func NewStackBasedLoaderWithProgress(prog *progress.Progress) *StackBasedLoader {
	return &StackBasedLoader{
		progress: prog,
		stack:    NewGitignoreStack(),
	}
}

// NewStackBasedLoaderWithLogger creates a new stack-based gitignore loader with logging
func NewStackBasedLoaderWithLogger(prog *progress.Progress, logger *slog.Logger) *StackBasedLoader {
	return &StackBasedLoader{
		progress: prog,
		logger:   logger,
		stack:    NewGitignoreStack(),
	}
}

// InitializeWithTopLevelExcludes adds config and CLI excludes as a top-level pseudo .gitignore
func (l *StackBasedLoader) InitializeWithTopLevelExcludes(basePath string, excludePatterns []string, configExcludes []string) error {
	l.basePath = basePath
	var allExcludes []string

	// Add CLI exclude patterns first (lowest priority)
	allExcludes = append(allExcludes, excludePatterns...)

	// Add config exclude patterns
	allExcludes = append(allExcludes, configExcludes...)

	// If we have any excludes, add them as a bottom-level pseudo .gitignore
	if len(allExcludes) > 0 {
		l.stack.Push(basePath, allExcludes)
		if l.logger != nil {
			l.logger.Info("Added top-level excludes",
				"base_path", basePath,
				"exclude_count", len(allExcludes),
				"patterns", allExcludes)
		}
	}

	// Add .git/info/exclude patterns with highest priority
	if gitInfoPatterns, err := l.loadGitInfoExclude(); err == nil && len(gitInfoPatterns) > 0 {
		l.stack.Push(basePath, gitInfoPatterns)
		if l.logger != nil {
			l.logger.Info("Loaded .git/info/exclude patterns",
				"path", filepath.Join(basePath, ".git/info/exclude"),
				"count", len(gitInfoPatterns))
		}
	}

	return nil
}

// LoadAndPushGitignore loads .gitignore for directory and pushes to stack if found
// Returns true if .gitignore was found and loaded successfully
func (l *StackBasedLoader) LoadAndPushGitignore(directory string) bool {
	gitignorePath := filepath.Join(directory, ".gitignore")

	// Check if .gitignore exists in this directory
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		return false // No .gitignore in this directory
	}

	// Load patterns from .gitignore
	patterns, err := loadPatternsFromGitignore(gitignorePath)
	if err != nil {
		l.log().logError(gitignorePath, err)
		return false
	}

	// Push to stack and log success
	l.stack.Push(directory, patterns)
	l.log().logLoaded(gitignorePath, patterns)
	return true
}

// log returns a gitignoreLogger for this loader
func (l *StackBasedLoader) log() *gitignoreLogger {
	return &gitignoreLogger{progress: l.progress, logger: l.logger}
}

// PopGitignore removes patterns from stack when leaving directory
func (l *StackBasedLoader) PopGitignore() {
	if l.stack.GetStackDepth() > 0 {
		l.stack.Pop()
	}
}

// ShouldExclude checks if a file/directory should be excluded based on current stack
func (l *StackBasedLoader) ShouldExclude(name, relativePath string) bool {
	return l.stack.ShouldExclude(name, relativePath)
}

// GetStack returns the current gitignore stack (for testing/debugging)
func (l *StackBasedLoader) GetStack() *GitignoreStack {
	return l.stack
}

// loadGitInfoExclude loads .git/info/exclude patterns for the base path
func (l *StackBasedLoader) loadGitInfoExclude() ([]string, error) {
	// Find .git directory (could be .git or in gitdir file for submodules/worktrees)
	gitDir, err := findGitDir(l.basePath)
	if err != nil {
		return nil, err // Not a git repo, that's OK
	}

	return loadGitInfoExclude(gitDir)
}

// Loader handles loading ignore patterns from .gitignore files (legacy approach)
type Loader struct {
	progress *progress.Progress
	logger   *slog.Logger
}

// NewGitignoreLoader creates a new gitignore loader
func NewGitignoreLoader() *Loader {
	return &Loader{}
}

// NewGitignoreLoaderWithProgress creates a new gitignore loader with progress reporting
func NewGitignoreLoaderWithProgress(prog *progress.Progress) *Loader {
	return &Loader{
		progress: prog,
	}
}

// NewGitignoreLoaderWithLogger creates a new gitignore loader with logging
func NewGitignoreLoaderWithLogger(prog *progress.Progress, logger *slog.Logger) *Loader {
	return &Loader{
		progress: prog,
		logger:   logger,
	}
}

// LoadPatterns loads ignore patterns from .gitignore files recursively
// Searches from the scan path down through all subdirectories
// Skips .gitignore files in directories that are typically cache/temp directories
func (l *Loader) LoadPatterns(scanPath string) ([]string, error) {
	var allPatterns []string
	var gitignoreFiles []string

	// Find all .gitignore files recursively
	err := filepath.Walk(scanPath, l.processGitignoreFile(&allPatterns, &gitignoreFiles))
	if err != nil {
		return nil, fmt.Errorf("error walking directory tree: %w", err)
	}

	// Report gitignore loading info through progress system (consistent across verbose/debug)
	l.reportLoadingProgress(len(allPatterns), len(gitignoreFiles))

	// Detailed debug logging
	l.logLoadingDetails(gitignoreFiles, allPatterns)

	// Deduplicate patterns
	return l.deduplicatePatterns(allPatterns), nil
}

// processGitignoreFile returns a filepath.WalkFunc that processes .gitignore files
func (l *Loader) processGitignoreFile(allPatterns *[]string, gitignoreFiles *[]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err // Skip if we can't access a path
		}

		// Only process .gitignore files
		if info.Name() == ".gitignore" && !info.IsDir() {
			if l.handleGitignoreFile(path, allPatterns, gitignoreFiles) {
				return nil // File was processed successfully
			}
		}

		return nil
	}
}

// handleGitignoreFile processes a single .gitignore file and returns true if successful
func (l *Loader) handleGitignoreFile(path string, allPatterns *[]string, gitignoreFiles *[]string) bool {
	*gitignoreFiles = append(*gitignoreFiles, path)
	patterns, err := loadPatternsFromGitignore(path)
	if err != nil {
		l.log().logError(path, err)
		return false
	}

	l.log().logLoaded(path, patterns)
	*allPatterns = append(*allPatterns, patterns...)
	return true
}

// log returns a gitignoreLogger for this loader
func (l *Loader) log() *gitignoreLogger {
	return &gitignoreLogger{progress: l.progress, logger: l.logger}
}

// reportLoadingProgress reports loading progress through the progress system
func (l *Loader) reportLoadingProgress(patternCount, fileCount int) {
	if l.progress != nil {
		if fileCount > 0 {
			l.progress.Info(fmt.Sprintf("Loaded %d patterns from %d .gitignore files", patternCount, fileCount))
		} else {
			l.progress.Info("No .gitignore files found")
		}
	}
}

// logLoadingDetails provides detailed debug logging about the loading process
func (l *Loader) logLoadingDetails(gitignoreFiles []string, allPatterns []string) {
	if l.logger != nil {
		l.logger.Debug("Gitignore loading complete:")
		l.logger.Debug("  - Total .gitignore files processed", "count", len(gitignoreFiles))
		for _, file := range gitignoreFiles {
			l.logger.Debug("    - " + file)
		}
		l.logger.Debug("  - Total unique patterns after deduplication", "count", len(l.deduplicatePatterns(allPatterns)))
	}
}

// LoadPatternsFromFile loads patterns from a specific .gitignore file path
// Useful for testing or custom paths
func (l *Loader) LoadPatternsFromFile(gitignorePath string) ([]string, error) {
	return loadPatternsFromGitignore(gitignorePath)
}

// deduplicatePatterns removes duplicate patterns while preserving order
func (l *Loader) deduplicatePatterns(patterns []string) []string {
	// Return empty slice if no patterns
	if len(patterns) == 0 {
		return []string{}
	}

	seen := make(map[string]bool)
	var result []string

	for _, pattern := range patterns {
		if !seen[pattern] {
			seen[pattern] = true
			result = append(result, pattern)
		}
	}

	return result
}

// loadGitInfoExclude loads patterns from .git/info/exclude
func loadGitInfoExclude(gitDir string) ([]string, error) {
	excludePath := filepath.Join(gitDir, "info", "exclude")

	// Check if file exists
	if _, err := os.Stat(excludePath); os.IsNotExist(err) {
		return nil, nil // No .git/info/exclude file
	}

	return loadPatternsFromGitignore(excludePath)
}

// findGitDir finds the .git directory (handles submodules, worktrees, etc.)
func findGitDir(startPath string) (string, error) {
	// Check for .git file (worktree/submodule)
	gitFile := filepath.Join(startPath, ".git")
	if content, err := os.ReadFile(gitFile); err == nil {
		gitDir := strings.TrimSpace(string(content))
		if strings.HasPrefix(gitDir, "gitdir: ") {
			return filepath.Join(startPath, strings.TrimPrefix(gitDir, "gitdir: ")), nil
		}
	}

	// Check for .git directory
	gitDir := filepath.Join(startPath, ".git")
	if stat, err := os.Stat(gitDir); err == nil && stat.IsDir() {
		return gitDir, nil
	}

	return "", fmt.Errorf("not a git repository")
}
