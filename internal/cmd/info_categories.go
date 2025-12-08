package cmd

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/petrarca/tech-stack-analyzer/internal/config"
	"github.com/spf13/cobra"
)

var categoriesFormat string
var showComponents bool

var categoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "List all technology categories",
	Long:  `List all technology categories with their descriptions.`,
	Run:   runTypes,
}

func init() {
	setupFormatFlag(categoriesCmd, &categoriesFormat)
	categoriesCmd.Flags().BoolVar(&showComponents, "components", false, "Show only component categories")
}

// CategoryInfo represents a single category entry
type CategoryInfo struct {
	Name        string `json:"name"`
	IsComponent bool   `json:"is_component"`
	Description string `json:"description"`
}

// ComponentsResult is the output for the components flag
type ComponentsResult struct {
	ComponentCategories    []string `json:"component_categories"`
	NonComponentCategories []string `json:"non_component_categories"`
}

// CategoriesResult is the output for the categories command
type CategoriesResult struct {
	Categories []CategoryInfo `json:"categories"`
	Count      int            `json:"count"`
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
	categoriesConfig, err := config.LoadCategoriesConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading categories config: %v\n", err)
		os.Exit(1)
	}

	// If --components flag is used, show component categories only
	if showComponents {
		var componentCategories, nonComponentCategories []string
		for typeName, typeDef := range categoriesConfig.Categories {
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
		Output(result, categoriesFormat)
		return
	}

	// Otherwise show all categories with descriptions
	var categories []CategoryInfo
	for typeName, typeDef := range categoriesConfig.Categories {
		categories = append(categories, CategoryInfo{
			Name:        typeName,
			IsComponent: typeDef.IsComponent,
			Description: typeDef.Description,
		})
	}

	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	result := &CategoriesResult{
		Categories: categories,
		Count:      len(categories),
	}
	Output(result, categoriesFormat)
}
