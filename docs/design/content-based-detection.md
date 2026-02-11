# Content-Based Detection

## Overview

Content-based detection enables technology identification through **pattern matching in file contents**. This is an **independent detection mechanism** that operates separately from extension/file-based detection, allowing precise identification of technologies that share common file extensions.

## Key Principle: Independence

**Content matching is NOT additive to extensions - it's completely independent.**

A technology can be detected by:
- Extension/file presence alone (`.py` → python)
- Content patterns alone (`Q_OBJECT` in `.cpp` → qt)
- Dependencies alone (`"react"` in package.json → react)
- Any combination of the above (OR logic)

## Architecture

### Components

1. **ContentRule** (`internal/types/rule.go`)
   ```go
   type ContentRule struct {
       Pattern    string   // Regex pattern to match
       Extensions []string // WHERE to check (e.g., [".cpp", ".h"])
       Files      []string // OR specific files (e.g., ["CMakeLists.txt"])
   }
   ```

2. **ContentMatcherRegistry** (`internal/scanner/matchers/content.go`)
   - **O(1) hash map lookups** by extension/filename
   - Pre-compiled regex patterns
   - Two separate registries:
     - `matchers map[string][]*ContentMatcher` - Key: extension (e.g., ".cpp")
     - `fileMatchers map[string][]*ContentMatcher` - Key: filename (e.g., "CMakeLists.txt")

3. **Scanner Integration** (`internal/scanner/scanner.go`)
   - `detectByContent()` - Processes files with content matchers
   - Only reads files that have registered content patterns
   - Independent from extension/file detection phase

## Rule Structure

### Content-Only Detection (No Top-Level Extensions)
```yaml
tech: mfc
name: Microsoft Foundation Class Library
type: ui_framework
content:
  - pattern: '#include\s+<afx'
    extensions: [.cpp, .h, .hpp]  # Defines WHERE to check
  - pattern: 'BEGIN_MESSAGE_MAP'
    extensions: [.cpp, .h, .hpp]
```
**Result:** MFC detected ONLY if patterns found in `.cpp/.h` files. No false positives!

### Hybrid Detection (Extension + Content)
```yaml
tech: qt
name: Qt Framework
type: ui
extensions: [.pro, .ui, .qrc]  # Qt-specific files (simple detection)
content:
  - pattern: 'Q_OBJECT'
    extensions: [.cpp, .h]       # Check C++ files for Qt code
  - pattern: 'Qt[0-9]::'
    files: [CMakeLists.txt]      # Check specific file
```
**Result:** Qt detected by `.pro` file OR `Q_OBJECT` in `.cpp` OR `Qt6::` in `CMakeLists.txt`

### File-Specific Patterns
```yaml
tech: qt
content:
  - pattern: 'find_package\s*\(\s*Qt[0-9]'
    files: [CMakeLists.txt]  # ONLY check CMakeLists.txt
```
**Result:** Pattern only checked in `CMakeLists.txt`, not in other files

## Detection Flow

### Independent Phases (OR Logic)

```
Phase 1: Extension/File Detection
  .py file exists → python detected
  package.json exists → nodejs detected

Phase 2: Content Detection (INDEPENDENT)
  .cpp file with Q_OBJECT → qt detected
  .cpp file with #include <afx → mfc detected
  CMakeLists.txt with Qt6:: → qt detected

Phase 3: Dependency Detection (INDEPENDENT)
  "react" in package.json → react detected

All results combined (OR)
```

### Content Matching Process

```go
// 1. Check if content matchers exist for this file
hasFileMatchers := contentMatcher.HasFileMatchers("CMakeLists.txt")
hasExtMatchers := contentMatcher.HasContentMatchers(".cpp")

if !hasFileMatchers && !hasExtMatchers {
    return  // Skip - no content patterns for this file
}

// 2. Read file content (only if matchers exist)
content := readFile(file)

// 3. Match patterns (O(1) lookup + regex matching)
matches := contentMatcher.Match(file, content)
// Returns: {"qt": ["content matched: Q_OBJECT"]}

// 4. Add to payload
for tech, reasons := range matches {
    payload.AddTech(tech, reasons)
}
```

### Key Principles

1. **Independence**: Content matching is NOT tied to top-level extensions
2. **Restrictions Required**: Content patterns MUST specify `extensions` or `files`
3. **No False Positives**: Pure C++ projects don't match MFC/Qt
4. **Performance**: O(1) lookup, only reads files with registered patterns
5. **Early Exit**: Stops after first match per tech

