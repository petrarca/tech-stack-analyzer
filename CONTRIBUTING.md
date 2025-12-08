# Contributing to Tech Stack Analyzer

Thank you for your interest in contributing to the Tech Stack Analyzer! This document provides guidelines and information to help you get started.

## 🚀 Quick Start

### Prerequisites

- **Go 1.19+** - Required for development
- **Task** - Task automation tool (recommended)
- **Git** - Version control
- **Docker** - For containerized testing (optional)

### Initial Setup

1. **Fork the repository**
   ```bash
   # Fork on GitHub, then clone your fork
   git clone https://github.com/YOUR_USERNAME/tech-stack-analyzer.git
   cd tech-stack-analyzer
   ```

2. **Add upstream remote**
   ```bash
   git remote add upstream https://github.com/original-org/tech-stack-analyzer.git
   ```

3. **Install development tools**
   ```bash
   # Install Task for automation
   go install github.com/go-task/task/v3/cmd/task@latest
   
   # Install development dependencies
   task setup
   ```

4. **Verify setup**
   ```bash
   task fct  # Format, Check, Test
   ```

## 🏗️ Development Workflow

### 1. Create a Feature Branch

```bash
# Sync with upstream
git fetch upstream
git checkout main
git merge upstream/main

# Create feature branch
git checkout -b feature/your-feature-name
```

### 2. Make Changes

```bash
# Make your changes
# ... (edit files)

# Run quality checks
task fct

# Run specific tests
go test -v ./internal/scanner/...

# Run benchmarks
go test -bench=. ./internal/scanner/...
```

### 3. Test Your Changes

```bash
# Build the project
task build

# Test against sample projects
./bin/scanner /path/to/test/project

# Run integration tests
task test-integration
```

### 4. Submit Changes

```bash
# Commit your changes
git add .
git commit -m "feat: add new technology detector"

# Push to your fork
git push origin feature/your-feature-name

# Create pull request
```

## 📋 Contribution Types

### 🐛 Bug Fixes

1. **Create an issue** describing the bug with:
   - Clear reproduction steps
   - Expected vs actual behavior
   - Environment details (OS, Go version, etc.)

2. **Add tests** that reproduce the bug
3. **Fix the bug** ensuring all tests pass
4. **Update documentation** if behavior changes

### ✨ New Features

1. **Discuss in an issue** before implementing
2. **Design the feature** considering:
   - Performance impact
   - Backward compatibility
   - User experience

3. **Implement with tests**:
   - Unit tests for core logic
   - Integration tests for end-to-end behavior
   - Benchmarks for performance-critical code

### 🔧 Technology Rules

Adding support for new technologies is highly encouraged!

#### Rule Structure

```yaml
# internal/rules/core/category/technology.yaml
tech: technology-id
name: Human Readable Name
type: category
dotenv:
  - ENV_PREFIX_
dependencies:
  - type: npm
    name: package-name
    example: package-name
  - type: python
    name: python-package
    example: python-package
files:
  - config-file.ext
detect:
  type: detection-type
  file: "*.pattern"
```

#### Rule Categories

```
internal/rules/core/
├── ai/                   # AI/ML frameworks and services
├── analytics/            # Analytics and monitoring platforms
├── application/          # Application frameworks
├── cloud/                # Cloud providers and services
├── database/             # Database systems and clients
├── hosting/              # Hosting and PaaS services
├── queue/                # Message queues and streaming
├── security/             # Security and authentication tools
└── tool/                 # Development and DevOps tools
```

#### Adding Rules

1. **Choose appropriate category** or create new one
2. **Create rule file** following naming convention
3. **Add comprehensive dependencies** for all supported languages
4. **Test against real projects** using the technology
5. **Update documentation** if adding new category

### 🧩 Component Detectors

Component detectors analyze project-specific files and configurations.

#### Detector Structure

```go
// internal/scanner/components/technology/detector.go
package technology

import (
    "tech-stack-analyzer/internal/scanner/components"
    "tech-stack-analyzer/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string {
    return "technology"
}

func (d *Detector) Detect(files []types.File, providers ...types.Provider) ([]*types.Payload, error) {
    // Implementation
}

func init() {
    components.Register(&Detector{})
}
```

#### Adding Detectors

1. **Create detector package** in `internal/scanner/components/`
2. **Implement Detector interface**
3. **Add comprehensive tests**
4. **Create parser if needed** in `internal/scanner/parsers/`
5. **Register in scanner** by importing in `internal/scanner/scanner.go`

## 📏 Code Style Guidelines

### Go Code Style

- **Formatting**: Use `gofmt` and `goimports`
- **Linting**: Must pass `golangci-lint` with no issues
- **Naming**: Follow Go conventions (CamelCase for exported, camelCase for unexported)
- **Comments**: Add godoc comments for all public functions and types
- **Error Handling**: Always handle errors explicitly

### Example Code Style

```go
// Package scanner provides technology stack analysis capabilities.
package scanner

import (
    "context"
    "fmt"
)

// Scanner analyzes file systems to detect technologies.
type Scanner struct {
    rules    *rules.Rules
    providers []types.Provider
}

// NewScanner creates a new scanner instance with the given options.
func NewScanner(opts ...Option) (*Scanner, error) {
    // Implementation
}

// Scan analyzes the given path and returns detected technologies.
func (s *Scanner) Scan(ctx context.Context, path string) (*types.Result, error) {
    // Implementation with proper error handling
    if path == "" {
        return nil, fmt.Errorf("path cannot be empty")
    }
    // ... rest of implementation
}
```

### Testing Guidelines

