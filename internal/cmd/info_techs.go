package cmd

import (
	"fmt"
	"io"
	"log"
	"sort"

	"github.com/petrarca/tech-stack-analyzer/internal/rules"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/spf13/cobra"
)

var techsFormat string

var techsCmd = &cobra.Command{
	Use:   "techs",
	Short: "List all available technologies",
	Long:  `List all technology names from the embedded rules.`,
	Run:   runTechs,
}

func init() {
	setupFormatFlag(techsCmd, &techsFormat)
}

// TechInfo holds information about a technology
type TechInfo struct {
	Name        string                 `json:"name"`
	Tech        string                 `json:"tech"`
	Category    string                 `json:"category"`
	Description string                 `json:"description,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

// TechsResult is the output for the techs command
type TechsResult struct {
	Technologies []TechInfo `json:"technologies"`
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
	allRules, err := rules.LoadEmbeddedRules()
	if err != nil {
		log.Fatalf("Failed to load rules: %v", err)
	}

	ruleMap := make(map[string]*types.Rule)
	for i := range allRules {
		ruleMap[allRules[i].Tech] = &allRules[i]
	}

	techKeys := make([]string, 0, len(ruleMap))
	for tech := range ruleMap {
		techKeys = append(techKeys, tech)
	}
	sort.Strings(techKeys)

	technologies := make([]TechInfo, 0, len(techKeys))
	for _, techKey := range techKeys {
		rule := ruleMap[techKey]
		info := TechInfo{
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
	Output(result, techsFormat)
}
