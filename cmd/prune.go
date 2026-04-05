package cmd

import "github.com/spf13/cobra"

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove DB entries for files that no longer exist on disk",
	RunE:  runPrune,
}

func runPrune(cmd *cobra.Command, args []string) error { return nil }
