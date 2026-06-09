package parsers

import (
	"bufio"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// CpanfileSnapshotParser parses Carton's cpanfile.snapshot (the resolved Perl
// dependency lockfile). It is a line-based, indentation-structured format:
//
//	DISTRIBUTIONS
//	  Dist-Name-1.23
//	    pathname: A/AU/AUTHOR/Dist-Name-1.23.tar.gz
//	    provides:
//	      Module::Name  1.23
//	    requires:
//	      Other::Module 0
type CpanfileSnapshotParser struct{}

// NewCpanfileSnapshotParser creates a new cpanfile.snapshot parser.
func NewCpanfileSnapshotParser() *CpanfileSnapshotParser {
	return &CpanfileSnapshotParser{}
}

// cpanDist is a resolved distribution: its name@version, the modules it
// provides, and the modules it requires.
type cpanDist struct {
	Name     string // distribution name without version, e.g. "Dist-Name"
	Version  string
	Provides []string // module names this distribution provides
	Requires []string // module names this distribution requires
}

// cpanDistHeaderRe matches a distribution header line "Dist-Name-1.23": a
// non-indented token ending in "-<version starting with a digit>".
var cpanDistHeaderRe = regexp.MustCompile(`^([A-Za-z0-9_.+-]+)-(v?[0-9][\w.]*)$`)

// ParseCpanfileSnapshot parses cpanfile.snapshot into resolved dependencies
// (one per distribution).
func (p *CpanfileSnapshotParser) ParseCpanfileSnapshot(content string) []types.Dependency {
	var deps []types.Dependency
	for _, d := range parseCpanDistributions(content) {
		if d.Version == "" {
			continue
		}
		deps = append(deps, types.Dependency{
			Type:       DependencyTypePerl,
			Name:       d.Name,
			Version:    d.Version,
			SourceFile: "cpanfile.snapshot",
		})
	}
	return deps
}

// parseCpanDistributions extracts every distribution block with its provides
// and requires module lists.
func parseCpanDistributions(content string) []cpanDist {
	var dists []cpanDist
	var cur *cpanDist
	section := "" // "provides" | "requires" | ""

	flush := func() {
		if cur != nil && cur.Name != "" {
			dists = append(dists, *cur)
		}
		cur = nil
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	inDistributions := false
	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Top-level section headers (not indented).
		if !strings.HasPrefix(raw, " ") {
			flush()
			inDistributions = trimmed == "DISTRIBUTIONS"
			section = ""
			continue
		}
		if !inDistributions {
			continue
		}

		indent := countLeadingSpaces(raw)
		switch {
		case indent == 2:
			// Distribution header (2-space indent), e.g. "Dist-Name-1.23".
			if m := cpanDistHeaderRe.FindStringSubmatch(trimmed); m != nil {
				flush()
				cur = &cpanDist{Name: m[1], Version: m[2]}
				section = ""
			}
		case cur != nil && indent == 4:
			switch {
			case trimmed == "provides:":
				section = "provides"
			case trimmed == "requires:":
				section = "requires"
			case strings.HasPrefix(trimmed, "pathname:"):
				section = ""
			default:
				section = ""
			}
		case cur != nil && indent >= 6:
			// "Module::Name  version" under provides/requires.
			name := strings.Fields(trimmed)[0]
			switch section {
			case "provides":
				cur.Provides = append(cur.Provides, name)
			case "requires":
				cur.Requires = append(cur.Requires, name)
			}
		}
	}
	flush()
	return dists
}
