package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/petrarca/tech-stack-analyzer/internal/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Outputter interface for commands with structured output
type Outputter interface {
	// ToJSON returns the data structure for JSON/YAML marshaling
	ToJSON() interface{}
	// ToText writes human-readable text format
	ToText(w io.Writer)
}

// Output handles unified output for any Outputter
func Output(o Outputter, format string) {
	switch util.NormalizeFormat(format) {
	case "json":
		data, err := json.MarshalIndent(o.ToJSON(), "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(o.ToJSON())
		if err != nil {
			log.Fatalf("Failed to marshal YAML: %v", err)
		}
		fmt.Print(string(data))
	default: // text
		o.ToText(os.Stdout)
	}
}

// setupFormatFlag configures format flag and validation for a command
func setupFormatFlag(cmd *cobra.Command, formatPtr *string) {
	cmd.Flags().StringVarP(formatPtr, "format", "f", "json", "Output format: json, yaml, or text")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		*formatPtr = util.NormalizeFormat(*formatPtr)
		return util.ValidateOutputFormat(*formatPtr)
	}
}