## When to Use Content Detection

### Perfect Use Cases

**Distinguish Similar Technologies:**
- **MFC vs Qt vs Pure C++** in `.cpp/.h` files
- **React vs Preact vs Vue** in `.jsx` files
- **OpenGL vs DirectX** in graphics code

**Library Detection Without Package Managers:**
- **C++ libraries**: STL, Boost, Qt, MFC
- **Header-only libraries**: Catch2, doctest
- **Embedded frameworks**: Arduino, ESP-IDF

**Framework-Specific Patterns:**
- **React Hooks**: `useState`, `useEffect`
- **Vue Composition API**: `ref`, `reactive`
- **Qt Macros**: `Q_OBJECT`, `Q_PROPERTY`

**Build System Detection:**
- **CMake with Qt**: `find_package(Qt6)`
- **CMake with Boost**: `find_package(Boost)`
- **Makefile patterns**: Specific compiler flags

### Don't Use For

**Already Definitive Detection:**
- Package dependencies (npm, pip, maven) - use `dependencies` field
- Unique config files (`.pro`, `.ui`) - use `extensions` field
- Specific filenames (`package.json`) - use `files` field

**Performance Concerns:**
- Very large files (>10MB) - content reading is expensive
- Binary files - regex won't work
- Redundant validation - if extension is sufficient

## Performance Characteristics

### O(1) Hash Map Lookups

```go
// Extension-based lookup
matchers := registry.matchers[".cpp"]  // O(1)
// Returns: [qt_matcher, mfc_matcher, stl_matcher]

// File-based lookup
matchers := registry.fileMatchers["CMakeLists.txt"]  // O(1)
// Returns: [qt_matcher, cmake_matcher]
```

**Performance Benefits:**
- **O(1) lookup** - Hash map access by extension/filename
- **Pre-compiled regex** - Patterns compiled once at startup
- **Filtered checks** - Only check relevant patterns per file
- **Early exit** - Stop after first match per tech
- **Lazy file reading** - Only read files with registered patterns

**Example:** For a `.cpp` file:
- Without hash map: Check all 700 rules = 700 iterations
- With hash map: Lookup `.cpp` matchers = ~10 patterns to check
- **70x faster!**

### Memory Efficiency

- **No file caching** - Read once, process, discard
- **Streaming architecture** - Process files as encountered
- **Minimal overhead** - Rules without content patterns have zero impact

## Real-World Examples

### MFC Detection (Content-Only)
```yaml
tech: mfc
name: Microsoft Foundation Class Library
type: ui_framework
content:
  - pattern: '#include\s+<afx'
    extensions: [.cpp, .h, .hpp]
  - pattern: 'class\s+\w+\s*:\s*public\s+C(Wnd|FrameWnd|Dialog)'
    extensions: [.cpp, .h, .hpp]
  - pattern: 'BEGIN_MESSAGE_MAP|END_MESSAGE_MAP'
    extensions: [.cpp, .h, .hpp]
```
**Behavior:**
- `.cpp` with `#include <afxwin.h>` → MFC detected
- `.cpp` without MFC patterns → MFC NOT detected
- Pure C++ project → No false positives

### Qt Detection (Hybrid)
```yaml
tech: qt
name: Qt Framework
type: ui
extensions: [.pro, .ui, .qrc]  # Qt-specific files
content:
  - pattern: 'Q_OBJECT'
    extensions: [.cpp, .h, .hpp, .c]
  - pattern: '#include\s+<Qt[A-Z][a-zA-Z]+>'
    extensions: [.cpp, .h, .hpp, .c]
  - pattern: 'Qt[0-9]::'
    files: [CMakeLists.txt]
  - pattern: 'find_package\s*\(\s*Qt[0-9]'
    files: [CMakeLists.txt]
```
**Behavior:**
- `.pro` file → Qt detected (extension match)
- `.cpp` with `Q_OBJECT` → Qt detected (content match)
- `CMakeLists.txt` with `Qt6::` → Qt detected (content match)
- Pure C++ project → Qt NOT detected

### OpenGL Detection
```yaml
tech: opengl
name: OpenGL
type: library
content:
  - pattern: '#include\s+<GL/'
    extensions: [.c, .cpp, .h]
  - pattern: '\b(glBegin|glEnd|glVertex|glColor)\b'
    extensions: [.c, .cpp, .h]
```

