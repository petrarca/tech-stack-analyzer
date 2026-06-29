package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/petrarca/tech-stack-analyzer/internal/currency"
	currencycache "github.com/petrarca/tech-stack-analyzer/internal/currency/cache"
	"github.com/petrarca/tech-stack-analyzer/internal/progress"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolvestats"
	"github.com/petrarca/tech-stack-analyzer/internal/store"
)

var (
	currencyOutput     string
	currencyCachePath  string
	currencyTTLHours   int
	currencyDepsDevURL string
	currencyForce      bool
	currencyQuiet      bool
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
	currencyCmd.Flags().BoolVarP(&currencyQuiet, "quiet", "q", false, "Suppress progress output")
}

func runCurrency(aggPath string) error {
	// Resolve and open the shared store (lazy: this is the consumer that needs it).
	cachePath, _, err := store.ResolvePath(currencyCachePath)
	if err != nil {
		return err
	}
	st, err := store.Open(cachePath, 5000)
	if err != nil {
		return fmt.Errorf("open currency cache: %w", err)
	}
	defer func() { _ = st.Close() }()

	ttl := time.Duration(currencyTTLHours) * time.Hour
	chain := currency.NewChainResolver(currency.NewDepsDevResolver(currencyDepsDevURL))
	resolver, err := currencycache.New(st, chain, ttl, currencyForce)
	if err != nil {
		return err
	}

	stop := startCurrencyReporter(currencyQuiet)
	art, err := currency.ResolveAggregateFile(aggPath, resolver, currency.Options{
		SourceEndpoint: depsDevEndpointOrDefault(currencyDepsDevURL),
		TTLHours:       currencyTTLHours,
		DirectOnly:     true,
	})
	stop()
	if err != nil {
		return err
	}

	out := currencyOutput
	if out == "" {
		out = currencyOutputFileFor(aggPath)
	}
	if err := art.WriteFile(out); err != nil {
		return err
	}
	if !currencyQuiet {
		s := art.Summary
		fmt.Fprintf(os.Stderr,
			"Currency written to %s (%d direct: %d up-to-date, %d patch, %d minor, %d major, %d unsupported, %d unknown)\n",
			out, s.Total, s.UpToDate, s.Patch, s.Minor, s.Major, s.Unsupported, s.Unknown)
	}
	return nil
}

// currencyOutputFileFor derives {stem}.currency.json from an input path,
// stripping a trailing -agg if present (so foo-agg.json -> foo.currency.json).
func currencyOutputFileFor(input string) string {
	ext := filepath.Ext(input)
	base := strings.TrimSuffix(input, ext)
	base = strings.TrimSuffix(base, "-agg")
	return base + ".currency.json"
}

func depsDevEndpointOrDefault(url string) string {
	if url == "" {
		return "https://api.deps.dev"
	}
	return url
}

// startCurrencyReporter mirrors startSBOMResolveReporter but samples the
// currency counter group and renders with FormatCurrency, so currency lookups
// show the same live progress as dependency resolution.
func startCurrencyReporter(quiet bool) func() {
	if quiet {
		return func() {}
	}
	prog := progress.New(true, progress.NewSimpleHandler(os.Stderr))
	base := resolvestats.Get()
	start := time.Now()
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		started := false
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				delta := resolvestats.Get().Sub(base)
				if !delta.CurrencyActive() {
					continue
				}
				if !started {
					started = true
					prog.ResolveStart()
				}
				prog.ResolveProgress(delta.FormatCurrency())
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
		delta := resolvestats.Get().Sub(base)
		if delta.CurrencyActive() {
			prog.ResolveComplete(delta.FormatCurrency(), time.Since(start))
		}
	}
}
