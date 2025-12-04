package git

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/progress"
	"github.com/sirupsen/logrus"
)

// Loader handles loading ignore patterns from .gitignore files
type Loader struct {
	progress *progress.Progress
	logger   *logrus.Logger
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
func NewGitignoreLoaderWithLogger(prog *progress.Progress, logger *logrus.Logger) *Loader {
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
	err := filepath.Walk(scanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err // Skip if we can't access a path
		}

		// Only process .gitignore files
		if info.Name() == ".gitignore" && !info.IsDir() {
			// Skip .gitignore files in cache/temp directories - these often contain "*"
			// which would exclude everything inappropriately
			dir := filepath.Dir(path)
			if l.shouldSkipGitignore(dir) {
				if l.logger != nil {
					l.logger.Debugf("Skipping .gitignore in cache directory: %s", path)
				}
				return nil
			}

			gitignoreFiles = append(gitignoreFiles, path)
			patterns, err := l.loadPatternsFromFile(path)
			if err != nil {
				// Log error but continue processing other files
				if l.progress != nil {
					l.progress.Info(fmt.Sprintf("Warning: Failed to read %s: %v", path, err))
				}
				if l.logger != nil {
					l.logger.Errorf("Failed to read .gitignore file %s: %v", path, err)
				}
				return nil
			}

			// Debug logging for loaded patterns
			if l.logger != nil {
				l.logger.Debugf("Loaded %d patterns from %s: %v", len(patterns), path, patterns)
			}
			allPatterns = append(allPatterns, patterns...)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking directory tree: %w", err)
	}

	// Report gitignore loading info through progress system (consistent across verbose/debug)
	if l.progress != nil {
		if len(gitignoreFiles) > 0 {
			l.progress.Info(fmt.Sprintf("Loaded %d patterns from %d .gitignore files", len(allPatterns), len(gitignoreFiles)))
		} else {
			l.progress.Info("No .gitignore files found")
		}
	}

	// Detailed debug logging
	if l.logger != nil {
		l.logger.Debugf("Gitignore loading complete:")
		l.logger.Debugf("  - Total .gitignore files processed: %d", len(gitignoreFiles))
		for _, file := range gitignoreFiles {
			l.logger.Debugf("    - %s", file)
		}
		l.logger.Debugf("  - Total unique patterns after deduplication: %d", len(l.deduplicatePatterns(allPatterns)))
	}

	// Deduplicate patterns
	return l.deduplicatePatterns(allPatterns), nil
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
