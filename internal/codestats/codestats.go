// Package codestats provides code statistics analysis (lines of code, comments, blanks, complexity)
package codestats

import (
	"math"
	"os"
	"sort"
	"sync"

	"github.com/boyter/scc/v3/processor"
	"github.com/go-enry/go-enry/v2"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// round2 rounds a float to 2 decimal places
func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

// Stats holds code statistics for a language or total (SCC-analyzed files)
type Stats struct {
	Lines      int64 `json:"lines"`
	Code       int64 `json:"code"`
	Comments   int64 `json:"comments"`
	Blanks     int64 `json:"blanks"`
	Complexity int64 `json:"complexity"`
	Files      int   `json:"files"`
}

// LanguageStats holds stats for a specific language (includes language name for sorted output)
type LanguageStats struct {
	Language   string `json:"language"`
	Lines      int64  `json:"lines"`
	Code       int64  `json:"code"`
	Comments   int64  `json:"comments"`
	Blanks     int64  `json:"blanks"`
	Complexity int64  `json:"complexity"`
	Files      int    `json:"files"`
}

// OtherStats holds statistics for files SCC cannot analyze (just line counts)
type OtherStats struct {
	Lines int64 `json:"lines"`
	Files int   `json:"files"`
}

// OtherLanguageStats holds stats for unanalyzed language (includes language name)
type OtherLanguageStats struct {
	Language string `json:"language"`
	Lines    int64  `json:"lines"`
	Files    int    `json:"files"`
}

// AnalyzedBucket holds stats for SCC-analyzed files (full code/comments/blanks breakdown)
type AnalyzedBucket struct {
	Total      Stats           `json:"total"`
	ByLanguage []LanguageStats `json:"by_language"` // Sorted by lines descending
}

// UnanalyzedBucket holds stats for files SCC cannot analyze (just line counts)
type UnanalyzedBucket struct {
	Total      OtherStats           `json:"total"`
	ByLanguage []OtherLanguageStats `json:"by_language"` // Sorted by lines descending
}

// PrimaryLanguage represents a primary programming language in Metrics
type PrimaryLanguage struct {
	Language string  `json:"language"`
	Pct      float64 `json:"pct"`
}

// Metrics holds derived code metrics (programming languages only)
type Metrics struct {
	CommentRatio      float64           `json:"comment_ratio"`       // comments / code (documentation level)
	CodeDensity       float64           `json:"code_density"`        // code / lines (actual code vs whitespace/comments)
	AvgFileSize       float64           `json:"avg_file_size"`       // lines / files
	ComplexityPerKLOC float64           `json:"complexity_per_kloc"` // complexity / (code / 1000)
	AvgComplexity     float64           `json:"avg_complexity"`      // complexity / files
	PrimaryLanguages  []PrimaryLanguage `json:"primary_languages"`   // primary programming languages (≥1%)
}

// TypeBucket holds stats for a language type (programming, data, markup, prose)
type TypeBucket struct {
	Total     Stats    `json:"total"`             // Aggregated stats for this type
	Metrics   *Metrics `json:"metrics,omitempty"` // Derived metrics (programming languages only)
	Languages []string `json:"languages"`         // Languages in this type (sorted by lines desc)
}

// ByType holds stats grouped by GitHub Linguist language type
type ByType struct {
	Programming *TypeBucket `json:"programming,omitempty"`
	Data        *TypeBucket `json:"data,omitempty"`
	Markup      *TypeBucket `json:"markup,omitempty"`
	Prose       *TypeBucket `json:"prose,omitempty"`
}

// CodeStats holds aggregated code statistics
type CodeStats struct {
	Total      Stats            `json:"total"`      // Grand total (analyzed only)
	ByType     ByType           `json:"by_type"`    // Stats grouped by language type (metrics in programming section)
	Analyzed   AnalyzedBucket   `json:"analyzed"`   // SCC-recognized languages
	Unanalyzed UnanalyzedBucket `json:"unanalyzed"` // Files SCC can't parse
}

// Analyzer interface for code statistics collection
type Analyzer interface {
	// ProcessFile analyzes a file and adds its stats
	// language is the go-enry detected language name (used for grouping)
	// If content is provided, it will be used; otherwise the file will be read
	ProcessFile(filename string, language string, content []byte)

	// ProcessFileForComponent analyzes a file for a specific component
	// componentID is used to group stats by component
	// language is the go-enry detected language name (used for grouping)
	// If content is provided, it will be used; otherwise the file will be read
	ProcessFileForComponent(filename string, language string, content []byte, componentID string)

	// GetStats returns the aggregated statistics
	GetStats() interface{}

	// GetComponentStats returns statistics for a specific component
	GetComponentStats(componentID string) interface{}

	// IsEnabled returns whether code stats collection is enabled
	IsEnabled() bool

	// IsPerComponentEnabled returns whether per-component code stats collection is enabled
	IsPerComponentEnabled() bool
}

// NewAnalyzer creates an analyzer based on enabled flag
func NewAnalyzer(enabled bool) Analyzer {
	if enabled {
		return newSCCAnalyzer(false) // Per-component disabled by default for backward compatibility
	}
	return &noopAnalyzer{}
}

// NewAnalyzerWithPerComponent creates an analyzer with per-component option
func NewAnalyzerWithPerComponent(enabled bool, perComponent bool) Analyzer {
	if enabled {
		return newSCCAnalyzerWithOptions(perComponent, 0.05, 5) // Default values
	}
	return &noopAnalyzer{}
}

// NewAnalyzerWithOptions creates an analyzer with all custom options
func NewAnalyzerWithOptions(enabled bool, perComponent bool, primaryThreshold float64, maxPrimaryLangs int) Analyzer {
	if enabled {
		return newSCCAnalyzerWithOptions(perComponent, primaryThreshold, maxPrimaryLangs)
	}
	return &noopAnalyzer{}
}

// noopAnalyzer is a no-op implementation when code stats are disabled
type noopAnalyzer struct{}

func (n *noopAnalyzer) ProcessFile(filename string, language string, content []byte) {} // language and content optional
func (n *noopAnalyzer) ProcessFileForComponent(filename string, language string, content []byte, componentID string) {
}                                                                        // component-specific analysis
func (n *noopAnalyzer) GetStats() interface{}                            { return nil }
func (n *noopAnalyzer) GetComponentStats(componentID string) interface{} { return nil }
func (n *noopAnalyzer) IsEnabled() bool                                  { return false }
func (n *noopAnalyzer) IsPerComponentEnabled() bool                      { return false }

// sccAnalyzer uses boyter/scc for code statistics
type sccAnalyzer struct {
	mu              sync.Mutex
	total           Stats                  // Grand total (code only)
	codeByLanguage  map[string]*Stats      // SCC-analyzed languages
	otherTotal      OtherStats             // Total for non-SCC files
	otherByLanguage map[string]*OtherStats // Non-SCC languages
	// By type aggregation (programming, data, markup, prose)
	byType map[string]*Stats // key: "programming", "data", "markup", "prose"
	// Per-component tracking
	perComponentEnabled bool                       // Whether per-component tracking is enabled
	componentStats      map[string]*componentStats // Stats by component ID
	// Primary language configuration
	primaryThreshold float64 // Minimum percentage for primary languages
	maxPrimaryLangs  int     // Maximum number of primary languages to show
}

// componentStats holds statistics for a single component
type componentStats struct {
	total           Stats                  // Grand total (code only)
	codeByLanguage  map[string]*Stats      // SCC-analyzed languages
	otherTotal      OtherStats             // Total for non-SCC files
	otherByLanguage map[string]*OtherStats // Non-SCC languages
	byType          map[string]*Stats      // By type aggregation (programming, data, markup, prose)
}

func newSCCAnalyzer(perComponent bool) *sccAnalyzer {
	return newSCCAnalyzerWithOptions(perComponent, 0.05, 5) // Default values
}

// newSCCAnalyzerWithOptions creates an analyzer with custom primary language settings
func newSCCAnalyzerWithOptions(perComponent bool, primaryThreshold float64, maxPrimaryLangs int) *sccAnalyzer {
	analyzer := &sccAnalyzer{
		codeByLanguage:   make(map[string]*Stats),
		otherByLanguage:  make(map[string]*OtherStats),
		byType:           make(map[string]*Stats),
		primaryThreshold: primaryThreshold,
		maxPrimaryLangs:  maxPrimaryLangs,
	}

	if perComponent {
		analyzer.perComponentEnabled = true
		analyzer.componentStats = make(map[string]*componentStats)
	}

	return analyzer
}

func (a *sccAnalyzer) IsEnabled() bool {
	return true
}

func (a *sccAnalyzer) IsPerComponentEnabled() bool {
	return a.perComponentEnabled
}

// buildAnalyzedSlice converts codeByLanguage map to sorted slice
func (a *sccAnalyzer) buildAnalyzedSlice() []LanguageStats {
	result := make([]LanguageStats, 0, len(a.codeByLanguage))
	for lang, stats := range a.codeByLanguage {
		result = append(result, LanguageStats{
			Language: lang, Lines: stats.Lines, Code: stats.Code,
			Comments: stats.Comments, Blanks: stats.Blanks,
			Complexity: stats.Complexity, Files: stats.Files,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Lines > result[j].Lines })
	return result
}

// buildUnanalyzedSlice converts otherByLanguage map to sorted slice
func (a *sccAnalyzer) buildUnanalyzedSlice() []OtherLanguageStats {
	result := make([]OtherLanguageStats, 0, len(a.otherByLanguage))
	for lang, stats := range a.otherByLanguage {
		result = append(result, OtherLanguageStats{Language: lang, Lines: stats.Lines, Files: stats.Files})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Lines > result[j].Lines })
	return result
}

// buildByType creates the ByType structure from language stats
func (a *sccAnalyzer) buildByType(analyzed []LanguageStats, unanalyzed []OtherLanguageStats, metrics Metrics) ByType {
	typeLanguages := make(map[string][]string)

	// Collect languages by type (analyzed)
	for _, ls := range analyzed {
		typeName := types.LanguageTypeToString(enry.GetLanguageType(ls.Language))
		if typeName != "unknown" {
			typeLanguages[typeName] = append(typeLanguages[typeName], ls.Language)
		}
	}
	// Collect languages by type (unanalyzed)
	for _, ls := range unanalyzed {
		typeName := types.LanguageTypeToString(enry.GetLanguageType(ls.Language))
		if typeName != "unknown" {
			typeLanguages[typeName] = append(typeLanguages[typeName], ls.Language)
		}
	}

	byType := ByType{}
	for typeName, langs := range typeLanguages {
		if stats, ok := a.byType[typeName]; ok {
			bucket := &TypeBucket{Total: *stats, Languages: langs}

			// Only add metrics for programming type (since they're calculated from programming languages only)
			if typeName == "programming" && stats.Code > 0 {
				bucket.Metrics = &metrics
			}

			switch typeName {
			case "programming":
				byType.Programming = bucket
			case "data":
				byType.Data = bucket
			case "markup":
				byType.Markup = bucket
			case "prose":
				byType.Prose = bucket
			}
		}
	}
	return byType
}

// calculateMetrics computes Metrics from programming language stats
func (a *sccAnalyzer) calculateMetrics(analyzed []LanguageStats) Metrics {
	kpis := Metrics{}
	progStats := a.byType["programming"]
	if progStats == nil {
		return kpis
	}

	if progStats.Code > 0 {
		kpis.CommentRatio = round2(float64(progStats.Comments) / float64(progStats.Code))
		kpis.ComplexityPerKLOC = round2(float64(progStats.Complexity) / (float64(progStats.Code) / 1000))
	}
	if progStats.Lines > 0 {
		kpis.CodeDensity = round2(float64(progStats.Code) / float64(progStats.Lines))
	}
	if progStats.Files > 0 {
		kpis.AvgFileSize = round2(float64(progStats.Lines) / float64(progStats.Files))
		kpis.AvgComplexity = round2(float64(progStats.Complexity) / float64(progStats.Files))
	}

	// Primary programming languages
	var progLangs []LanguageStats
	for _, ls := range analyzed {
		if enry.GetLanguageType(ls.Language) == enry.Programming {
			progLangs = append(progLangs, ls)
		}
	}
	a.setPrimaryLanguages(&kpis, progLangs, progStats.Lines)
	return kpis
}

// setPrimaryLanguages sets primary programming languages (uses configurable threshold and max)
func (a *sccAnalyzer) setPrimaryLanguages(kpis *Metrics, progLangs []LanguageStats, totalLines int64) {
	if totalLines == 0 || len(progLangs) == 0 {
		return
	}

	for i, ls := range progLangs {
		if i >= a.maxPrimaryLangs {
			break
		}
		pct := round2(float64(ls.Lines) / float64(totalLines))
		if pct < a.primaryThreshold {
			break // Languages are sorted by lines, so remaining will be smaller
		}
		kpis.PrimaryLanguages = append(kpis.PrimaryLanguages, PrimaryLanguage{
			Language: ls.Language,
			Pct:      pct,
		})
	}
}

func (a *sccAnalyzer) GetStats() interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()

	analyzed := a.buildAnalyzedSlice()
	unanalyzed := a.buildUnanalyzedSlice()
	metrics := a.calculateMetrics(analyzed)

	return &CodeStats{
		Total:      a.total,
		ByType:     a.buildByType(analyzed, unanalyzed, metrics),
		Analyzed:   AnalyzedBucket{Total: a.total, ByLanguage: analyzed},
		Unanalyzed: UnanalyzedBucket{Total: a.otherTotal, ByLanguage: unanalyzed},
	}
}

