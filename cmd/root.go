package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pavlitoss/scout-cli/internal/db"
	"github.com/pavlitoss/scout-cli/internal/ui"
	"github.com/spf13/cobra"
)

var version = "dev" // overridden at build time via -X cmd.version=...

var banner = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render(`
███████╗ ██████╗ ██████╗ ██╗   ██╗████████╗
██╔════╝██╔════╝██╔═══██╗██║   ██║╚══██╔══╝
███████╗██║     ██║   ██║██║   ██║   ██║
╚════██║██║     ██║   ██║██║   ██║   ██║
███████║╚██████╗╚██████╔╝╚██████╔╝   ██║
╚══════╝ ╚═════╝ ╚═════╝  ╚═════╝    ╚═╝
`) + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("  \ntag and search your files, fast\n")

// database is the open DB connection shared by all subcommands.
// It is opened in PersistentPreRunE and closed in PersistentPostRunE.
var database *db.DB

var rootCmd = &cobra.Command{
	Use:          "scout",
	Short:        "Find your files fast",
	Long:         banner,
	Version:      version,
	Args:         cobra.ArbitraryArgs,
	SilenceUsage: true,

	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		database = d
		return nil
	},

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
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(tagCmd)
	rootCmd.AddCommand(pruneCmd)

	watchCmd.AddCommand(watchAddCmd)
	watchCmd.AddCommand(watchRemoveCmd)
	watchCmd.AddCommand(watchListCmd)
	watchCmd.AddCommand(watchSyncCmd)

	tagCmd.AddCommand(tagAddCmd)
	tagCmd.AddCommand(tagRemoveCmd)
	tagCmd.AddCommand(tagListCmd)
	tagCmd.AddCommand(tagShowCmd)
}

func searchAction(query string) error {
	results, err := database.Search(query)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		fmt.Printf("No files found matching %q\n", query)
		return nil
	}

	// Prune stale paths inline.
	paths := make([]string, 0, len(results))
	for _, r := range results {
		paths = append(paths, "  "+ui.FormatPath(r.Path))
	}

	m := ui.ResultsModel{
		Title:  fmt.Sprintf("Results for %q", query),
		Items:  paths,
		Footer: fmt.Sprintf("%d files", len(paths)),
	}
	fmt.Print(m.View())
	return nil
}

func scopedSearchAction(tag, query string) error {
	results, err := database.SearchByTag(tag, query)
	if err != nil {
		return fmt.Errorf("scoped search: %w", err)
	}

	if len(results) == 0 {
		fmt.Printf("No files found in %s matching %q\n", ui.FormatTag(tag), query)
		return nil
	}

	paths := make([]string, 0, len(results))
	for _, r := range results {
		paths = append(paths, "  "+ui.FormatPath(r.Path))
	}

	m := ui.ResultsModel{
		Title:  fmt.Sprintf("Results for %s %q", tag, query),
		Items:  paths,
		Footer: fmt.Sprintf("%d files", len(paths)),
	}
	fmt.Print(m.View())
	return nil
}
