package cmd

import "github.com/spf13/cobra"

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

func runTagAdd(cmd *cobra.Command, args []string) error    { return nil }
func runTagRemove(cmd *cobra.Command, args []string) error { return nil }
func runTagList(cmd *cobra.Command, args []string) error   { return nil }
func runTagShow(cmd *cobra.Command, args []string) error   { return nil }