### React Hooks Detection
```yaml
tech: react
content:
  - pattern: '\b(useState|useEffect|useContext|useReducer)\b'
    extensions: [.js, .jsx, .ts, .tsx]
```

## Implementation Details

### Building Content Matchers

```go
func (r *ContentMatcherRegistry) BuildFromRules(rules []types.Rule) error {
    for _, rule := range rules {
        // Skip rules with dependencies
        if len(rule.Dependencies) > 0 {
            continue
        }
        
        // Skip rules without content patterns
        if len(rule.Content) == 0 {
            continue
        }
        
        // Check if any content pattern has extensions/files
        hasValidPatterns := false
        for _, contentRule := range rule.Content {
            if len(contentRule.Extensions) > 0 || len(contentRule.Files) > 0 {
                hasValidPatterns = true
                break
            }
        }
        
        // Allow fallback to top-level extensions if needed
        if !hasValidPatterns && len(rule.Extensions) == 0 {
            continue  // Skip - no way to determine which files to check
        }
        
        // Register patterns in hash maps
        for _, contentRule := range rule.Content {
            pattern := regexp.MustCompile(contentRule.Pattern)
            matcher := &ContentMatcher{Tech: rule.Tech, Pattern: pattern}
            
            if len(contentRule.Files) > 0 {
                // File-specific matcher
                for _, filename := range contentRule.Files {
                    r.fileMatchers[filename] = append(r.fileMatchers[filename], matcher)
                }
            } else {
                // Extension-based matcher
                extensions := contentRule.Extensions
                if len(extensions) == 0 {
                    extensions = rule.Extensions  // Fallback
                }
                for _, ext := range extensions {
                    r.matchers[ext] = append(r.matchers[ext], matcher)
                }
            }
        }
    }
}
```

### Testing

Comprehensive test coverage in `internal/scanner/matchers/content_test.go`:

- Pattern compilation and matching
- Extension-based filtering
- File-specific matching
- Rule validation (requires extensions/files)
- Multiple pattern matching
- Fallback to top-level extensions

Run tests:
```bash
go test ./internal/scanner/matchers/... -v
task test  # Run all tests
```

## Best Practices

### 1. Always Specify Restrictions

**Bad** - No restrictions (would check every file):
```yaml
content:
  - pattern: 'some_pattern'  # WHERE to check?
```

**Good** - Explicit restrictions:
```yaml
content:
  - pattern: 'some_pattern'
    extensions: [.cpp, .h]  # Only check C++ files
```

### 2. Use Specific Patterns

**Bad** - Too broad:
```yaml
content:
  - pattern: 'include'  # Matches too much
```

**Good** - Specific:
```yaml
content:
  - pattern: '#include\s+<afx'  # MFC-specific
```

### 3. Prefer File-Specific Over Extension

**Less efficient** - Check all `.txt` files:
```yaml
content:
  - pattern: 'Qt6::'
    extensions: [.txt]
```

**More efficient** - Check specific file:
```yaml
content:
  - pattern: 'Qt6::'
    files: [CMakeLists.txt]
```

### 4. Use Word Boundaries

**Bad** - Partial matches:
```yaml
content:
  - pattern: 'useState'  # Matches "myuseState"
```

**Good** - Exact matches:
```yaml
content:
  - pattern: '\buseState\b'  # Only "useState"
```

## Debugging

### Enable Rule Tracing

```bash
# See which rules matched and why
./scanner scan --trace-rules /path/to/project

# Output:
│  └─ MATCHED: qt - content matched: Q_OBJECT (in widget.cpp)
│  └─ MATCHED: mfc - content matched: #include\s+<afx (in MainFrame.h)
```

### Test Specific Rules

```bash
# Only check specific technologies
./scanner scan --rules qt,mfc /path/to/project
```

## Future Enhancements

Potential improvements (not yet implemented):

- **Size Limits**: Skip files larger than threshold (e.g., 10MB)
- **Binary Detection**: Skip binary files automatically
- **Parallel Processing**: Process multiple files concurrently
- **Content Sampling**: Check first N lines for performance
- **Negative Patterns**: Exclude technologies based on absence of patterns

## Summary

Content-based detection provides:
- **Independent detection** - Not tied to extensions
- **O(1) performance** - Hash map lookups
- **No false positives** - Precise pattern matching
- **Flexible restrictions** - Extensions or specific files
- **Easy to extend** - Just add YAML rules

This enables accurate detection of technologies that share common file extensions, preventing false positives while maintaining excellent performance.