- **Coverage**: Maintain >90% test coverage
- **Table-driven tests**: Use for multiple test cases
- **Mock interfaces**: Use for external dependencies
- **Benchmark tests**: Add for performance-critical functions

#### Example Test

```go
func TestScanner_Scan(t *testing.T) {
    tests := []struct {
        name    string
        path    string
        want    *types.Result
        wantErr bool
    }{
        {
            name: "valid Node.js project",
            path: "testdata/nodejs",
            want: &types.Result{
                Tech:  "nodejs",
                Techs: []string{"nodejs", "express"},
            },
            wantErr: false,
        },
        {
            name:    "empty path",
            path:    "",
            want:    nil,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            s := NewScanner()
            got, err := s.Scan(context.Background(), tt.path)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("Scanner.Scan() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("Scanner.Scan() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## 📝 Commit Message Guidelines

Use conventional commit format:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Types

- `feat`: New feature or enhancement
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring without behavior change
- `perf`: Performance improvement
- `test`: Test additions or changes
- `chore`: Maintenance tasks, dependency updates

### Examples

```
feat: add Oracle database component detector

Add comprehensive Oracle database detection supporting:
- OCI drivers for multiple languages
- Configuration file parsing
- Environment variable detection

Fixes #123

perf: optimize file matching algorithm

Improve extension matching from O(n*m) to O(n) using
pre-computed lookup tables. Results in 10x performance
improvement for large repositories.
```

## 🧪 Testing Strategy

### Test Categories

1. **Unit Tests**: Test individual functions and methods
2. **Integration Tests**: Test component interactions
3. **End-to-End Tests**: Test complete scanning workflows
4. **Performance Tests**: Benchmark critical paths
5. **Rule Tests**: Validate technology detection rules

### Running Tests

```bash
# Run all tests
task test

# Run with coverage
task test-coverage

# Run benchmarks
task benchmark

# Run integration tests
task test-integration

# Run specific test package
go test -v ./internal/scanner/...
```

### Test Data

Use structured test data in `testdata/` directories:

```
testdata/
├── nodejs/
│   ├── package.json
│   └── src/
├── dotnet/
│   ├── project.csproj
│   └── Program.cs
└── python/
    ├── pyproject.toml
    └── requirements.txt
```

## 🚀 Performance Guidelines

### Performance Considerations

- **Memory Usage**: Avoid unnecessary allocations
- **Concurrency**: Use goroutines for I/O-bound operations
- **Caching**: Cache expensive computations
- **Profiling**: Use `pprof` for performance analysis

### Benchmarking

Add benchmarks for performance-critical functions:

```go
func BenchmarkScanner_Scan(b *testing.B) {
    s := NewScanner()
    path := "testdata/large-project"
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = s.Scan(context.Background(), path)
    }
}
```

### Performance Testing

```bash
# Run benchmarks
go test -bench=. -benchmem ./...

# Profile memory usage
go test -memprofile=mem.prof -bench=. ./...

# Profile CPU usage
go test -cpuprofile=cpu.prof -bench=. ./...
```

## 📋 Pull Request Process

### Before Submitting

1. **Run all checks**
   ```bash
   task fct  # Format, Check, Test
   ```

2. **Update documentation**
   - README.md for user-facing changes
   - Code comments for API changes
   - Architecture docs for structural changes

3. **Add tests** for new functionality
4. **Update CHANGELOG.md** for significant changes

### Pull Request Template

```markdown
## Description
Brief description of changes and motivation.

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing completed

## Performance Impact
- [ ] No performance impact
- [ ] Performance improvement (describe)
- [ ] Performance regression (explain)

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] Tests added/updated
- [ ] All tests pass
- [ ] Ready for review
```

### Review Process

1. **Automated checks** must pass
2. **Code review** by maintainers
3. **Testing review** for coverage and quality
4. **Performance review** for performance changes
5. **Documentation review** for user-facing changes

## 🐛 Issue Reporting

### Bug Reports

Use the bug report template with:

- **Clear title** describing the issue
- **Reproduction steps** with minimal example
- **Expected vs actual behavior**
- **Environment details** (OS, Go version, etc.)
- **Additional context** (logs, screenshots, etc.)

### Feature Requests

- **Use case** and problem description
- **Proposed solution** (if any)
- **Alternative approaches** considered
- **Additional context** and requirements

## 🤝 Community Guidelines

### Code of Conduct

- Be respectful and inclusive
- Welcome newcomers and help them learn
- Focus on constructive feedback
- Assume good intentions

### Getting Help

- **GitHub Issues**: For bug reports and feature requests
- **Discussions**: For questions and ideas
- **Documentation**: Check existing docs first

### Recognition

Contributors are recognized in:
- README.md contributors section
- Release notes for significant contributions
- Annual contributor highlights

## 📚 Resources

### Development Tools

- **[Task](https://taskfile.dev/)**: Task runner
- **[golangci-lint](https://golangci-lint.run/)**: Go linter
- **[pprof](https://pkg.go.dev/net/http/pprof)**: Go profiler
- **[goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports)**: Import management

### Learning Resources

- **[Effective Go](https://go.dev/doc/effective_go)**: Go best practices
- **[Go Testing](https://go.dev/doc/testing)**: Testing in Go
- **[Go Performance](https://go.dev/doc/diagnostics)**: Performance diagnostics

### Project Documentation

- **[Architecture Overview](docs/architecture.md)**: System design
- **[Rule Development](docs/rule-development.md)**: Writing technology rules
- **[Component Detectors](docs/component-detectors.md)**: Adding detectors

---

Thank you for contributing to the Tech Stack Analyzer! Your contributions help make technology stack analysis faster and more comprehensive for everyone. 🚀
