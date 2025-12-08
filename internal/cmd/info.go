package cmd

import (
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display information about rules, categories, and languages",
	Long:  `Display information about component categories, available technologies, rule details, and supported languages.`,
}

func init() {
	rootCmd.AddCommand(infoCmd)
	infoCmd.AddCommand(techsCmd)
	infoCmd.AddCommand(ruleCmd)
	infoCmd.AddCommand(languagesCmd)
	infoCmd.AddCommand(categoriesCmd)
}
