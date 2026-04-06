package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/teknal/fleet-commander/internal/fleet"
	"github.com/teknal/fleet-commander/internal/tui"
)

// Set via -ldflags at build time. Falls back to "dev" if unset.
var (
	version = "dev"
	commit  = "unknown"
)

var rootCmd = &cobra.Command{
	Use:     "fleet",
	Short:   "Fleet Commander - Multi-agent orchestration for AI coding",
	Version: version + " (" + commit + ")",
	Long: `Fleet Commander lets you run multiple Claude Code instances in parallel,
each on different branches. When agents need input, they queue up.

Quick start:
  fleet init ~/projects/my-app
  fleet add feat-auth "feature/auth"
  fleet add bug-fix "bugfix/login"
  fleet queue`,
}

var initCmd = &cobra.Command{
	Use:   "init [repo-path]",
	Short: "Initialize a fleet for a repository",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		f, err := fleet.Init(repoPath)
		if err != nil {
			return fmt.Errorf("failed to initialize fleet: %w", err)
		}
		fmt.Printf("Fleet initialized for %s\n", f.RepoPath)
		fmt.Printf("Fleet directory: %s\n", f.FleetDir)
		return nil
	},
}

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Open the queue TUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		return tui.Run(f)
	},
}

func init() {
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().Bool("agent-list", false, "Print only agent names, one per line (useful for piping to xargs)")
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(queueCmd)
	rootCmd.AddCommand(launchCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(renameCmd)
	rootCmd.AddCommand(hintCmd)
	rootCmd.AddCommand(signalCmd)

	contextReadCmd.Flags().Bool("shared", false, "Read only the shared context section")
	contextCmd.AddCommand(contextReadCmd)
	contextCmd.AddCommand(contextWriteCmd)
	contextCmd.AddCommand(contextSetSharedCmd)
	contextCmd.AddCommand(contextLogCmd)
	contextTrimCmd.Flags().Int("keep", 500, "Number of entries to keep")
	contextTrimCmd.Flags().String("channel", "", "Trim a specific channel's log instead of the shared log")
	contextCmd.AddCommand(contextTrimCmd)
	channelCreateCmd.Flags().String("description", "", "Channel description")
	contextCmd.AddCommand(channelCreateCmd)
	contextCmd.AddCommand(channelSendCmd)
	contextCmd.AddCommand(channelReadCmd)
	contextCmd.AddCommand(channelListCmd)
	rootCmd.AddCommand(contextCmd)

	removeCmd.Flags().Bool("branch", false, "Also delete the git branch")
	launchCmd.Flags().Bool("ultra-dangerous-yolo-mode", false, "Skip all reviews, pass --dangerously-skip-permissions to Claude, and instruct agents to merge on completion")
	launchCmd.Flags().Bool("i-know-what-im-doing", false, "Skip the yolo mode confirmation prompt")
	launchCmd.Flags().Bool("no-auto-merge", false, "Disable auto-merge instructions; agents stop when done and leave worktrees intact for review")
	launchCmd.Flags().Bool("use-jump-sh", false, "Include jump.sh local dev server instructions in the system prompt")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
