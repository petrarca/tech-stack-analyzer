package codestats

import (
	"sync"
)

var initOnce sync.Once

// ProcessFile analyzes a file using SCC and aggregates stats
// language is the go-enry detected language name (used for grouping in by_language)
// If content is provided (non-nil, non-empty), it will be used; otherwise the file will be read
func (a *sccAnalyzer) ProcessFile(filename string, language string, content []byte) {
	// Process the file and get the stats using shared logic
	filejob, sccLang, success := a.processFileCommon(filename, language, content)
	if !success {
		return
	}

	// Aggregate results with thread safety
	a.mu.Lock()
	defer a.mu.Unlock()

	// Add to global stats using the shared unsafe method
	a.addToGlobalStatsUnsafe(filejob, language, sccLang)
}
