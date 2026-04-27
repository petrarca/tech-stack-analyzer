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

// Analyzer interface for code statistics collection.
// ProcessFile runs SCC once per file and distributes results to global, component, and
// subsystem buckets in a single pass. Pass empty strings for componentKey/subsystemKey
// to skip those buckets.
type Analyzer interface {
	// ProcessFile analyzes a file and adds its stats to the applicable buckets.
	// language is the go-enry detected language name (used for grouping).
	// typeOverride: if non-empty, overrides enry.GetLanguageType() for by-type aggregation
	//   (one of "programming", "data", "markup", "prose"). Empty string = use go-enry default.
	// componentKey: stable component identifier (e.g. manifest path); empty = skip component bucket.
	// subsystemKey: subsystem identifier (depth-N prefix or group name); empty = skip subsystem bucket.
	ProcessFile(filename, language, typeOverride string, content []byte, componentKey, subsystemKey string)

	// GetStats returns the aggregated global statistics. Returns nil when disabled.
	GetStats() *CodeStats

	// GetComponentStats returns statistics for the component identified by key.
	// Returns nil when per-component tracking is disabled or no stats were recorded.
	GetComponentStats(key string) *CodeStats

	// IsEnabled returns whether code stats collection is enabled.
	IsEnabled() bool
}

// SubsystemAnalyzer is an optional interface for subsystem-level stats retrieval.
// Satisfied by the SCC analyzer when subsystem tracking is enabled.
// Callers use a type assertion: if sa, ok := analyzer.(SubsystemAnalyzer); ok { ... }
type SubsystemAnalyzer interface {
	GetSubsystemStats(key string) *CodeStats
	SubsystemKeys() []string
}

// AnalyzerConfig holds all configuration for creating an Analyzer.
type AnalyzerConfig struct {
	PerComponent     bool    // Enable per-component stats tracking
	Subsystem        bool    // Enable subsystem-level stats tracking
	PrimaryThreshold float64 // Minimum percentage for primary languages (default: 0.05)
	MaxPrimaryLangs  int     // Maximum primary languages to show (default: 5)
}

// NewAnalyzer creates a code stats analyzer from config. Returns a no-op implementation
// when the zero-value config is passed.
func NewAnalyzer(cfg AnalyzerConfig) Analyzer {
	if cfg.PrimaryThreshold <= 0 {
		cfg.PrimaryThreshold = 0.05
	}
	if cfg.MaxPrimaryLangs <= 0 {
		cfg.MaxPrimaryLangs = 5
	}
	a := &sccAnalyzer{
		codeByLanguage:   make(map[string]*Stats),
		otherByLanguage:  make(map[string]*OtherStats),
		byType:           make(map[string]*Stats),
		languageType:     make(map[string]string),
		primaryThreshold: cfg.PrimaryThreshold,
		maxPrimaryLangs:  cfg.MaxPrimaryLangs,
	}
	if cfg.PerComponent {
		a.perComponentEnabled = true
		a.componentBuckets = make(map[string]*statsBucket)
	}
	if cfg.Subsystem {
		a.subsystemEnabled = true
		a.subsystemStats = make(map[string]*statsBucket)
	}
	return a
}

// NewNoopAnalyzer returns a disabled analyzer that does nothing.
func NewNoopAnalyzer() Analyzer {
	return &noopAnalyzer{}
}

// noopAnalyzer is a no-op implementation when code stats are disabled
type noopAnalyzer struct{}

