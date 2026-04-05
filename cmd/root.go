package cmd

import (
	"fmt"
	"strings"

	"github.com/pavlitoss/scout-cli/internal/db"
	"github.com/spf13/cobra"
)

// database is the open DB connection shared by all subcommands.
// It is opened in PersistentPreRunE and closed in PersistentPostRunE.
var database *db.DB

var rootCmd = &cobra.Command{
	Use:   "scout",
	Short: "Find your files fast",
	Long:  "scout — tag and search your files using workspaces and full-text search.",

	// ArbitraryArgs allows 0, 1, or 2 positional args so we can dispatch
	// manually below rather than letting cobra reject them.
	Args: cobra.ArbitraryArgs,

	// SilenceUsage prevents cobra from printing the usage block when a
	// runtime error occurs (it only shows on bad argument errors).
	SilenceUsage: true,

	// PersistentPreRunE runs before every command (including subcommands).
	// We open the database here once so all handlers share the connection.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		database = d
		return nil
	},

	// PersistentPostRunE closes the DB after every command finishes.
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if database != nil {
			return database.Close()
		}
		return nil
	},

	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		// Dispatch based on argument shape:
		//   scout @tagname           → list files under that tag
		//   scout "some query"       → full-text search
		//   scout @tagname "query"   → search scoped to tag
		first := args[0]

		if len(args) == 2 && strings.HasPrefix(first, "@") {
			return scopedSearchAction(first, args[1])
		}

		if strings.HasPrefix(first, "@") {
			return tagShowAction(first)
		}

		return searchAction(first)
	},
}

// Execute is the entry point called from main.go.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Register all top-level subcommands
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(tagCmd)
	rootCmd.AddCommand(pruneCmd)

	// Register watch subcommands under watch
	watchCmd.AddCommand(watchAddCmd)
	watchCmd.AddCommand(watchRemoveCmd)
	watchCmd.AddCommand(watchListCmd)
	watchCmd.AddCommand(watchSyncCmd)

	// Register tag subcommands under tag
	tagCmd.AddCommand(tagAddCmd)
	tagCmd.AddCommand(tagRemoveCmd)
	tagCmd.AddCommand(tagListCmd)
	tagCmd.AddCommand(tagShowCmd)
}

// --- Action stubs (will be filled in during later phases) ---

func tagShowAction(tag string) error {
	fmt.Printf("(stub) tag show: %s\n", tag)
	return nil
}

func searchAction(query string) error {
	fmt.Printf("(stub) search: %s\n", query)
	return nil
}

func scopedSearchAction(tag, query string) error {
	fmt.Printf("(stub) scoped search: %s %q\n", tag, query)
	return nil
}
