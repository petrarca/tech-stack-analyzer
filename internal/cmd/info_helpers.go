package cmd

import (
	"log"
	"sort"

	"github.com/petrarca/tech-stack-analyzer/internal/config"
	"github.com/petrarca/tech-stack-analyzer/internal/rules"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// LoadRulesAndCategories loads both rules and categories config
func LoadRulesAndCategories() ([]types.Rule, map[string]types.CategoryDefinition) {
	// Load categories config
	categoriesConfig, err := config.LoadCategoriesConfig()
	if err != nil {
		log.Fatalf("Failed to load categories config: %v", err)
	}

	// Load all rules
	allRules, err := rules.LoadEmbeddedRules()
	if err != nil {
		log.Fatalf("Failed to load rules: %v", err)
	}

	return allRules, categoriesConfig.Categories
}

// GroupTechsByCategory groups technologies by their categories
func GroupTechsByCategory(allRules []types.Rule) map[string][]types.TechInfo {
	categoryMap := make(map[string][]types.TechInfo)
	for _, rule := range allRules {
		techInfo := types.TechInfo{
			Name:        rule.Name,
			Tech:        rule.Tech,
			Category:    rule.Type,
			Description: rule.Description,
		}
		if len(rule.Properties) > 0 {
			techInfo.Properties = rule.Properties
		}
		categoryMap[rule.Type] = append(categoryMap[rule.Type], techInfo)
	}
	return categoryMap
}

// SortCategories sorts categories by name
func SortCategories(categories []types.CategoryInfo) {
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})
}

// SortTechs sorts technologies by tech name
func SortTechs(technologies []types.TechInfo) {
	sort.Slice(technologies, func(i, j int) bool {
		return technologies[i].Tech < technologies[j].Tech
	})
}
