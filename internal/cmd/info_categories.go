package cmd

import (
	"fmt"
	"io"
	"sort"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/spf13/cobra"
)

var categoriesFormat string
var categoriesOutput string
var showComponents bool

var categoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "List all technology categories",
	Long:  `List all technology categories with their descriptions.`,
	Run:   runTypes,
}

func init() {
	setupOutputFlags(categoriesCmd, &categoriesFormat, &categoriesOutput)
	categoriesCmd.Flags().BoolVar(&showComponents, "components", false, "Show only component categories")
}

// ComponentsResult is the output for the components flag
type ComponentsResult struct {
	ComponentCategories    []string `json:"component_categories"`
	NonComponentCategories []string `json:"non_component_categories"`
}

// CategoriesResult is the output for the categories command
type CategoriesResult struct {
	Categories []types.CategoryInfo `json:"categories"`
	Count      int                  `json:"count"`
}

func (r *CategoriesResult) ToJSON() interface{} {
	return r
}

func (r *ComponentsResult) ToJSON() interface{} {
	return r
}

func (r *ComponentsResult) ToText(w io.Writer) {
	fmt.Fprintln(w, "=== Component Categories (create components) ===")
	for _, t := range r.ComponentCategories {
		fmt.Fprintln(w, t)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "=== Non-Component Categories (tools/libraries only) ===")
	for _, t := range r.NonComponentCategories {
		fmt.Fprintln(w, t)
	}
}

func (r *CategoriesResult) ToText(w io.Writer) {
	fmt.Fprintf(w, "=== Technology Categories (%d) ===\n\n", r.Count)
	for _, t := range r.Categories {
		componentStr := ""
		if t.IsComponent {
			componentStr = " [component]"
		}
		fmt.Fprintf(w, "%-20s%s\n", t.Name, componentStr)
		if t.Description != "" {
			fmt.Fprintf(w, "  %s\n", t.Description)
		}
	}
}

func runTypes(cmd *cobra.Command, args []string) {
	_, categoriesConfig := LoadRulesAndCategories()

	// If --components flag is used, show component categories only
	if showComponents {
		var componentCategories, nonComponentCategories []string
		for typeName, typeDef := range categoriesConfig {
			if typeDef.IsComponent {
				componentCategories = append(componentCategories, typeName)
			} else {
				nonComponentCategories = append(nonComponentCategories, typeName)
			}
		}

		sort.Strings(componentCategories)
		sort.Strings(nonComponentCategories)

		result := &ComponentsResult{
			ComponentCategories:    componentCategories,
			NonComponentCategories: nonComponentCategories,
		}
		OutputToFile(result, categoriesFormat, categoriesOutput)
		return
	}

	// Otherwise show all categories with descriptions
	var categories []types.CategoryInfo
	for typeName, typeDef := range categoriesConfig {
		categories = append(categories, types.CategoryInfo{
			Name:        typeName,
			IsComponent: typeDef.IsComponent,
			Description: typeDef.Description,
		})
	}

	SortCategories(categories)

	result := &CategoriesResult{
		Categories: categories,
		Count:      len(categories),
	}
	OutputToFile(result, categoriesFormat, categoriesOutput)
}
