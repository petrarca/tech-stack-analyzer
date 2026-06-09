# Contributing to Tech Stack Analyzer

## Development Workflow

### Prerequisites

- **Go 1.25+**
- [Task](https://taskfile.dev/) -- task runner
- [golangci-lint](https://golangci-lint.run/) -- linter (installed automatically by pre-commit)

### Setup

```bash
git clone https://github.com/petrarca/tech-stack-analyzer.git
cd tech-stack-analyzer
task build                # compile binary to bin/stack-analyzer
task pre-commit:install   # install pre-commit hooks
```

### Daily workflow

```bash
task fct                  # format + check (vet + lint) + test
task build                # compile binary
task run -- summary /path # run from source
```

### Commit style

Use conventional commit prefixes: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`.

### Testing

```bash
task test          # full offline test suite
task fct           # format + check + test -- run before committing
task test:online   # opt-in live network tests (deps.dev); requires internet
```

The default suite is **fully offline**. The network-dependent live deps.dev test is build-tag gated (`//go:build online`) and excluded from `task test`; run it explicitly with `task test:online`.

**Dependency-graph tests** run at three offline layers: per-parser unit tests, real-lockfile fixture tests (`internal/scanner/parsers/testdata/lockfiles/`), and end-to-end scanner tests. See the [testing strategy](docs/design/detector-implementation.md#testing-strategy) for details.

> **Fixtures policy:** lockfile fixtures must contain **only public open-source packages** (serde, express, requests, sinatra, monolog, flask, plug, ...). Generate them fresh from public packages with the real package managers -- never copy a lockfile from an internal or proprietary repository. No internal package names, registry URLs, or filesystem paths may appear in a committed fixture.

---

## Pre-commit Hooks

| Hook | Stage | What it checks |
|------|-------|----------------|
| `go-pre-commit` | pre-commit | `gofmt` + `go vet` + `golangci-lint` + `go test` |
| `go-pre-push` | pre-push | same + `-race` detector |

Install with `task pre-commit:install`. Run manually with `task pre-commit:run`.

---

## Adding Technology Rules

Rules live in `internal/rules/techs/<category>/`. Each rule is a YAML file:

```yaml
tech: technology-id           # unique identifier (lowercase, hyphens)
name: Human Readable Name
type: category                # e.g. database, application, tool
dependencies:
  - type: npm
    name: package-name
files:
  - config-file.ext
content:
  - pattern: 'regex pattern'
    extensions: [.js, .ts]
```

After adding a rule, run `task fct` to verify.

## Adding Component Detectors

Detectors live in `internal/scanner/components/<name>/`. Create a package
with a `Detector` struct implementing the `components.ComponentDetector`
interface and register it via `init()`:

```go
func init() {
    components.Register(&Detector{})
}
```

Import the package in `internal/scanner/scanner.go` to trigger registration.

---

## Releasing

Uses [GoReleaser](https://goreleaser.com/) triggered by Git tags. Creating a
tag on `main` starts the release workflow.

### Version numbering

Follow [Semantic Versioning](https://semver.org/):

| Change | Bump | Example |
|--------|------|---------|
| Bug fixes, minor improvements | Patch | `v0.4.2` -> `v0.4.3` |
| New features, backward-compatible | Minor | `v0.4.3` -> `v0.5.0` |
| Breaking changes to output schema or CLI | Major | `v0.5.0` -> `v1.0.0` |

### Steps

1. **Ensure `main` is clean and all checks pass**

   ```bash
   git checkout main && git pull
   task fct build
   ```

2. **Check what has changed since the last release**

   ```bash
   git log --oneline $(git describe --tags --abbrev=0)..HEAD
   ```

3. **Create and push the tag**

   ```bash
   git tag v0.x.y
   git push origin v0.x.y
   ```

4. **Update the GitHub release notes**

   ```bash
   gh release edit v0.x.y --notes "..."
   ```

### Deleting a bad tag

```bash
git tag -d v0.x.y
git push origin :refs/tags/v0.x.y
# fix, then re-tag
git tag v0.x.y
git push origin v0.x.y
```