// ProcessFileForComponent analyzes a file for a specific component
// This processes the file ONCE and adds stats to both global and component buckets
func (a *sccAnalyzer) ProcessFileForComponent(filename string, language string, content []byte, componentID string) {
	// If per-component tracking is disabled, just use regular ProcessFile
	if !a.perComponentEnabled {
		a.ProcessFile(filename, language, content)
		return
	}

	// Process the file and get the stats (same logic as ProcessFile)
	filejob, sccLang, success := a.processFileCommon(filename, language, content)
	if !success {
		return
	}

	// Aggregate results with thread safety
	a.mu.Lock()
	defer a.mu.Unlock()

	// 1. Add to GLOBAL stats (existing behavior)
	a.addToGlobalStatsUnsafe(filejob, language, sccLang)

	// 2. Add to COMPONENT stats (new per-component feature)
	a.addComponentStatsUnsafe(filejob, language, sccLang, componentID)
}

// processFileCommon contains the shared logic for file processing
func (a *sccAnalyzer) processFileCommon(filename string, language string, content []byte) (*processor.FileJob, string, bool) {
	// Skip if no language detected by go-enry
	if language == "" {
		return nil, "", false
	}

	// Read file if content not provided
	if len(content) == 0 {
		var err error
		content, err = os.ReadFile(filename)
		if err != nil || len(content) == 0 {
			return nil, "", false
		}
	}

	// Initialize SCC language definitions once
	initOnce.Do(func() {
		processor.ProcessConstants()
	})

	// Detect SCC language for proper comment/code parsing
	sccLangs, _ := processor.DetectLanguage(filename)
	sccLang := ""
	if len(sccLangs) > 0 {
		sccLang = sccLangs[0]
	}

	// Create file job for SCC
	filejob := &processor.FileJob{
		Filename: filename,
		Language: sccLang,
		Content:  content,
		Bytes:    int64(len(content)),
	}

	// Count stats
	processor.CountStats(filejob)

	return filejob, sccLang, true
}

