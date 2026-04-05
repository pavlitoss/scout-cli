package cmd

import (
	"fmt"
	"os"

	"github.com/pavlitoss/scout-cli/internal/ui"
	"github.com/spf13/cobra"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove DB entries for files that no longer exist on disk",
	RunE:  runPrune,
}

func runPrune(cmd *cobra.Command, args []string) error {
	files, err := database.GetAllFiles()
	if err != nil {
		return fmt.Errorf("prune: %w", err)
	}

	var pruned []string
	for _, f := range files {
		if _, err := os.Stat(f.Path); os.IsNotExist(err) {
			if err := database.DeleteFileByID(f.ID); err != nil {
				return fmt.Errorf("prune: delete %s: %w", f.Path, err)
			}
			pruned = append(pruned, f.Path)
		}
	}

	if len(pruned) == 0 {
		fmt.Println(ui.StyleMuted.Render("No stale entries found."))
		return nil
	}

	fmt.Printf("Pruned %d stale %s:\n", len(pruned), plural(len(pruned), "entry", "entries"))
	for _, p := range pruned {
		fmt.Printf("  %s\n", ui.FormatPath(p))
	}
	return nil
}
