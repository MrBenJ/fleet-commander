package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/teknal/fleet-commander/internal/state"
)

var hintCmd = &cobra.Command{
	Use:   "hint",
	Short: "Show keyboard shortcuts and workflow tips",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(`
Fleet Commander - Quick Reference
══════════════════════════════════

  WORKFLOW
  ─────────────────────────────────────────
  fleet queue          Open agent selector
  fleet attach <name>  Jump into an agent
  fleet list           Show all agents

  INSIDE A TMUX SESSION
  ─────────────────────────────────────────
  Ctrl+B, Q            Detach → back to shell (agent keeps running)
  Ctrl+B, D            Detach (standard tmux)
  Ctrl+B, L            List all fleet sessions

  TYPICAL LOOP
  ─────────────────────────────────────────
  1. fleet queue       → pick an agent
  2. Give Claude a task
  3. Ctrl+B, Q         → detach (Claude keeps working)
  4. fleet queue       → pick another agent
  5. Repeat!
`)
	},
}

var signalCmd = &cobra.Command{
	Use:    "signal [state]",
	Short:  "Write agent state (called by Claude Code hooks)",
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		stateFile := os.Getenv("FLEET_STATE_FILE")
		agentName := os.Getenv("FLEET_AGENT_NAME")
		if stateFile == "" || agentName == "" {
			return nil // not in a fleet session — silently succeed
		}
		return state.Write(stateFile, agentName, args[0])
	},
}
