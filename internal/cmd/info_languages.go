package cmd

import (
	"fmt"
	"io"
	"sort"

	"github.com/go-enry/go-enry/v2"
	"github.com/go-enry/go-enry/v2/data"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/spf13/cobra"
)

var languagesFormat string
var languagesOutput string

var languagesCmd = &cobra.Command{
	Use:   "languages",
	Short: "List all languages known to go-enry",
	Long:  `List all programming languages, data formats, markup, and prose languages from go-enry (GitHub Linguist).`,
	Run:   runLanguages,
}

func init() {
	setupOutputFlags(languagesCmd, &languagesFormat, &languagesOutput)
}

// LanguageInfo holds information about a language from go-enry
type LanguageInfo struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Extensions []string `json:"extensions"`
}

// LanguagesSummary holds summary statistics
type LanguagesSummary struct {
	Total  int            `json:"total"`
	ByType map[string]int `json:"by_type"`
}

// LanguagesResult is the output for the languages command
type LanguagesResult struct {
	Languages []LanguageInfo   `json:"languages"`
	Summary   LanguagesSummary `json:"summary"`
}

func (r *LanguagesResult) ToJSON() interface{} {
	return r
}

func (r *LanguagesResult) ToText(w io.Writer) {
	for _, lang := range r.Languages {
		fmt.Fprintf(w, "%-30s %-12s %v\n", lang.Name, lang.Type, lang.Extensions)
	}
	fmt.Fprintf(w, "\nTotal: %d languages\n", r.Summary.Total)
	fmt.Fprintf(w, "By type: programming=%d, data=%d, markup=%d, prose=%d\n",
		r.Summary.ByType["programming"], r.Summary.ByType["data"],
		r.Summary.ByType["markup"], r.Summary.ByType["prose"])
}

func runLanguages(cmd *cobra.Command, args []string) {
	result := buildLanguagesResult()
	OutputToFile(result, languagesFormat, languagesOutput)
}

func buildLanguagesResult() *LanguagesResult {
	// Get all languages from go-enry's data
	langNames := data.LanguagesByExtension

	// Build unique language set
	langSet := make(map[string]bool)
	for _, langs := range langNames {
		for _, lang := range langs {
			langSet[lang] = true
		}
	}

	// Build language info list
	languages := make([]LanguageInfo, 0, len(langSet))
	byType := make(map[string]int)

	for lang := range langSet {
		langType := enry.GetLanguageType(lang)
		typeName := types.LanguageTypeToString(langType)

		extensions := getExtensionsForLanguage(lang)

		languages = append(languages, LanguageInfo{
			Name:       lang,
			Type:       typeName,
			Extensions: extensions,
		})

		byType[typeName]++
	}

	// Sort by name
	sort.Slice(languages, func(i, j int) bool {
		return languages[i].Name < languages[j].Name
	})

	return &LanguagesResult{
		Languages: languages,
		Summary: LanguagesSummary{
			Total:  len(languages),
			ByType: byType,
		},
	}
}

// getExtensionsForLanguage returns file extensions for a language
func getExtensionsForLanguage(lang string) []string {
	var extensions []string
	for ext, langs := range data.LanguagesByExtension {
		for _, l := range langs {
			if l == lang {
				extensions = append(extensions, ext)
				break
			}
		}
	}
	sort.Strings(extensions)
	return extensions
}
