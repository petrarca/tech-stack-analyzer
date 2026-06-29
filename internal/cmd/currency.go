package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/petrarca/tech-stack-analyzer/internal/currency"
	"github.com/petrarca/tech-stack-analyzer/internal/progress"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolvestats"
)

var (
	currencyOutput      string
	currencyCachePath   string
	currencyTTLHours    int
	currencyDepsDevURL  string
	currencyForce       bool
	currencyQuiet       bool
	currencyConcurrency int
)

// currencyCmd resolves dependency freshness from a previously written aggregate
// file and emits a {stem}.currency.json artifact. Re-running it is the refresh
// path. Like the sbom command, it transforms an existing scan artifact -- no
// re-scan -- but reads the AGGREGATE only (deduped, with reliable direct flags).
var currencyCmd = &cobra.Command{
	Use:   "currency <agg.json>",
	Short: "Resolve dependency currency (freshness) from an aggregate JSON",
	Long: `Resolve how far each direct dependency is behind its latest available
version, from a previously written aggregate (-agg.json) file, and write a
{stem}.currency.json artifact.

Latest versions come from Google deps.dev (opt-in network access). Results are
cached in a shared SQLite store with a per-entry TTL; re-running this command
refreshes the artifact, re-fetching only stale entries (or all of them with
--force).

Input is the AGGREGATE file only (deduped, with reliable direct/transitive
flags). Ecosystems deps.dev does not cover are recorded as unsupported; packages
deps.dev does not know (e.g. internal/private) are recorded as unknown.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return runCurrency(args[0])
	},
}

func init() {
	rootCmd.AddCommand(currencyCmd)
	currencyCmd.Flags().StringVarP(&currencyOutput, "output", "o", "", "Output file (default: <agg-stem>.currency.json)")
	currencyCmd.Flags().StringVar(&currencyCachePath, "currency-cache", "", "Override the currency cache DB path (default: STACK_ANALYZER_CURRENCY_CACHE or the OS cache dir)")
	currencyCmd.Flags().IntVar(&currencyTTLHours, "currency-ttl", 24, "Per-entry cache TTL in hours")
	currencyCmd.Flags().StringVar(&currencyDepsDevURL, "deps-dev-endpoint", "", "Base URL for deps.dev (default: public deps.dev). Override with an API-compatible mirror.")
	currencyCmd.Flags().BoolVar(&currencyForce, "force", false, "Ignore the cache TTL and re-fetch every package")
	currencyCmd.Flags().IntVar(&currencyConcurrency, "currency-concurrency", 10, "Parallel deps.dev lookups (higher is faster but riskier for rate limits)")
	currencyCmd.Flags().BoolVarP(&currencyQuiet, "quiet", "q", false, "Suppress progress output")
}

func runCurrency(aggPath string) error {
	deps, err := currency.LoadAggregateDeps(aggPath)
	if err != nil {
		return err
	}
	out := currencyOutput
	if out == "" {
		out = currencyOutputFileFor(aggPath)
	}
	return runCurrencyEngine(deps, out, currencyRunOpts{
		CachePath:   currencyCachePath,
		TTLHours:    currencyTTLHours,
		Endpoint:    currencyDepsDevURL,
		Concurrency: currencyConcurrency,
		Force:       currencyForce,
		Quiet:       currencyQuiet,
	})
}

// currencyOutputFileFor derives {stem}.currency.json from an input path,
// stripping a trailing -agg if present (so foo-agg.json -> foo.currency.json).
func currencyOutputFileFor(input string) string {
	ext := filepath.Ext(input)
	base := strings.TrimSuffix(input, ext)
	base = strings.TrimSuffix(base, "-agg")
	return base + ".currency.json"
}

func depsDevEndpointOrDefault(endpoint string) string {
	if endpoint == "" {
		return resolver.DefaultDepsDevBaseURL
	}
	return endpoint
}

// startCurrencyReporter shows live currency-resolution progress using the same
// animated spinner as dependency resolution (the SummaryHandler) on a TTY, and
// a plain line otherwise (piped/CI). Uses the shared startResolveReporter
// goroutine helper; currency-specific parts are the counter group, format
// function, tick rate, and optional bar-fraction update.
func startCurrencyReporter(quiet bool) func() {
	if quiet {
		return func() {}
	}
	isTTY := isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd())
	var prog *progress.Progress
	var summary *progress.SummaryHandler // non-nil only on a TTY (for the bar)
	if isTTY {
		summary = progress.NewSummaryHandler(os.Stderr, true)
		summary.SetPhaseLabel("currency")
		prog = progress.New(true, summary)
	} else {
		prog = progress.New(true, progress.NewSimpleHandler(os.Stderr))
	}

	// Fast tick on a TTY so the spinner animates; slower when piped (each tick
	// is a printed line there, so 2s avoids log spam).
	tick := 150 * time.Millisecond
	if !isTTY {
		tick = 2 * time.Second
	}

	var onTick func(resolvestats.Snapshot)
	if summary != nil {
		onTick = func(delta resolvestats.Snapshot) {
			if delta.CurrencyTotal > 0 {
				processed := delta.CurrencyResolved + delta.CurrencyUnsupported +
					delta.CurrencyUnpinned + delta.CurrencyUnknown + delta.CurrencyErrors
				summary.SetResolveFraction(float64(processed) / float64(delta.CurrencyTotal))
			}
		}
	}

	return startResolveReporter(resolveReporterOpts{
		prog:     prog,
		tick:     tick,
		isActive: func(s resolvestats.Snapshot) bool { return s.CurrencyActive() },
		format:   func(s resolvestats.Snapshot) string { return s.FormatCurrency() },
		onTick:   onTick,
	})
}
