package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/petrarca/tech-stack-analyzer/internal/progress"
	"github.com/petrarca/tech-stack-analyzer/internal/sbom"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolvestats"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/spf13/cobra"
)

var (
	sbomFormat            string
	sbomOutput            string
	sbomPretty            bool
	sbomDirectOnly        bool
	sbomResolveTransitive bool

	// Dedicated flag vars for the sbom command's transitive resolution. They
	// are separate from the shared scan `settings` struct so cobra does not
	// bind the same global variable to flags on two commands.
	sbomGraphMode       string
	sbomUseDepsDev      bool
	sbomDepsDevEndpoint string
	sbomMavenGraphSrc   string
	sbomMavenRepoURL    string
	sbomMavenCentral    bool
	sbomMavenSettings   string
	sbomMavenLocalRepo  bool
	sbomMavenLocalDir   string
	sbomQuiet           bool
)

// noFileProvider is a types.Provider for off-scan resolution: the original
// source tree is gone, so every file operation fails. Only GetBasePath is used
// (by the online resolvers, as a cache key).
type noFileProvider struct{ base string }

func (noFileProvider) ListDir(string) ([]types.File, error) { return nil, errors.New("no source tree") }
func (noFileProvider) Open(string) (string, error)          { return "", errors.New("no source tree") }
func (noFileProvider) Exists(string) (bool, error)          { return false, nil }
func (noFileProvider) IsDir(string) (bool, error)           { return false, nil }
func (noFileProvider) ReadFile(string) ([]byte, error)      { return nil, errors.New("no source tree") }
func (p noFileProvider) GetBasePath() string                { return p.base }

// sbomCmd re-projects a previously written scan output JSON into an SBOM,
// without re-scanning. The scan's resolved dependencies are already in the
// output file, so this is a pure transformation -- useful to emit additional
// SBOM formats (or refresh one) from a single expensive scan.
var sbomCmd = &cobra.Command{
	Use:   "sbom <scan-output.json>",
	Short: "Generate an SBOM from a previously written scan output JSON",
	Long: `Generate a CycloneDX or SPDX SBOM from a scan output JSON file.

The SBOM is a projection of the scan's resolved dependencies, which are already
present in the scan output. This command reads that output and re-emits it as an
SBOM, so a single (potentially long) scan can produce multiple SBOM formats
without re-scanning.

The input must be a full scan output that still contains the "dependencies"
field (i.e. produced without --omit-fields dependencies and without --aggregate
stripping them).`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return runSBOM(args[0])
	},
}

func runSBOM(inputPath string) error {
	switch strings.ToLower(sbomFormat) {
	case "", "cyclonedx", "spdx":
	default:
		return fmt.Errorf("invalid --format %q: valid values are cyclonedx, spdx", sbomFormat)
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read scan output: %w", err)
	}

	var payload types.Payload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("parse scan output (expected a stack-analyzer scan JSON): %w", err)
	}

	if sbomResolveTransitive {
		if sbomDirectOnly {
			return errors.New("--resolve-transitive and --direct-only are mutually exclusive")
		}
		resolveTransitiveFromCoordinates(&payload)
	}

	out, err := marshalSBOM(&payload, sbomFormat, sbomDirectOnly, sbomPretty)
	if err != nil {
		return fmt.Errorf("generate SBOM: %w", err)
	}

	if sbomOutput == "" {
		_, err = os.Stdout.Write(out)
		return err
	}
	if err := os.WriteFile(sbomOutput, out, 0644); err != nil {
		return fmt.Errorf("write SBOM: %w", err)
	}
	fmt.Fprintf(os.Stderr, "SBOM written to %s\n", sbomOutput)
	return nil
}

// resolveTransitiveFromCoordinates enriches the loaded payload with its
// transitive dependency graph, resolved online from the components' direct
// dependency coordinates (the original source files are gone). It reuses the
// scan's resolution engine and online flags; only coordinate-based resolvers
// apply (deps.dev for public packages, the Maven repo crawl/hybrid for private
// Maven artifacts when a repository is configured). Private non-Maven packages
// that deps.dev cannot resolve simply stay direct.
func resolveTransitiveFromCoordinates(payload *types.Payload) {
	logger := slog.Default()

	// Copy the sbom command's flags into the shared settings struct so the
	// existing scan plumbing (applyMavenSettings) and global setters can be
	// reused verbatim -- no duplicated wiring.
	settings.DependencyGraph = sbomGraphMode
	settings.UseDepsDev = sbomUseDepsDev
	settings.DepsDevEndpoint = sbomDepsDevEndpoint
	settings.MavenGraphSource = sbomMavenGraphSrc
	settings.MavenRepoURL = sbomMavenRepoURL
	settings.UseMavenCentral = sbomMavenCentral
	settings.MavenSettings = sbomMavenSettings
	settings.MavenLocalRepo = sbomMavenLocalRepo
	settings.MavenLocalRepoDir = sbomMavenLocalDir

	mode := sbomGraphMode
	if mode == "" {
		mode = "full"
	}
	components.SetDependencyGraphMode(types.ParseDependencyGraphMode(mode))
	components.SetUseLockFiles(true)
	components.SetUseDepsDev(sbomUseDepsDev)
	components.SetDepsDevEndpoint(sbomDepsDevEndpoint)
	components.SetUseMavenCentral(sbomMavenCentral)
	components.SetMavenGraphSource(sbomMavenGraphSrc)
	applyMavenSettings(logger)

	provider := noFileProvider{base: "sbom:" + payload.Name}

	// Report resolution progress through the same phase events the scan uses,
	// driven by the shared resolvestats counters the resolvers increment.
	stop := startSBOMResolveReporter()
	n := components.ResolvePayloadGraphOnline(payload, provider)
	stop()
	if !sbomQuiet {
		fmt.Fprintf(os.Stderr, "Resolved transitive dependencies for %d component(s)\n", n)
	}
}

