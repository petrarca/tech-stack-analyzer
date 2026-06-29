package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/petrarca/tech-stack-analyzer/internal/store"
)

var cacheVacuumCmd = &cobra.Command{
	Use:   "vacuum",
	Short: "Reclaim space after deletions (SQLite VACUUM)",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return runCacheVacuum()
	},
}

func runCacheVacuum() error {
	path, source, err := store.ResolvePath(cacheCachePath)
	if err != nil {
		return err
	}
	info, err := store.Stat(path, source)
	if err != nil {
		return err
	}
	if !info.Exists {
		fmt.Println("No cache yet; nothing to vacuum.")
		return nil
	}

	st, err := store.Open(path, 5000)
	if err != nil {
		return fmt.Errorf("open cache: %w", err)
	}
	defer func() { _ = st.Close() }()

	before := info.SizeBytes
	if err := st.Vacuum(); err != nil {
		return err
	}
	after, _ := store.Stat(path, source)
	fmt.Printf("Vacuumed %s (%d -> %d bytes).\n", path, before, after.SizeBytes)
	return nil
}
