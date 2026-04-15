package git

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/petrarca/tech-stack-analyzer/internal/progress"
)

// Pattern represents a single parsed gitignore pattern with metadata.
type Pattern struct {
	Glob    string // The glob expression (without leading ! or trailing /)
	Negate  bool   // True if this is a negation pattern (re-includes a previously excluded path)
	DirOnly bool   // True if the original pattern had a trailing slash (matches directories only)
}

// ParsePatterns parses gitignore-style lines into structured Pattern values.
func ParsePatterns(lines []string) []Pattern {
	var patterns []Pattern
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p := Pattern{}
		if strings.HasPrefix(line, "!") {
			p.Negate = true
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			p.DirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		p.Glob = line
		patterns = append(patterns, p)
	}
	return patterns
}

// loadPatternsFromGitignore loads patterns from a specific .gitignore file.
// Returns raw non-empty, non-comment lines preserving negation (!) and trailing slash (/) syntax.
func loadPatternsFromGitignore(gitignorePath string) ([]string, error) {
	file, err := os.Open(gitignorePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .gitignore: %w", err)
	}
	defer file.Close()

	var lines []string
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("error reading .gitignore: %w", err)
	}
	return lines, nil
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
		gl.logger.Debug("Loaded patterns from file", "path", path, "count", len(patterns))
	}
}

// GitignoreStack represents a stack of .gitignore pattern sets
type GitignoreStack struct {
	stack []*PatternSet
}

// PatternSet represents patterns from a single .gitignore file
type PatternSet struct {
	Directory string    // Directory where this .gitignore was found
	Patterns  []Pattern // Parsed patterns from this .gitignore
}

// NewGitignoreStack creates a new empty gitignore stack
func NewGitignoreStack() *GitignoreStack {
	return &GitignoreStack{
		stack: make([]*PatternSet, 0),
	}
}

// Push parses raw gitignore lines and adds them to the stack.
func (gs *GitignoreStack) Push(directory string, rawLines []string) {
	parsed := ParsePatterns(rawLines)
	if len(parsed) == 0 {
		return
	}
	gs.stack = append(gs.stack, &PatternSet{Directory: directory, Patterns: parsed})
}

// Pop removes the top pattern set from the stack
func (gs *GitignoreStack) Pop() {
	if len(gs.stack) > 0 {
		gs.stack = gs.stack[:len(gs.stack)-1]
	}
}

// ShouldExclude checks if a path should be excluded based on the current stack.
// Implements gitignore last-match-wins semantics: the last matching pattern
// determines whether the path is excluded (positive match) or re-included (negation).
// isDir should be true when checking a directory path.
func (gs *GitignoreStack) ShouldExclude(name, relativePath string, isDir bool) bool {
	excluded := false
	for _, ps := range gs.stack {
		for _, p := range ps.Patterns {
			if p.DirOnly && !isDir {
				continue
			}
			if !matchPattern(p.Glob, name, relativePath) {
				continue
			}
			excluded = !p.Negate
		}
	}
	return excluded
}

// matchPattern checks whether a glob matches a name or relative path.
func matchPattern(glob, name, relativePath string) bool {
	if matched, err := doublestar.Match(glob, relativePath); err == nil && matched {
		return true
	}
	if matched, err := doublestar.Match(glob, name); err == nil && matched {
		return true
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

	// Always exclude .git directory (Git's internal metadata)
	// Git itself ignores this directory, but we need to exclude it explicitly
	// for performance (prevents scanning .git/objects/* structure)
	allExcludes = append(allExcludes, ".git")

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

// ShouldExclude checks if a file/directory should be excluded based on current stack.
// isDir should be true when checking a directory path.
func (l *StackBasedLoader) ShouldExclude(name, relativePath string, isDir bool) bool {
	return l.stack.ShouldExclude(name, relativePath, isDir)
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

// LoadPatternsFromFile loads raw (non-empty, non-comment) lines from a .gitignore file.
// The returned lines preserve negation (!) and trailing slash (/) syntax for use with ParsePatterns.
func LoadPatternsFromFile(gitignorePath string) ([]string, error) {
	return loadPatternsFromGitignore(gitignorePath)
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