// addToGlobalStatsUnsafe adds file job results to global statistics (caller must hold mutex)
func (a *sccAnalyzer) addToGlobalStatsUnsafe(filejob *processor.FileJob, language string, sccLang string) {
	// This is the existing global aggregation logic from ProcessFile
	// Determine if SCC recognized this file
	sccRecognized := sccLang != ""

	if sccRecognized {
		// Analyzed bucket: SCC-analyzed files with full stats
		a.total.Lines += filejob.Lines
		a.total.Code += filejob.Code
		a.total.Comments += filejob.Comment
		a.total.Blanks += filejob.Blank
		a.total.Complexity += filejob.Complexity
		a.total.Files++

		if _, ok := a.codeByLanguage[language]; !ok {
			a.codeByLanguage[language] = &Stats{}
		}
		a.codeByLanguage[language].Lines += filejob.Lines
		a.codeByLanguage[language].Code += filejob.Code
		a.codeByLanguage[language].Comments += filejob.Comment
		a.codeByLanguage[language].Blanks += filejob.Blank
		a.codeByLanguage[language].Complexity += filejob.Complexity
		a.codeByLanguage[language].Files++

		// Get language type from go-enry (programming, data, markup, prose)
		langType := enry.GetLanguageType(language)
		typeName := types.LanguageTypeToString(langType)

		// Aggregate by language type
		if typeName != "unknown" {
			if _, ok := a.byType[typeName]; !ok {
				a.byType[typeName] = &Stats{}
			}
			a.byType[typeName].Lines += filejob.Lines
			a.byType[typeName].Code += filejob.Code
			a.byType[typeName].Comments += filejob.Comment
			a.byType[typeName].Blanks += filejob.Blank
			a.byType[typeName].Complexity += filejob.Complexity
			a.byType[typeName].Files++
		}
	} else {
		// Other bucket: files SCC can't analyze (just line counts)
		a.otherTotal.Lines += filejob.Lines
		a.otherTotal.Files++

		if _, ok := a.otherByLanguage[language]; !ok {
			a.otherByLanguage[language] = &OtherStats{}
		}
		a.otherByLanguage[language].Lines += filejob.Lines
		a.otherByLanguage[language].Files++

		// Also aggregate by type for unanalyzed files
		langType := enry.GetLanguageType(language)
		typeName := types.LanguageTypeToString(langType)
		if typeName != "unknown" {
			if _, ok := a.byType[typeName]; !ok {
				a.byType[typeName] = &Stats{}
			}
			a.byType[typeName].Lines += filejob.Lines
			a.byType[typeName].Files++
		}
	}
}

