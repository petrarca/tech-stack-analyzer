package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	currencycache "github.com/petrarca/tech-stack-analyzer/internal/currency/cache"
	"github.com/petrarca/tech-stack-analyzer/internal/store"
)

var cacheClearExpiredOnly bool

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Remove cached entries (all, or --expired-only)",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return runCacheClear()
	},
}

func init() {
	cacheClearCmd.Flags().BoolVar(&cacheClearExpiredOnly, "expired-only", false,
		"Remove only entries past their TTL (keep fresh entries)")
}

func runCacheClear() error {
	path, source, err := store.ResolvePath(cacheCachePath)
	if err != nil {
		return err
	}
	// Never create the file: if it does not exist, there is nothing to clear.
	info, err := store.Stat(path, source)
	if err != nil {
		return err
	}
	if !info.Exists {
		fmt.Println("No cache yet; nothing to clear.")
		return nil
	}

	st, err := store.Open(path, 5000)
	if err != nil {
		return fmt.Errorf("open cache: %w", err)
	}
	defer func() { _ = st.Close() }()

	var n int64
	if cacheClearExpiredOnly {
		n, err = currencycache.ClearExpired(st)
	} else {
		n, err = currencycache.ClearAll(st)
	}
	if err != nil {
		return err
	}
	scope := "all"
	if cacheClearExpiredOnly {
		scope = "expired"
	}
	fmt.Fprintf(os.Stdout, "Removed %d %s currency entries.\n", n, scope)
	return nil
}
