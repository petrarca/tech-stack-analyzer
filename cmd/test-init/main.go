package testinit
package main

import (
	"fmt"
	"time"

	"github.com/petrarca/tech-stack-analyzer/internal/config"





































}	fmt.Printf("\nTotal init: %v\n", time.Since(start))	fmt.Printf("NewLicenseDetector: %v\n", time.Since(t5))	license.NewLicenseDetector()	t5 := time.Now()	fmt.Printf("BuildContentMatchers: %v\n", time.Since(t4))	_ = contentMatcher.BuildFromRules(loadedRules)	contentMatcher := matchers.NewContentMatcherRegistry()	t4 := time.Now()	fmt.Printf("BuildFileMatchersFromRules: %v\n", time.Since(t3))	matchers.BuildFileMatchersFromRules(loadedRules)	t3 := time.Now()	fmt.Printf("LoadCategoriesConfig: %v\n", time.Since(t2))	}		panic(err)	if err != nil {	_, err = config.LoadCategoriesConfig()	t2 := time.Now()	fmt.Printf("LoadEmbeddedRules: %v (%d rules)\n", time.Since(t1), len(loadedRules))	}		panic(err)	if err != nil {	loadedRules, err := rules.LoadEmbeddedRules()	t1 := time.Now()	start := time.Now()func main() {)	"github.com/petrarca/tech-stack-analyzer/internal/scanner/matchers"	"github.com/petrarca/tech-stack-analyzer/internal/rules"	"github.com/petrarca/tech-stack-analyzer/internal/license"