// addComponentStatsUnsafe adds file job results to component statistics (caller must hold mutex)
func (a *sccAnalyzer) addComponentStatsUnsafe(filejob *processor.FileJob, language string, sccLang string, componentID string) {
	// Initialize component stats if not exists
	if _, ok := a.componentStats[componentID]; !ok {
		a.componentStats[componentID] = &componentStats{
			codeByLanguage:  make(map[string]*Stats),
			otherByLanguage: make(map[string]*OtherStats),
			byType:          make(map[string]*Stats),
		}
	}

	compStats := a.componentStats[componentID]

	// Determine if SCC recognized this file
	sccRecognized := sccLang != ""

	if sccRecognized {
		// Analyzed bucket: SCC-analyzed files with full stats
		compStats.total.Lines += filejob.Lines
		compStats.total.Code += filejob.Code
		compStats.total.Comments += filejob.Comment
		compStats.total.Blanks += filejob.Blank
		compStats.total.Complexity += filejob.Complexity
		compStats.total.Files++

		if _, ok := compStats.codeByLanguage[language]; !ok {
			compStats.codeByLanguage[language] = &Stats{}
		}
		compStats.codeByLanguage[language].Lines += filejob.Lines
		compStats.codeByLanguage[language].Code += filejob.Code
		compStats.codeByLanguage[language].Comments += filejob.Comment
		compStats.codeByLanguage[language].Blanks += filejob.Blank
		compStats.codeByLanguage[language].Complexity += filejob.Complexity
		compStats.codeByLanguage[language].Files++

		// Aggregate by language type (programming, data, markup, prose)
		langType := enry.GetLanguageType(language)
		typeName := types.LanguageTypeToString(langType)
		if typeName != "unknown" {
			if _, ok := compStats.byType[typeName]; !ok {
				compStats.byType[typeName] = &Stats{}
			}
			compStats.byType[typeName].Lines += filejob.Lines
			compStats.byType[typeName].Code += filejob.Code
			compStats.byType[typeName].Comments += filejob.Comment
			compStats.byType[typeName].Blanks += filejob.Blank
			compStats.byType[typeName].Complexity += filejob.Complexity
			compStats.byType[typeName].Files++
		}
	} else {
		// Other bucket: files SCC can't analyze (just line counts)
		compStats.otherTotal.Lines += filejob.Lines
		compStats.otherTotal.Files++

		if _, ok := compStats.otherByLanguage[language]; !ok {
			compStats.otherByLanguage[language] = &OtherStats{}
		}
		compStats.otherByLanguage[language].Lines += filejob.Lines
		compStats.otherByLanguage[language].Files++

		// Also aggregate by type for unanalyzed files
		langType := enry.GetLanguageType(language)
		typeName := types.LanguageTypeToString(langType)
		if typeName != "unknown" {
			if _, ok := compStats.byType[typeName]; !ok {
				compStats.byType[typeName] = &Stats{}
			}
			compStats.byType[typeName].Lines += filejob.Lines
			compStats.byType[typeName].Files++
		}
	}
}

