package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pavlitoss/scout-cli/internal/config"
	"github.com/pavlitoss/scout-cli/internal/scanner"
	"github.com/pavlitoss/scout-cli/internal/ui"
	"github.com/pavlitoss/scout-cli/internal/pathutil"
	"github.com/spf13/cobra"
)

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

func runWatchAdd(cmd *cobra.Command, args []string) error {
	absPath, err := pathutil.Normalize(args[0])
	if err != nil {
		return fmt.Errorf("watch add: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("watch add: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("watch add: %s is not a directory", absPath)
	}

	name := filepath.Base(absPath)

	if err := database.AddWorkspace(absPath, name); err != nil {
		return fmt.Errorf("watch add: %w", err)
	}

	ignorePatterns, err := scanner.LoadIgnoreFile(absPath)
	if err != nil {
		return fmt.Errorf("watch add: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("watch add: %w", err)
	}

	opts := scanner.Options{
		ExtraDirs:       cfg.Ignore.Dirs,
		ExtraExtensions: cfg.Ignore.Extensions,
		IgnorePatterns:  ignorePatterns,
	}

	results, err := scanner.ScanDir(absPath, opts)
	if err != nil {
		return fmt.Errorf("watch add: scan: %w", err)
	}

	for _, r := range results {
		if _, err := database.UpsertFile(r.Path, r.Name, r.Preview); err != nil {
			return fmt.Errorf("watch add: upsert file: %w", err)
		}
	}

	tagName := "@" + name
	tagID, err := database.EnsureTag(tagName)
	if err != nil {
		return fmt.Errorf("watch add: ensure tag: %w", err)
	}

	if err := database.TagFilesUnderPath(absPath, tagID); err != nil {
		return fmt.Errorf("watch add: tag files: %w", err)
	}

	fmt.Printf("Registered workspace %s (%d files)\n",
		ui.FormatTag(tagName),
		len(results),
	)
	return nil
}

func runWatchRemove(cmd *cobra.Command, args []string) error {
	absPath, err := pathutil.Normalize(args[0])
	if err != nil {
		return fmt.Errorf("watch remove: %w", err)
	}

	name := filepath.Base(absPath)
	tagName := "@" + name

	tag, err := database.GetTagByName(tagName)
	if err == nil {
		if err := database.UntagFilesUnderPath(absPath, tag.ID); err != nil {
			return fmt.Errorf("watch remove: untag files: %w", err)
		}
	}

	if _, err := database.DeleteFilesUnderPath(absPath); err != nil {
		return fmt.Errorf("watch remove: delete files: %w", err)
	}

	if err := database.RemoveWorkspace(absPath); err != nil {
		return fmt.Errorf("watch remove: %w", err)
	}

	fmt.Printf("Removed workspace %s\n", ui.FormatTag(tagName))
	return nil
}

func runWatchList(cmd *cobra.Command, args []string) error {
	workspaces, err := database.ListWorkspaces()
	if err != nil {
		return fmt.Errorf("watch list: %w", err)
	}

	if len(workspaces) == 0 {
		fmt.Println(ui.StyleMuted.Render("No workspaces registered. Use 'scout watch add <path>' to get started."))
		return nil
	}

	ui.PrintHeader("Workspaces")
	for _, w := range workspaces {
		fmt.Printf("  %s  %s  %s\n",
			ui.FormatTag("@"+w.Name),
			ui.FormatPath(w.Path),
			ui.StyleMuted.Render(fmt.Sprintf("%d files", w.FileCount)),
		)
	}
	return nil
}

func runWatchSync(cmd *cobra.Command, args []string) error {
	workspaces, err := database.GetAllWorkspaces()
	if err != nil {
		return fmt.Errorf("watch sync: %w", err)
	}

	if len(workspaces) == 0 {
		fmt.Println(ui.StyleMuted.Render("No workspaces to sync."))
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("watch sync: %w", err)
	}

	for _, w := range workspaces {
		ignorePatterns, err := scanner.LoadIgnoreFile(w.Path)
		if err != nil {
			return fmt.Errorf("watch sync: %w", err)
		}

		opts := scanner.Options{
			ExtraDirs:       cfg.Ignore.Dirs,
			ExtraExtensions: cfg.Ignore.Extensions,
			IgnorePatterns:  ignorePatterns,
		}

		currentResults, err := scanner.ScanDir(w.Path, opts)
		if err != nil {
			return fmt.Errorf("watch sync: scan %s: %w", w.Path, err)
		}

		existingFiles, err := database.GetFilesUnderPath(w.Path)
		if err != nil {
			return fmt.Errorf("watch sync: get files: %w", err)
		}

		currentPaths := make(map[string]scanner.Result, len(currentResults))
		for _, r := range currentResults {
			currentPaths[r.Path] = r
		}

		existingPaths := make(map[string]struct{}, len(existingFiles))
		for _, f := range existingFiles {
			existingPaths[f.Path] = struct{}{}
		}

		tagName := "@" + w.Name
		tagID, err := database.EnsureTag(tagName)
		if err != nil {
			return fmt.Errorf("watch sync: ensure tag: %w", err)
		}

		var added, removed int

		// Add new files
		for path, r := range currentPaths {
			if _, exists := existingPaths[path]; !exists {
				fileID, err := database.UpsertFile(r.Path, r.Name, r.Preview)
				if err != nil {
					return fmt.Errorf("watch sync: upsert: %w", err)
				}
				if err := database.TagFile(fileID, tagID); err != nil {
					return fmt.Errorf("watch sync: tag file: %w", err)
				}
				added++
			}
		}

		// Remove deleted files
		for _, f := range existingFiles {
			if _, exists := currentPaths[f.Path]; !exists {
				if err := database.DeleteFileByID(f.ID); err != nil {
					return fmt.Errorf("watch sync: delete: %w", err)
				}
				removed++
			}
		}

		fmt.Printf("Synced %s: %s, %s\n",
			ui.FormatTag(tagName),
			ui.StyleGreen.Render(fmt.Sprintf("+%d added", added)),
			ui.StyleYellow.Render(fmt.Sprintf("%d removed", removed)),
		)
	}
	return nil
}
