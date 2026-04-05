package cmd

import "github.com/spf13/cobra"

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Manage watched workspaces",
}

var watchAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Register a directory as a workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runWatchAdd,
}

var watchRemoveCmd = &cobra.Command{
	Use:   "remove <path>",
	Short: "Unregister a workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runWatchRemove,
}

var watchListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered workspaces",
	RunE:  runWatchList,
}

var watchSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Rescan all workspaces for new and deleted files",
	RunE:  runWatchSync,
}

func runWatchAdd(cmd *cobra.Command, args []string) error    { return nil }
func runWatchRemove(cmd *cobra.Command, args []string) error { return nil }
func runWatchList(cmd *cobra.Command, args []string) error   { return nil }
func runWatchSync(cmd *cobra.Command, args []string) error   { return nil }