func (n *noopAnalyzer) ProcessFile(_, _, _ string, _ []byte, _, _ string) {}
func (n *noopAnalyzer) GetStats() *CodeStats                              { return nil }
func (n *noopAnalyzer) GetComponentStats(_ string) *CodeStats             { return nil }
func (n *noopAnalyzer) IsEnabled() bool                                   { return false }

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
	perComponentEnabled bool                    // Whether per-component tracking is enabled
	componentBuckets    map[string]*statsBucket // Stats by component path
	// Per-subsystem tracking (rollup per depth-N path prefix, e.g. "/med")
	subsystemEnabled bool                    // Whether subsystem tracking is enabled
	subsystemStats   map[string]*statsBucket // Stats by subsystem key (reuses statsBucket struct)
	// language label → resolved type (honours reclassify overrides; used by buildByType)
	languageType map[string]string
	// Primary language configuration
	primaryThreshold float64 // Minimum percentage for primary languages
	maxPrimaryLangs  int     // Maximum number of primary languages to show
}

// statsBucket holds statistics for a single component or subsystem.
type statsBucket struct {
	total           Stats                  // Grand total (code only)
	codeByLanguage  map[string]*Stats      // SCC-analyzed languages
	otherTotal      OtherStats             // Total for non-SCC files
	otherByLanguage map[string]*OtherStats // Non-SCC languages
	byType          map[string]*Stats      // By type aggregation (programming, data, markup, prose)
	languageType    map[string]string      // language label → resolved type (honours reclassify overrides)
}