// startSBOMResolveReporter reports dependency-resolution progress for the sbom
// command's transitive resolution. Uses the shared startResolveReporter
// goroutine helper.
func startSBOMResolveReporter() func() {
	if sbomQuiet {
		return func() {}
	}
	prog := progress.New(true, progress.NewSimpleHandler(os.Stderr))
	return startResolveReporter(resolveReporterOpts{
		prog:     prog,
		tick:     2 * time.Second,
		isActive: func(s resolvestats.Snapshot) bool { return s.Active() },
		format:   func(s resolvestats.Snapshot) string { return s.Format() },
	})
}

// marshalSBOM builds and marshals an SBOM in the requested format from a
// payload. Shared shape with scan's generateSBOM, but operating on a payload
// loaded from disk rather than the in-memory scan result. When directOnly is
// set, only the project's direct dependencies are emitted, excluding transitive
// graph nodes even when the scan captured a full dependency graph.
func marshalSBOM(p *types.Payload, format string, directOnly, pretty bool) ([]byte, error) {
	if strings.EqualFold(format, "spdx") {
		var doc *sbom.SPDXDocument
		if directOnly {
			doc = sbom.SPDXFromPayloadDirect(p)
		} else {
			doc = sbom.SPDXFromPayload(p)
		}
		sbom.SPDXStamp(doc)
		return marshalJSON(doc, pretty)
	}
	var bom *sbom.BOM
	if directOnly {
		bom = sbom.FromPayloadDirect(p)
	} else {
		bom = sbom.FromPayload(p)
	}
	sbom.Stamp(bom)
	return marshalJSON(bom, pretty)
}

func init() {
	rootCmd.AddCommand(sbomCmd)
	sbomCmd.Flags().StringVar(&sbomFormat, "format", "cyclonedx", "SBOM format: 'cyclonedx' (CycloneDX 1.7 JSON) or 'spdx' (SPDX 2.3 JSON).")
	sbomCmd.Flags().StringVarP(&sbomOutput, "output", "o", "", "Output file path (default: stdout).")
	sbomCmd.Flags().BoolVar(&sbomPretty, "pretty", true, "Pretty-print the JSON output.")
	sbomCmd.Flags().BoolVar(&sbomDirectOnly, "direct-only", false, "Emit only the project's direct dependencies, excluding transitive (dependency-of-dependency) graph nodes. Matches the direct/transitive axis of --dependency-graph; has no effect when the scan captured no transitive graph.")

	// Transitive resolution from coordinates (online). The original source tree
	// is gone, so resolution is coordinate-based: deps.dev for public packages,
	// the Maven repo crawl/hybrid for private Maven artifacts when configured.
	sbomCmd.Flags().BoolVar(&sbomResolveTransitive, "resolve-transitive", false, "Resolve transitive dependencies online from the direct dependency coordinates and fold them into the SBOM. Uses deps.dev (public) and, for Maven/Gradle, the configured Maven repo. Mutually exclusive with --direct-only.")
	sbomCmd.Flags().StringVar(&sbomGraphMode, "dependency-graph", "", "Resolution depth for --resolve-transitive: 'direct' or 'full' (default).")
	sbomCmd.Flags().BoolVar(&sbomUseDepsDev, "deps-dev", false, "Use deps.dev for transitive resolution of public packages (all ecosystems).")
	sbomCmd.Flags().StringVar(&sbomDepsDevEndpoint, "deps-dev-endpoint", "", "Base URL for deps.dev (default: public deps.dev).")
	sbomCmd.Flags().StringVar(&sbomMavenGraphSrc, "maven-graph-source", "", "Maven transitive graph source: 'repo' | 'deps-dev' (hybrid) | 'none'. Default follows --deps-dev.")
	sbomCmd.Flags().StringVar(&sbomMavenRepoURL, "maven-repo-url", "", "Remote Maven repository base (internal Artifactory/JFrog) for transitive POM fetch. Credentials via STACK_ANALYZER_MAVEN_USER/TOKEN.")
	sbomCmd.Flags().BoolVar(&sbomMavenCentral, "maven-central", false, "Enable the public Maven Central source for transitive POM fetch.")
	sbomCmd.Flags().StringVar(&sbomMavenSettings, "maven-settings", "", "Path to a Maven settings.xml for repository URLs and credentials (default: ~/.m2/settings.xml).")
	sbomCmd.Flags().BoolVar(&sbomMavenLocalRepo, "maven-local-repo", false, "Read the local ~/.m2/repository cache for transitive POM resolution.")
	sbomCmd.Flags().StringVar(&sbomMavenLocalDir, "maven-local-repo-dir", "", "Override the local Maven repository path.")
	sbomCmd.Flags().BoolVarP(&sbomQuiet, "quiet", "q", false, "Suppress progress output.")
}
