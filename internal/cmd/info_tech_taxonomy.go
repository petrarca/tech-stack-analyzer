package cmd

import (
	"fmt"
	"io"
	"sort"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/spf13/cobra"
)

var techTaxonomyFormat string
var techTaxonomyOutput string

var techTaxonomyCmd = &cobra.Command{
	Use:   "tech-taxonomy",
	Short: "Show technology taxonomy grouped by categories",
	Long:  `Display technologies grouped by their categories in a hierarchical format.`,
	Run:   runTechTaxonomy,
}

func init() {
	setupOutputFlags(techTaxonomyCmd, &techTaxonomyFormat, &techTaxonomyOutput)
}

// TechTaxonomyCategory represents a category with its technologies
type TechTaxonomyCategory struct {
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	IsComponent  bool             `json:"is_component"`
	Technologies []types.TechInfo `json:"technologies"`
}

// TechTaxonomyResult is the output for the tech-taxonomy command
type TechTaxonomyResult struct {
	Categories []TechTaxonomyCategory `json:"categories"`
	Count      int                    `json:"count"`
}

func (r *TechTaxonomyResult) ToJSON() interface{} {
	return r
}

func (r *TechTaxonomyResult) ToText(w io.Writer) {
	fmt.Fprintf(w, "=== Technology Taxonomy (%d categories) ===\n\n", r.Count)
	for _, category := range r.Categories {
		componentStr := ""
		if category.IsComponent {
			componentStr = " [component]"
		}
		fmt.Fprintf(w, "%s%s (%d technologies)\n", category.Name, componentStr, len(category.Technologies))
		if category.Description != "" {
			fmt.Fprintf(w, "  %s\n", category.Description)
		}
		for _, tech := range category.Technologies {
			fmt.Fprintf(w, "    • %s", tech.Tech)
			if tech.Name != tech.Tech {
				fmt.Fprintf(w, " - %s", tech.Name)
			}
			if tech.Description != "" {
				fmt.Fprintf(w, ": %s", tech.Description)
			}
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w)
	}
}

func (r *TechTaxonomyResult) ToYAML() interface{} {
	return r.ToJSON()
}

func runTechTaxonomy(cmd *cobra.Command, args []string) {
	allRules, categoriesConfig := LoadRulesAndCategories()

	// Group technologies by category
	categoryMap := GroupTechsByCategory(allRules)

	// Build categories with their technologies
	var categories []TechTaxonomyCategory
	for categoryName, categoryDef := range categoriesConfig {
		technologies := categoryMap[categoryName]
		if technologies == nil {
			technologies = []types.TechInfo{}
		} else {
			// Sort technologies by tech name
			SortTechs(technologies)
		}

		category := TechTaxonomyCategory{
			Name:         categoryName,
			Description:  categoryDef.Description,
			IsComponent:  categoryDef.IsComponent,
			Technologies: technologies,
		}
		categories = append(categories, category)
	}

	// Sort categories by name
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	result := &TechTaxonomyResult{
		Categories: categories,
		Count:      len(categories),
	}
	OutputToFile(result, techTaxonomyFormat, techTaxonomyOutput)
}
