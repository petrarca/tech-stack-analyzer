package git

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/petrarca/tech-stack-analyzer/internal/progress"
)

// Loader handles loading ignore patterns from .gitignore files
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
	// Skip .gitignore files in cache/temp directories
	dir := filepath.Dir(path)
	if l.shouldSkipGitignore(dir) {
		l.logSkippedGitignore(path)
		return false
	}

	*gitignoreFiles = append(*gitignoreFiles, path)
	patterns, err := l.loadPatternsFromFile(path)
	if err != nil {
		l.logGitignoreError(path, err)
		return false
	}

	l.logLoadedPatterns(path, patterns)
	*allPatterns = append(*allPatterns, patterns...)
	return true
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

// logSkippedGitignore logs when a .gitignore file is skipped
func (l *Loader) logSkippedGitignore(path string) {
	if l.logger != nil {
		l.logger.Debug("Skipping .gitignore in cache directory", "path", path)
	}
}

// logGitignoreError logs errors when reading .gitignore files
func (l *Loader) logGitignoreError(path string, err error) {
	if l.progress != nil {
		l.progress.Info(fmt.Sprintf("Warning: Failed to read %s: %v", path, err))
	}
	if l.logger != nil {
		l.logger.Error("Failed to read .gitignore file", "path", path, "error", err)
	}
}

// logLoadedPatterns logs successfully loaded patterns
func (l *Loader) logLoadedPatterns(path string, patterns []string) {
	if l.logger != nil {
		l.logger.Debug("Loaded patterns from file", "path", path, "patterns", patterns, "count", len(patterns))
	}
}

// shouldSkipGitignore checks if we should skip loading a .gitignore file
// based on its directory location
func (l *Loader) shouldSkipGitignore(dir string) bool {
	basename := filepath.Base(dir)

	// Skip cache, temp, and build directories that typically contain "*" in their .gitignore
	skipDirs := []string{
		".pytest_cache", ".ruff_cache", ".tox", ".coverage", ".mypy_cache",
		".venv", "venv", "env", "__pycache__", ".git", ".svn", ".hg",
		"node_modules", ".npm", ".yarn", ".pnpm", "dist", "build",
		".next", ".nuxt", ".target", "site",
	}

	for _, skip := range skipDirs {
		if basename == skip {
			return true
		}
	}

	return false
}

// loadPatternsFromFile loads patterns from a specific .gitignore file
func (l *Loader) loadPatternsFromFile(gitignorePath string) ([]string, error) {
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

// LoadPatternsFromFile loads patterns from a specific .gitignore file path
// Useful for testing or custom paths
func (l *Loader) LoadPatternsFromFile(gitignorePath string) ([]string, error) {
	return l.loadPatternsFromFile(gitignorePath)
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
