package cmd

import (
	"github.com/spf13/cobra"
)

// cacheCmd is the parent for managing the shared currency/store cache.
var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Inspect and manage the shared currency cache",
	Long: `Inspect and manage the shared SQLite cache used by currency resolution.

The cache stores latest-version lookups across runs and products. These commands
never create the cache file: if it does not exist, they report "no cache yet".`,
}

// cacheCachePath is the shared --currency-cache override for all cache
// subcommands (path resolution: flag > STACK_ANALYZER_CURRENCY_CACHE > default).
var cacheCachePath string

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(cacheInfoCmd)
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheVacuumCmd)

	cacheCmd.PersistentFlags().StringVar(&cacheCachePath, "currency-cache", "",
		"Cache DB path (default: STACK_ANALYZER_CURRENCY_CACHE or the OS cache dir)")
}
