package cmd

import (
	"fmt"

	"github.com/pavlitoss/scout-cli/internal/pathutil"
	"github.com/pavlitoss/scout-cli/internal/ui"
	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage tags",
}

var tagAddCmd = &cobra.Command{
	Use:   "add <@tag> <path>",
	Short: "Add a tag to a file or directory",
	Args:  cobra.ExactArgs(2),
	RunE:  runTagAdd,
}

var tagRemoveCmd = &cobra.Command{
	Use:   "remove <@tag> <path>",
	Short: "Remove a tag from a file or directory",
	Args:  cobra.ExactArgs(2),
	RunE:  runTagRemove,
}

var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tags with file counts",
	RunE:  runTagList,
}

var tagShowCmd = &cobra.Command{
	Use:   "show <@tag>",
	Short: "List all files under a tag",
	Args:  cobra.ExactArgs(1),
	RunE:  runTagShow,
}

func runTagAdd(cmd *cobra.Command, args []string) error {
	tagName := args[0]
	if len(tagName) == 0 || tagName[0] != '@' {
		return fmt.Errorf("tag name must start with @")
	}

	absPath, err := pathutil.Normalize(args[1])
	if err != nil {
		return fmt.Errorf("tag add: %w", err)
	}

	isDir, err := pathutil.IsDir(absPath)
	if err != nil {
		return fmt.Errorf("tag add: %w", err)
	}

	tagID, err := database.EnsureTag(tagName)
	if err != nil {
		return fmt.Errorf("tag add: %w", err)
	}

	var count int

	if isDir {
		files, err := database.GetFilesUnderPath(absPath)
		if err != nil {
			return fmt.Errorf("tag add: %w", err)
		}
		if len(files) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "%s No tracked files found under %s\n",
				ui.StyleYellow.Render("warning:"),
				ui.FormatPath(absPath),
			)
			return nil
		}
		for _, f := range files {
			if err := database.TagFile(f.ID, tagID); err != nil {
				return fmt.Errorf("tag add: %w", err)
			}
		}
		count = len(files)
	} else {
		f, err := database.GetFileByPath(absPath)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "%s file not tracked: %s\n",
				ui.StyleYellow.Render("warning:"),
				ui.FormatPath(absPath),
			)
			return nil
		}
		if err := database.TagFile(f.ID, tagID); err != nil {
			return fmt.Errorf("tag add: %w", err)
		}
		count = 1
	}

	fmt.Printf("Tagged %d %s with %s\n",
		count,
		plural(count, "file", "files"),
		ui.FormatTag(tagName),
	)
	return nil
}

func runTagRemove(cmd *cobra.Command, args []string) error {
	tagName := args[0]
	if len(tagName) == 0 || tagName[0] != '@' {
		return fmt.Errorf("tag name must start with @")
	}

	absPath, err := pathutil.Normalize(args[1])
	if err != nil {
		return fmt.Errorf("tag remove: %w", err)
	}

	tag, err := database.GetTagByName(tagName)
	if err != nil {
		return fmt.Errorf("tag remove: %w", err)
	}

	isDir, err := pathutil.IsDir(absPath)
	if err != nil {
		return fmt.Errorf("tag remove: %w", err)
	}

	if isDir {
		if err := database.UntagFilesUnderPath(absPath, tag.ID); err != nil {
			return fmt.Errorf("tag remove: %w", err)
		}
	} else {
		f, err := database.GetFileByPath(absPath)
		if err != nil {
			return fmt.Errorf("tag remove: %w", err)
		}
		if err := database.UntagFile(f.ID, tag.ID); err != nil {
			return fmt.Errorf("tag remove: %w", err)
		}
	}

	fmt.Printf("Removed tag %s from %s\n", ui.FormatTag(tagName), ui.FormatPath(absPath))
	return nil
}

func runTagList(cmd *cobra.Command, args []string) error {
	tags, err := database.ListTags()
	if err != nil {
		return fmt.Errorf("tag list: %w", err)
	}

	if len(tags) == 0 {
		fmt.Println(ui.StyleMuted.Render("No tags yet."))
		return nil
	}

	ui.PrintHeader("Tags")
	for _, t := range tags {
		badge := ""
		if t.IsWorkspace {
			badge = "  " + ui.StyleMuted.Render("(workspace)")
		}
		fmt.Printf("  %s  %s%s\n",
			ui.FormatTag(t.Name),
			ui.StyleMuted.Render(fmt.Sprintf("%d files", t.FileCount)),
			badge,
		)
	}
	return nil
}

func runTagShow(cmd *cobra.Command, args []string) error {
	return tagShowAction(args[0])
}

func tagShowAction(tagName string) error {
	if len(tagName) == 0 || tagName[0] != '@' {
		return fmt.Errorf("tag name must start with @")
	}

	files, err := database.GetFilesByTag(tagName)
	if err != nil {
		return fmt.Errorf("tag show: %w", err)
	}

	if len(files) == 0 {
		fmt.Printf("No files found for %s\n", ui.FormatTag(tagName))
		return nil
	}

	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = "  " + ui.FormatPath(f.Path)
	}

	m := ui.ResultsModel{
		Title:  "Results for " + tagName,
		Items:  paths,
		Footer: fmt.Sprintf("%d files", len(files)),
	}
	fmt.Print(m.View())
	return nil
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
