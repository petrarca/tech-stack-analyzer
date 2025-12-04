package main

import (
	"fmt"
	"time"

	"github.com/petrarca/tech-stack-analyzer/internal/git"
)

func main() {
	// Test on a directory that's definitely not a git repository
	testPath := "/tmp"

	fmt.Printf("Benchmarking git.GetGitInfo on non-git directory: %s\n", testPath)
	fmt.Printf("Running 10,000 calls...\n\n")

	// Warm up call
	git.GetGitInfo(testPath)

	// Benchmark 10,000 calls
	start := time.Now()
	for i := 0; i < 10000; i++ {
		git.GetGitInfo(testPath)
	}
	duration := time.Since(start)

	fmt.Printf("Results:\n")
	fmt.Printf("- Total time: %v\n", duration)
	fmt.Printf("- Average per call: %v\n", duration/10000)
	fmt.Printf("- Calls per second: %.0f\n", float64(10000)/duration.Seconds())

	// Test on a git repository for comparison
	fmt.Printf("\nComparison: 1,000 calls on a git repository...\n")
	gitPath := "/Users/wmi/Develop/petrarca/tech-stack-analyzer"

	start = time.Now()
	for i := 0; i < 1000; i++ {
		git.GetGitInfo(gitPath)
	}
	gitDuration := time.Since(start)

	fmt.Printf("- Total time: %v\n", gitDuration)
	fmt.Printf("- Average per call: %v\n", gitDuration/1000)
	fmt.Printf("- Calls per second: %.0f\n", float64(1000)/gitDuration.Seconds())

	// Calculate overhead ratio
	overheadRatio := float64(gitDuration) / float64(duration) * 10 // Adjust for different call counts
	fmt.Printf("\nOverhead ratio (git vs non-git): %.2fx\n", overheadRatio)
}
