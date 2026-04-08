package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/config"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/spf13/cobra"
)

var ecosystemsFormat string
var ecosystemsOutput string

var ecosystemsCmd = &cobra.Command{
	Use:   "ecosystems",
	Short: "List all technology ecosystem definitions",
	Long:  `Display all technology ecosystem definitions with their detection signals (component types, techs, languages).`,
	Run:   runEcosystems,
}

func init() {
	setupOutputFlags(ecosystemsCmd, &ecosystemsFormat, &ecosystemsOutput)
}

// EcosystemsResult is the output for the ecosystems command
type EcosystemsResult struct {
	Ecosystems []types.EcosystemInfo `json:"ecosystems"`
	Count      int                   `json:"count"`
}

func (r *EcosystemsResult) ToJSON() interface{} {
	return r
}

func (r *EcosystemsResult) ToText(w io.Writer) {
	fmt.Fprintf(w, "=== Technology Ecosystems (%d) ===\n\n", r.Count)
	for _, eco := range r.Ecosystems {
		fmt.Fprintf(w, "%s\n", eco.Name)
		if eco.Description != "" {
			fmt.Fprintf(w, "  %s\n", eco.Description)
		}
		if len(eco.ComponentTypes) > 0 {
			fmt.Fprintf(w, "  component_types: %s\n", strings.Join(eco.ComponentTypes, ", "))
		}
		if len(eco.Techs) > 0 {
			fmt.Fprintf(w, "  techs:           %s\n", strings.Join(eco.Techs, ", "))
		}
		if len(eco.Languages) > 0 {
			fmt.Fprintf(w, "  languages:       %s\n", strings.Join(eco.Languages, ", "))
		}
		fmt.Fprintln(w)
	}
}

func (r *EcosystemsResult) ToYAML() interface{} {
	return r.ToJSON()
}

func runEcosystems(cmd *cobra.Command, args []string) {
	ecosystemsCfg, err := config.LoadEcosystemsConfig()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to load ecosystems config: %v\n", err)
		return
	}

	var ecosystems []types.EcosystemInfo
	for _, eco := range ecosystemsCfg.Ecosystems {
		ecosystems = append(ecosystems, types.EcosystemInfo(eco))
	}

	result := &EcosystemsResult{
		Ecosystems: ecosystems,
		Count:      len(ecosystems),
	}
	OutputToFile(result, ecosystemsFormat, ecosystemsOutput)
}
