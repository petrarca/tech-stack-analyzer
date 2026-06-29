package cmd

import (
	"fmt"
	"sort"

	humanize "github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/petrarca/tech-stack-analyzer/internal/store"
)

var cacheInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show the cache location, size, and record counts",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return runCacheInfo()
	},
}

func runCacheInfo() error {
	path, source, err := store.ResolvePath(cacheCachePath)
	if err != nil {
		return err
	}
	info, err := store.Stat(path, source)
	if err != nil {
		return err
	}

	fmt.Printf("Location: %s (%s)\n", info.Path, info.Source)
	if !info.Exists {
		fmt.Println("Status:   no cache yet")
		return nil
	}
	fmt.Printf("Size:     %s\n", humanize.Bytes(uint64(info.SizeBytes)))
	if info.SchemaVersion != "" {
		fmt.Printf("Schema:   v%s\n", info.SchemaVersion)
	}

	if len(info.TableRows) == 0 {
		fmt.Println("Records:  (empty)")
		return nil
	}
	tables := make([]string, 0, len(info.TableRows))
	for t := range info.TableRows {
		tables = append(tables, t)
	}
	sort.Strings(tables)
	fmt.Println("Records:")
	for _, t := range tables {
		fmt.Printf("  %-12s %d\n", t, info.TableRows[t])
	}
	return nil
}
