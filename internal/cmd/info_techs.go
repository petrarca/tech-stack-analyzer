package cmd

import (
	"fmt"
	"io"
	"sort"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/spf13/cobra"
)

var techsFormat string
var techsOutput string

var techsCmd = &cobra.Command{
	Use:   "techs",
	Short: "List all available technologies",
	Long:  `List all technology names from the embedded rules.`,
	Run:   runTechs,
}

func init() {
	setupOutputFlags(techsCmd, &techsFormat, &techsOutput)
}

// TechsResult is the output for the techs command
type TechsResult struct {
	Technologies []types.TechInfo `json:"technologies"`
}

func (r *TechsResult) ToJSON() interface{} {
	return r
}

func (r *TechsResult) ToText(w io.Writer) {
	for _, tech := range r.Technologies {
		fmt.Fprintf(w, "%s (%s)\n", tech.Tech, tech.Category)
	}
	fmt.Fprintf(w, "\nTotal: %d technologies\n", len(r.Technologies))
}

func runTechs(cmd *cobra.Command, args []string) {
	allRules, _ := LoadRulesAndCategories()

	// Create rule map for easy lookup
	ruleMap := make(map[string]*types.Rule)
	for i := range allRules {
		ruleMap[allRules[i].Tech] = &allRules[i]
	}

	// Get sorted tech keys
	techKeys := make([]string, 0, len(ruleMap))
	for tech := range ruleMap {
		techKeys = append(techKeys, tech)
	}
	sort.Strings(techKeys)

	// Build technologies list
	technologies := make([]types.TechInfo, 0, len(techKeys))
	for _, techKey := range techKeys {
		rule := ruleMap[techKey]
		info := types.TechInfo{
			Name:     rule.Name,
			Tech:     techKey,
			Category: rule.Type,
		}
		if rule.Description != "" {
			info.Description = rule.Description
		}
		if len(rule.Properties) > 0 {
			info.Properties = rule.Properties
		}
		technologies = append(technologies, info)
	}

	result := &TechsResult{Technologies: technologies}
	OutputToFile(result, techsFormat, techsOutput)
}