func (a *sccAnalyzer) IsEnabled() bool { return true }

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

	// Collect languages by type — use the stored resolved type (honours reclassify overrides)
	// rather than re-querying go-enry which would ignore any reclassification.
	for _, ls := range analyzed {
		typeName := a.languageType[ls.Language]
		if typeName == "" {
			typeName = types.LanguageTypeToString(enry.GetLanguageType(ls.Language))
		}
		if typeName != "unknown" {
			typeLanguages[typeName] = append(typeLanguages[typeName], ls.Language)
		}
	}
	for _, ls := range unanalyzed {
		typeName := a.languageType[ls.Language]
		if typeName == "" {
			typeName = types.LanguageTypeToString(enry.GetLanguageType(ls.Language))
		}
		if typeName != "unknown" {
			typeLanguages[typeName] = append(typeLanguages[typeName], ls.Language)
		}
	}

	// Iterate a.byType directly so type-only reclassified files (which have no language
	// label) still produce a type bucket even when typeLanguages has no entry for them.
	byType := ByType{}
	for typeName, stats := range a.byType {
		langs := typeLanguages[typeName] // may be nil for type-only reclassified files
		bucket := &TypeBucket{Total: *stats, Languages: langs}

		// Only add metrics for programming type
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

func (a *sccAnalyzer) GetStats() *CodeStats {
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

// ProcessFile analyzes a file once and distributes results to global, component, and subsystem buckets.
// typeOverride: if non-empty, overrides enry.GetLanguageType() for by-type aggregation.
// componentKey and subsystemKey are optional — empty string means skip that bucket.
func (a *sccAnalyzer) ProcessFile(filename, language, typeOverride string, content []byte, componentKey, subsystemKey string) {
	// Skip entirely when there is neither a language label nor a type override.
	if language == "" && typeOverride == "" {
		return
	}
	// For type-only reclassify rules (language == "", typeOverride != ""), processFileCommon
	// would exit early on the empty-language guard. We call it only when language is known;
	// otherwise we create a minimal filejob directly so the file still gets line-counted.
	var filejob *processor.FileJob
	var sccLang string
	if language != "" {
		var ok bool
		filejob, sccLang, ok = a.processFileCommon(filename, language, content)
		if !ok {
			return
		}
	} else {
		// Type-only: count lines via SCC but do not record per-language stats.
		if len(content) == 0 {
			var err error
			content, err = os.ReadFile(filename)
			if err != nil {
				return
			}
		}
		initOnce.Do(func() { processor.ProcessConstants() })
		sccLangs, _ := processor.DetectLanguage(filename)
		if len(sccLangs) > 0 {
			sccLang = sccLangs[0]
		}
		filejob = &processor.FileJob{
			Filename: filename,
			Language: sccLang,
			Content:  content,
			Bytes:    int64(len(content)),
		}
		processor.CountStats(filejob)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Always add to global stats
	a.addToGlobalStatsUnsafe(filejob, language, sccLang, typeOverride)

	// Optionally add to component bucket
	if a.perComponentEnabled && componentKey != "" {
		a.addToBucketUnsafe(filejob, language, sccLang, typeOverride, componentKey, a.componentBuckets)
	}

	// Optionally add to subsystem bucket
	if a.subsystemEnabled && subsystemKey != "" {
		a.addToBucketUnsafe(filejob, language, sccLang, typeOverride, subsystemKey, a.subsystemStats)
	}
}

// processFileCommon contains the shared logic for file processing
func (a *sccAnalyzer) processFileCommon(filename string, language string, content []byte) (*processor.FileJob, string, bool) {

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

// resolveTypeName returns the language type name for by-type aggregation.
// If typeOverride is set (from a reclassify rule), it takes precedence over go-enry's detection.
func resolveTypeName(language, typeOverride string) string {
	if typeOverride != "" {
		return typeOverride
	}
	langType := enry.GetLanguageType(language)
	return types.LanguageTypeToString(langType)
}

// addToGlobalStatsUnsafe adds file job results to global statistics (caller must hold mutex)
func (a *sccAnalyzer) addToGlobalStatsUnsafe(filejob *processor.FileJob, language string, sccLang string, typeOverride string) {
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

		// Only record per-language stats when we have a language label
		// (type-only reclassify rules leave language empty intentionally)
		typeName := resolveTypeName(language, typeOverride)
		if language != "" {
			if _, ok := a.codeByLanguage[language]; !ok {
				a.codeByLanguage[language] = &Stats{}
			}
			a.codeByLanguage[language].Lines += filejob.Lines
			a.codeByLanguage[language].Code += filejob.Code
			a.codeByLanguage[language].Comments += filejob.Comment
			a.codeByLanguage[language].Blanks += filejob.Blank
			a.codeByLanguage[language].Complexity += filejob.Complexity
			a.codeByLanguage[language].Files++
			// Record resolved type for buildByType (honours reclassify overrides)
			a.languageType[language] = typeName
		}

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

		typeName := resolveTypeName(language, typeOverride)
		if language != "" {
			if _, ok := a.otherByLanguage[language]; !ok {
				a.otherByLanguage[language] = &OtherStats{}
			}
			a.otherByLanguage[language].Lines += filejob.Lines
			a.otherByLanguage[language].Files++
			a.languageType[language] = typeName
		}
		if typeName != "unknown" {
			if _, ok := a.byType[typeName]; !ok {
				a.byType[typeName] = &Stats{}
			}
			a.byType[typeName].Lines += filejob.Lines
			a.byType[typeName].Files++
		}
	}
}

// addToBucketUnsafe adds file job results to a keyed stats bucket (caller must hold mutex).
// statsMap is either statsBucket or subsystemStats — same logic, different map.
func (a *sccAnalyzer) addToBucketUnsafe(filejob *processor.FileJob, language string, sccLang string, typeOverride string, key string, statsMap map[string]*statsBucket) {
	if _, ok := statsMap[key]; !ok {
		statsMap[key] = &statsBucket{
			codeByLanguage:  make(map[string]*Stats),
			otherByLanguage: make(map[string]*OtherStats),
			byType:          make(map[string]*Stats),
			languageType:    make(map[string]string),
		}
	}

	compStats := statsMap[key]

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

		typeName := resolveTypeName(language, typeOverride)
		if language != "" {
			if _, ok := compStats.codeByLanguage[language]; !ok {
				compStats.codeByLanguage[language] = &Stats{}
			}
			compStats.codeByLanguage[language].Lines += filejob.Lines
			compStats.codeByLanguage[language].Code += filejob.Code
			compStats.codeByLanguage[language].Comments += filejob.Comment
			compStats.codeByLanguage[language].Blanks += filejob.Blank
			compStats.codeByLanguage[language].Complexity += filejob.Complexity
			compStats.codeByLanguage[language].Files++
			compStats.languageType[language] = typeName
		}

		// Aggregate by language type — reclassify override takes precedence
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

		typeName := resolveTypeName(language, typeOverride)
		if language != "" {
			if _, ok := compStats.otherByLanguage[language]; !ok {
				compStats.otherByLanguage[language] = &OtherStats{}
			}
			compStats.otherByLanguage[language].Lines += filejob.Lines
			compStats.otherByLanguage[language].Files++
			compStats.languageType[language] = typeName
		}

		// Also aggregate by type for unanalyzed files
		typeName = resolveTypeName(language, typeOverride)
		if typeName != "unknown" {
			if _, ok := compStats.byType[typeName]; !ok {
				compStats.byType[typeName] = &Stats{}
			}
			compStats.byType[typeName].Lines += filejob.Lines
			compStats.byType[typeName].Files++
		}
	}
}

// GetComponentStats returns statistics for a specific component.
func (a *sccAnalyzer) GetComponentStats(componentID string) *CodeStats {
	if !a.perComponentEnabled {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	compStats, exists := a.componentBuckets[componentID]
	if !exists {
		return nil
	}
	return a.buildCodeStatsFromComponentStats(compStats)
}

// SubsystemKeys returns all subsystem keys that have collected stats, sorted.
func (a *sccAnalyzer) SubsystemKeys() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	keys := make([]string, 0, len(a.subsystemStats))
	for k := range a.subsystemStats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// GetSubsystemStats returns statistics for the subsystem identified by subsystemKey.
func (a *sccAnalyzer) GetSubsystemStats(subsystemKey string) *CodeStats {
	if !a.subsystemEnabled {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	compStats, exists := a.subsystemStats[subsystemKey]
	if !exists {
		return nil
	}
	return a.buildCodeStatsFromComponentStats(compStats)
}

// buildCodeStatsFromComponentStats builds a CodeStats from a statsBucket bucket.
func (a *sccAnalyzer) buildCodeStatsFromComponentStats(compStats *statsBucket) *CodeStats {
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
		unanalyzed = append(unanalyzed, OtherLanguageStats{Language: lang, Lines: stats.Lines, Files: stats.Files})
	}
	sort.Slice(analyzed, func(i, j int) bool { return analyzed[i].Lines > analyzed[j].Lines })
	sort.Slice(unanalyzed, func(i, j int) bool { return unanalyzed[i].Lines > unanalyzed[j].Lines })
	byType := a.buildComponentByType(compStats, analyzed, unanalyzed)
	return &CodeStats{
		Total:      compStats.total,
		ByType:     byType,
		Analyzed:   AnalyzedBucket{Total: compStats.total, ByLanguage: analyzed},
		Unanalyzed: UnanalyzedBucket{Total: compStats.otherTotal, ByLanguage: unanalyzed},
	}
}

// buildComponentByType builds by-type stats for components with metrics
func (a *sccAnalyzer) buildComponentByType(compStats *statsBucket, analyzed []LanguageStats, unanalyzed []OtherLanguageStats) ByType {
	typeLanguages := make(map[string][]string)

	// Use the bucket's stored resolved type (honours reclassify overrides).
	for _, ls := range analyzed {
		typeName := compStats.languageType[ls.Language]
		if typeName == "" {
			typeName = types.LanguageTypeToString(enry.GetLanguageType(ls.Language))
		}
		if typeName != "unknown" {
			typeLanguages[typeName] = append(typeLanguages[typeName], ls.Language)
		}
	}
	for _, ls := range unanalyzed {
		typeName := compStats.languageType[ls.Language]
		if typeName == "" {
			typeName = types.LanguageTypeToString(enry.GetLanguageType(ls.Language))
		}
		if typeName != "unknown" {
			typeLanguages[typeName] = append(typeLanguages[typeName], ls.Language)
		}
	}

	byType := ByType{}
	for typeName, stats := range compStats.byType {
		langs := typeLanguages[typeName] // may be nil for type-only reclassified files
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
