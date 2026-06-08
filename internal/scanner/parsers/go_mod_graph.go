package parsers

import (
	"bufio"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// GoModGraphFileName is the pre-generated `go mod graph` output. A full Go
// transitive graph is not stated by go.mod alone (go.mod lists requirements,
// not the resolved module graph), so -- like Maven's dependency-tree.json --
// the operator/CI generates this file and the analyzer reads it without ever
// running go. go.mod alone still yields the direct dependencies.
//
//	go mod graph > go.mod.graph
const GoModGraphFileName = "go.mod.graph"

// ParseGoModGraph parses the pre-generated `go mod graph` output and returns the
// module-to-module edges. It implements the GraphProducer contract.
//
// Each line is "from to" where from is "module@version" (or the bare root
// module with no version) and to is "module@version". For direct mode, only
// the root module's edges are emitted (from the synthetic "." node).
func ParseGoModGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	scanner := bufio.NewScanner(strings.NewReader(string(input.Lockfile)))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	seen := make(map[string]bool)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		from, to := fields[0], fields[1]
		// The root module is the only "from" without an @version.
		isRoot := !strings.Contains(from, "@")

		switch input.Mode {
		case types.DependencyGraphDirect:
			if isRoot {
				addGoEdge(&result.Edges, seen, ".", to)
			}
		case types.DependencyGraphFull:
			if isRoot {
				addGoEdge(&result.Edges, seen, ".", to)
			} else {
				addGoEdge(&result.Edges, seen, from, to)
			}
		}
	}
	return result
}

// ParseGoModDirectGraph derives direct-dependency edges from go.mod itself (the
// require block, excluding // indirect). It is the fallback when no pre-generated
// go mod graph file is present; it can only produce the direct graph.
func ParseGoModDirectGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}
	// Only direct edges are derivable from go.mod; full transitive requires the
	// go mod graph file.
	for _, mod := range goModRequires(string(input.Lockfile)) {
		result.Edges = append(result.Edges, types.DependencyEdge{From: ".", To: mod, Scope: types.ScopeProd})
	}
	return result
}

// goModRequires extracts "module@version" for each direct (non-indirect)
// requirement in go.mod, handling both the single-line and block require forms.
func goModRequires(content string) []string {
	var mods []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	inBlock := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "require (":
			inBlock = true
			continue
		case inBlock && line == ")":
			inBlock = false
			continue
		}
		var spec string
		if inBlock {
			spec = line
		} else if strings.HasPrefix(line, "require ") {
			spec = strings.TrimPrefix(line, "require ")
		} else {
			continue
		}
		if spec == "" || strings.HasPrefix(spec, "//") {
			continue
		}
		// Skip indirect requirements (transitive, not direct).
		if strings.Contains(spec, "// indirect") {
			continue
		}
		// Strip trailing comments.
		if i := strings.Index(spec, "//"); i >= 0 {
			spec = strings.TrimSpace(spec[:i])
		}
		fields := strings.Fields(spec)
		if len(fields) >= 2 {
			mods = append(mods, fields[0]+"@"+fields[1])
		}
	}
	return mods
}

// addGoEdge appends an edge once (deduped).
func addGoEdge(edges *[]types.DependencyEdge, seen map[string]bool, from, to string) {
	key := from + "|" + to
	if seen[key] {
		return
	}
	seen[key] = true
	*edges = append(*edges, types.DependencyEdge{From: from, To: to})
}
