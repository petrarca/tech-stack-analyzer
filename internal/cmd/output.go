package cmd

import (
	"bytes"
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
	OutputToFile(o, format, "")
}

// OutputToFile handles unified output for any Outputter with optional file output
func OutputToFile(o Outputter, format string, outputFile string) {
	var data []byte
	var err error

	switch util.NormalizeFormat(format) {
	case "json":
		data, err = json.MarshalIndent(o.ToJSON(), "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal JSON: %v", err)
		}
	case "yaml":
		data, err = yaml.Marshal(o.ToJSON())
		if err != nil {
			log.Fatalf("Failed to marshal YAML: %v", err)
		}
	default: // text
		if outputFile != "" {
			// For text format to file, we need to capture the text output
			var buf bytes.Buffer
			o.ToText(&buf)
			data = buf.Bytes()
		} else {
			o.ToText(os.Stdout)
			return
		}
	}

	// Write to file or stdout
	if outputFile != "" {
		err = os.WriteFile(outputFile, data, 0644)
		if err != nil {
			log.Fatalf("Failed to write output file: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Results written to %s\n", outputFile)
	} else {
		fmt.Print(string(data))
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

// setupOutputFlags configures both format and output flags for a command
func setupOutputFlags(cmd *cobra.Command, formatPtr *string, outputPtr *string) {
	setupFormatFlag(cmd, formatPtr)
	cmd.Flags().StringVarP(outputPtr, "output", "o", "", "Output file path (default: stdout)")
}
