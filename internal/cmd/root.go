package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "stack-analyzer",
	Short: "Technology stack analyzer for codebases",
	Long: `Stack Analyzer detects technologies, frameworks, databases, and services
used in your codebase by analyzing configuration files, dependencies, and code patterns.

It provides comprehensive analysis with 700+ technology rules covering databases,
frameworks, tools, cloud services, and more.`,
	Version: "1.0.0",
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here if needed
}
