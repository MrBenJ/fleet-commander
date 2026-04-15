package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/squadron"
	"github.com/MrBenJ/fleet-commander/internal/tui"
)

var launchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch multiple agents from a list of prompts",
	Long: `Enter a list of tasks or prompts, review auto-generated agent names
and branches, then launch them all as parallel coding agent sessions.

Each prompt becomes a separate agent with its own git worktree.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		yoloMode, _ := cmd.Flags().GetBool("ultra-dangerous-yolo-mode")
		skipYoloConfirm, _ := cmd.Flags().GetBool("i-know-what-im-doing")
		noAutoMerge, _ := cmd.Flags().GetBool("no-auto-merge")
		useJumpSh, _ := cmd.Flags().GetBool("use-jump-sh")
		if skipYoloConfirm && !yoloMode {
			return fmt.Errorf("--i-know-what-im-doing requires --ultra-dangerous-yolo-mode")
		}
		if noAutoMerge && !yoloMode {
			return fmt.Errorf("--no-auto-merge requires --ultra-dangerous-yolo-mode")
		}
		return tui.RunLaunch(f, yoloMode, skipYoloConfirm, noAutoMerge, useJumpSh)
	},
}

var squadronCmd = &cobra.Command{
	Use:   "squadron",
	Short: "Launch a squadron — agents that reach consensus before merging",
	Long: `Launch a group of agents as a "squadron": they coordinate through a
fleet context channel, review each other's work, and converge onto a single
squadron/<name> branch via a designated merger agent.

Squadron mode always runs in yolo mode with per-agent auto-merge disabled.

Interactive flow:
  fleet launch squadron

Headless (hangar output):
  fleet launch squadron --data '<json>'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		dataJSON, _ := cmd.Flags().GetString("data")
		useJumpSh, _ := cmd.Flags().GetBool("use-jump-sh")

		if dataJSON != "" {
			data, errs := squadron.ParseAndValidate([]byte(dataJSON))
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintln(os.Stderr, "error:", e)
				}
				return fmt.Errorf("squadron --data validation failed (%d error(s))", len(errs))
			}
			_, err := squadron.RunHeadless(f, data)
			return err
		}

		return tui.RunSquadronLaunch(f, useJumpSh)
	},
}

func init() {
	squadronCmd.Flags().String("data", "", "JSON payload describing the full squadron (skips TUI)")
	squadronCmd.Flags().Bool("use-jump-sh", false, "Include jump.sh local dev server instructions in the system prompt")
	launchCmd.AddCommand(squadronCmd)
}
