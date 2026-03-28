package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/teknal/fleet-commander/internal/fleet"
	"github.com/teknal/fleet-commander/internal/tui"
)

var launchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch multiple agents from a list of prompts",
	Long: `Enter a list of tasks or prompts, review auto-generated agent names
and branches, then launch them all as parallel Claude Code sessions.

Each prompt becomes a separate agent with its own git worktree.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		yoloMode, _ := cmd.Flags().GetBool("ultra-dangerous-yolo-mode")
		skipYoloConfirm, _ := cmd.Flags().GetBool("i-know-what-im-doing")
		if skipYoloConfirm && !yoloMode {
			return fmt.Errorf("--i-know-what-im-doing requires --ultra-dangerous-yolo-mode")
		}
		return tui.RunLaunch(f, yoloMode, skipYoloConfirm)
	},
}
