package main

import (
	"fmt"
	"time"

	"github.com/petrarca/tech-stack-analyzer/internal/config"
	"github.com/petrarca/tech-stack-analyzer/internal/license"
	"github.com/petrarca/tech-stack-analyzer/internal/rules"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/matchers"
)

func main() {
	start := time.Now()

	t1 := time.Now()
	loadedRules, err := rules.LoadEmbeddedRules()
	if err != nil {
		panic(err)
	}
	fmt.Printf("LoadEmbeddedRules: %v (%d rules)\n", time.Since(t1), len(loadedRules))

	t2 := time.Now()
	_, err = config.LoadCategoriesConfig()
	if err != nil {
		panic(err)
	}
	fmt.Printf("LoadCategoriesConfig: %v\n", time.Since(t2))

	t3 := time.Now()
	matchers.BuildFileMatchersFromRules(loadedRules)
	fmt.Printf("BuildFileMatchersFromRules: %v\n", time.Since(t3))

	t4 := time.Now()
	contentMatcher := matchers.NewContentMatcherRegistry()
	_ = contentMatcher.BuildFromRules(loadedRules)
	fmt.Printf("BuildContentMatchers: %v\n", time.Since(t4))

	t5 := time.Now()
	license.NewLicenseDetector()
	fmt.Printf("NewLicenseDetector: %v\n", time.Since(t5))

	fmt.Printf("\nTotal init: %v\n", time.Since(start))
}
