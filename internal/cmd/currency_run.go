package cmd

// currency_run.go contains the shared currency-resolution pipeline used by
// both the 'currency' subcommand and the in-scan --resolve-currency path.
// Neither site is a special case; both build a cache-decorated resolver, drive
// the engine, and write the artifact. Any divergence belongs in the callers'
// wrapper, not here.
//
// It also hosts startResolveReporter, the shared resolution-progress goroutine
// used by both the currency reporter and the sbom transitive-resolve reporter.

import (
	"fmt"
	"os"
	"time"

	"github.com/petrarca/tech-stack-analyzer/internal/currency"
	currencycache "github.com/petrarca/tech-stack-analyzer/internal/currency/cache"
	"github.com/petrarca/tech-stack-analyzer/internal/progress"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolvestats"
	"github.com/petrarca/tech-stack-analyzer/internal/store"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// resolveReporterOpts configures the shared resolution-progress goroutine.
type resolveReporterOpts struct {
	prog     *progress.Progress
	tick     time.Duration
	isActive func(resolvestats.Snapshot) bool
	format   func(resolvestats.Snapshot) string
	onTick   func(resolvestats.Snapshot) // optional; called each tick (e.g. update bar)
}

// startResolveReporter is the shared ticker goroutine for any resolution phase.
// It samples resolvestats, drives prog.ResolveStart/Progress/Complete, and
// returns a stop function. Both the currency reporter and the sbom transitive
// reporter use this to avoid duplicating the goroutine boilerplate.
func startResolveReporter(opt resolveReporterOpts) func() {
	base := resolvestats.Get()
	start := time.Now()
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(opt.tick)
		defer ticker.Stop()
		started := false
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				delta := resolvestats.Get().Sub(base)
				if !opt.isActive(delta) {
					continue
				}
				if !started {
					started = true
					opt.prog.ResolveStart()
				}
				if opt.onTick != nil {
					opt.onTick(delta)
				}
				opt.prog.ResolveProgress(opt.format(delta))
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
		delta := resolvestats.Get().Sub(base)
		if opt.isActive(delta) {
			opt.prog.ResolveComplete(opt.format(delta), time.Since(start))
		}
	}
}

// currencyRunOpts bundles the parameters for a currency engine run.
type currencyRunOpts struct {
	CachePath   string
	TTLHours    int
	Endpoint    string // --deps-dev-endpoint (empty = public deps.dev)
	Concurrency int    // <=0 uses engine default
	Force       bool
	Quiet       bool
}

// runCurrencyEngine opens the shared store, builds the cache-decorated
// resolver, runs the engine over deps, emits live progress, and writes the
// artifact to outFile. Returns an error if any step fails; the caller is
// responsible for printing it.
func runCurrencyEngine(deps []types.Dependency, outFile string, opt currencyRunOpts) error {
	cachePath, _, err := store.ResolvePath(opt.CachePath)
	if err != nil {
		return err
	}
	st, err := store.Open(cachePath, 5000)
	if err != nil {
		return fmt.Errorf("open currency cache: %w", err)
	}
	defer func() { _ = st.Close() }()

	ttl := time.Duration(opt.TTLHours) * time.Hour
	chain := currency.NewChainResolver(currency.NewDepsDevResolver(opt.Endpoint))
	r, err := currencycache.New(st, chain, ttl, opt.Force)
	if err != nil {
		return err
	}

	stop := startCurrencyReporter(opt.Quiet)
	art := currency.Resolve(deps, r, currency.Options{
		SourceEndpoint: depsDevEndpointOrDefault(opt.Endpoint),
		TTLHours:       opt.TTLHours,
		DirectOnly:     true,
		Concurrency:    opt.Concurrency,
	})
	stop()

	if err := art.WriteFile(outFile); err != nil {
		return err
	}
	if !opt.Quiet {
		s := art.Summary
		fmt.Fprintf(os.Stderr,
			"Currency written to %s (%d direct: %d up-to-date, %d patch, %d minor, %d major, %d unsupported, %d unknown)\n",
			outFile, s.Total, s.UpToDate, s.Patch, s.Minor, s.Major, s.Unsupported, s.Unknown)
	}
	return nil
}
