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
// cpanParseState holds the mutable state while scanning the DISTRIBUTIONS block.
type cpanParseState struct {
	dists           []cpanDist
	cur             *cpanDist
	section         string // "provides" | "requires" | ""
	inDistributions bool
}

func (st *cpanParseState) flush() {
	if st.cur != nil && st.cur.Name != "" {
		st.dists = append(st.dists, *st.cur)
	}
	st.cur = nil
}

func parseCpanDistributions(content string) []cpanDist {
	st := &cpanParseState{}
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Top-level section headers (not indented).
		if !strings.HasPrefix(raw, " ") {
			st.flush()
			st.inDistributions = trimmed == "DISTRIBUTIONS"
			st.section = ""
			continue
		}
		if st.inDistributions {
			st.applyDistributionLine(raw, trimmed)
		}
	}
	st.flush()
	return st.dists
}

// applyDistributionLine dispatches one indented line within the DISTRIBUTIONS
// block to the distribution header, sub-section header, or module entry.
func (st *cpanParseState) applyDistributionLine(raw, trimmed string) {
	switch indent := countLeadingSpaces(raw); {
	case indent == 2:
		// Distribution header (2-space indent), e.g. "Dist-Name-1.23".
		if m := cpanDistHeaderRe.FindStringSubmatch(trimmed); m != nil {
			st.flush()
			st.cur = &cpanDist{Name: m[1], Version: m[2]}
			st.section = ""
		}
	case st.cur != nil && indent == 4:
		st.section = cpanSubSection(trimmed)
	case st.cur != nil && indent >= 6:
		st.addModule(strings.Fields(trimmed)[0])
	}
}

// cpanSubSection maps a 4-space sub-block header to the active section.
func cpanSubSection(trimmed string) string {
	switch trimmed {
	case "provides:":
		return "provides"
	case "requires:":
		return "requires"
	default:
		return ""
	}
}

// addModule records a module name under the current provides/requires section.
func (st *cpanParseState) addModule(name string) {
	switch st.section {
	case "provides":
		st.cur.Provides = append(st.cur.Provides, name)
	case "requires":
		st.cur.Requires = append(st.cur.Requires, name)
	}
}