// GetComponentStats returns statistics for a specific component
func (a *sccAnalyzer) GetComponentStats(componentID string) interface{} {
	if !a.perComponentEnabled {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	compStats, exists := a.componentStats[componentID]
	if !exists {
		return nil
	}

	// Build analyzed and unanalyzed slices for this component
	analyzed := make([]LanguageStats, 0, len(compStats.codeByLanguage))
	for lang, stats := range compStats.codeByLanguage {
		analyzed = append(analyzed, LanguageStats{
			Language: lang, Lines: stats.Lines, Code: stats.Code,
			Comments: stats.Comments, Blanks: stats.Blanks,
			Complexity: stats.Complexity, Files: stats.Files,
		})
	}

	unanalyzed := make([]OtherLanguageStats, 0, len(compStats.otherByLanguage))
	for lang, stats := range compStats.otherByLanguage {
		unanalyzed = append(unanalyzed, OtherLanguageStats{
			Language: lang, Lines: stats.Lines, Files: stats.Files,
		})
	}

	// Sort slices by lines descending (like global stats)
	sort.Slice(analyzed, func(i, j int) bool { return analyzed[i].Lines > analyzed[j].Lines })
	sort.Slice(unanalyzed, func(i, j int) bool { return unanalyzed[i].Lines > unanalyzed[j].Lines })

	// Build by-type structure with metrics for components
	byType := a.buildComponentByType(compStats, analyzed, unanalyzed)

	return &CodeStats{
		Total:      compStats.total,
		ByType:     byType,
		Analyzed:   AnalyzedBucket{Total: compStats.total, ByLanguage: analyzed},
		Unanalyzed: UnanalyzedBucket{Total: compStats.otherTotal, ByLanguage: unanalyzed},
	}
}

// buildComponentByType builds by-type stats for components with metrics
func (a *sccAnalyzer) buildComponentByType(compStats *componentStats, analyzed []LanguageStats, unanalyzed []OtherLanguageStats) ByType {
	// Collect languages by type
	typeLanguages := make(map[string][]string)

	// Collect languages by type (analyzed)
	for _, ls := range analyzed {
		typeName := types.LanguageTypeToString(enry.GetLanguageType(ls.Language))
		if typeName != "unknown" {
			typeLanguages[typeName] = append(typeLanguages[typeName], ls.Language)
		}
	}
	// Collect languages by type (unanalyzed)
	for _, ls := range unanalyzed {
		typeName := types.LanguageTypeToString(enry.GetLanguageType(ls.Language))
		if typeName != "unknown" {
			typeLanguages[typeName] = append(typeLanguages[typeName], ls.Language)
		}
	}

	byType := ByType{}
	for typeName, langs := range typeLanguages {
		stats, ok := compStats.byType[typeName]
		if !ok {
			continue
		}

		bucket := &TypeBucket{Total: *stats, Languages: langs}

		// Calculate metrics for programming type
		if typeName == "programming" && stats.Code > 0 {
			metrics := a.calculateComponentMetricsForType(*stats, analyzed)
			bucket.Metrics = &metrics
		}

		switch typeName {
		case "programming":
			byType.Programming = bucket
		case "data":
			byType.Data = bucket
		case "markup":
			byType.Markup = bucket
		case "prose":
			byType.Prose = bucket
		}
	}
	return byType
}

// calculateComponentMetricsForType calculates metrics including top languages for a component type
func (a *sccAnalyzer) calculateComponentMetricsForType(stats Stats, analyzed []LanguageStats) Metrics {
	kpis := Metrics{}

	if stats.Code > 0 {
		kpis.CommentRatio = round2(float64(stats.Comments) / float64(stats.Code))
		kpis.ComplexityPerKLOC = round2(float64(stats.Complexity) / (float64(stats.Code) / 1000))
	}
	if stats.Lines > 0 {
		kpis.CodeDensity = round2(float64(stats.Code) / float64(stats.Lines))
	}
	if stats.Files > 0 {
		kpis.AvgFileSize = round2(float64(stats.Lines) / float64(stats.Files))
		kpis.AvgComplexity = round2(float64(stats.Complexity) / float64(stats.Files))
	}

	// Calculate primary programming languages for this component
	var progLangs []LanguageStats
	for _, ls := range analyzed {
		if enry.GetLanguageType(ls.Language) == enry.Programming {
			progLangs = append(progLangs, ls)
		}
	}
	// Sort by lines descending
	sort.Slice(progLangs, func(i, j int) bool { return progLangs[i].Lines > progLangs[j].Lines })
	a.setPrimaryLanguages(&kpis, progLangs, stats.Lines)

	return kpis
}
