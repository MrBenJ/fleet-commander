package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/teknal/fleet-commander/internal/fleet"
	"github.com/teknal/fleet-commander/internal/global"
	"github.com/teknal/fleet-commander/internal/hooks"
	"github.com/teknal/fleet-commander/internal/tmux"
)

var addCmd = &cobra.Command{
	Use:   "add [name] [branch]",
	Short: "Add a new agent workspace",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		branch := args[1]

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		agent, err := f.AddAgent(name, branch)
		if err != nil {
			return fmt.Errorf("failed to add agent: %w", err)
		}

		fmt.Printf("Agent '%s' created on branch '%s'\n", agent.Name, agent.Branch)
		fmt.Printf("Worktree: %s\n", agent.WorktreePath)
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agents in the fleet",
	RunE: func(cmd *cobra.Command, args []string) error {
		allRepos, _ := cmd.Flags().GetBool("all")
		agentList, _ := cmd.Flags().GetBool("agent-list")

		if allRepos {
			return listAllRepos(agentList)
		}

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		if len(f.Agents) == 0 {
			fmt.Println("No agents in fleet. Use 'fleet add' to create one.")
			return nil
		}

		if agentList {
			for _, a := range f.Agents {
				fmt.Println(a.Name)
			}
			return nil
		}

		fmt.Println("AGENT\t\tBRANCH\t\t\tSTATUS\t\tHOOKS\tPID")
		fmt.Println("─────\t\t──────\t\t\t──────\t\t─────\t───")
		for _, a := range f.Agents {
			pid := "-"
			if a.PID != 0 {
				pid = fmt.Sprintf("%d", a.PID)
			}
			hooksStatus := "✗"
			if a.HooksOK {
				hooksStatus = "✓"
			}
			fmt.Printf("%-15s %-23s %-10s %-7s %s\n", a.Name, a.Branch, a.Status, hooksStatus, pid)
		}
		return nil
	},
}

func listAllRepos(agentListOnly bool) error {
	repos, err := global.List()
	if err != nil {
		return fmt.Errorf("failed to list repos: %w", err)
	}

	if len(repos) == 0 {
		fmt.Println("No repositories registered. Use 'fleet init' to initialize a fleet.")
		return nil
	}

	totalAgents := 0
	for _, r := range repos {
		f, err := fleet.Load(r.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load fleet at %s: %v\n", r.Path, err)
			continue
		}

		if agentListOnly {
			for _, a := range f.Agents {
				fmt.Printf("%s/%s\n", r.ShortName, a.Name)
			}
			continue
		}

		fmt.Printf("── %s (%s) ──\n", r.ShortName, r.Path)
		if len(f.Agents) == 0 {
			fmt.Println("  (no agents)")
		} else {
			fmt.Println("  AGENT\t\t\tBRANCH\t\t\tSTATUS\t\tHOOKS\tPID")
			for _, a := range f.Agents {
				pid := "-"
				if a.PID != 0 {
					pid = fmt.Sprintf("%d", a.PID)
				}
				hooksStatus := "✗"
				if a.HooksOK {
					hooksStatus = "✓"
				}
				fmt.Printf("  %-15s %-23s %-10s %-7s %s\n", a.Name, a.Branch, a.Status, hooksStatus, pid)
			}
		}
		totalAgents += len(f.Agents)
		fmt.Println()
	}

	if !agentListOnly {
		fmt.Printf("Total: %d repos, %d agents\n", len(repos), totalAgents)
	}
	return nil
}

var startCmd = &cobra.Command{
	Use:   "start [agent-name]",
	Short: "Start an agent's tmux session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := args[0]

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		agent, err := f.GetAgent(agentName)
		if err != nil {
			return err
		}

		tm := tmux.NewManager(f.TmuxPrefix())
		if !tm.IsAvailable() {
			return fmt.Errorf("tmux is not installed")
		}

		// Create session if it doesn't exist
		if !tm.SessionExists(agentName) {
			statesDir := filepath.Join(f.FleetDir, "states")
			if err := os.MkdirAll(statesDir, 0755); err != nil {
				return fmt.Errorf("failed to create states dir: %w", err)
			}
			stateFilePath := filepath.Join(statesDir, agentName+".json")

			if err := hooks.Inject(agent.WorktreePath); err != nil {
				// Non-fatal: common cause is malformed existing .claude/settings.json — check that file first.
				fmt.Printf("Warning: could not inject hooks into %s (.claude/settings.json may be malformed): %v\n", agent.WorktreePath, err)
				stateFilePath = ""
				f.UpdateAgentHooks(agentName, false)
			} else {
				f.UpdateAgentHooks(agentName, true)
			}

			if err := tm.CreateSession(agentName, agent.WorktreePath, nil, stateFilePath); err != nil {
				return fmt.Errorf("failed to create tmux session: %w", err)
			}
			fmt.Printf("Created tmux session for agent '%s'\n", agentName)

			f.UpdateAgentStateFile(agentName, stateFilePath)
		}

		// Update agent status
		pid, err := tm.GetPID(agentName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not get PID for agent '%s': %v\n", agentName, err)
		}
		f.UpdateAgent(agentName, "running", pid)

		fmt.Printf("Agent '%s' is running in tmux session: %s\n", agentName, tm.SessionName(agentName))
		fmt.Printf("Attach with: fleet attach %s\n", agentName)
		return nil
	},
}

var attachCmd = &cobra.Command{
	Use:   "attach [agent-name]",
	Short: "Attach to an agent's tmux session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := args[0]

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		tm := tmux.NewManager(f.TmuxPrefix())
		if !tm.IsAvailable() {
			return fmt.Errorf("tmux is not installed")
		}

		if !tm.SessionExists(agentName) {
			return fmt.Errorf("agent '%s' does not have a running session. Start it with 'fleet start %s'", agentName, agentName)
		}

		// If already in tmux, switch clients
		if tmux.IsInsideTmux() {
			return tm.SwitchClient(agentName)
		}

		// Otherwise attach to the session
		return tm.Attach(agentName)
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop [agent-name]",
	Short: "Stop an agent's tmux session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := args[0]

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		tm := tmux.NewManager(f.TmuxPrefix())
		if !tm.IsAvailable() {
			return fmt.Errorf("tmux is not installed")
		}

		if !tm.SessionExists(agentName) {
			return fmt.Errorf("agent '%s' does not have a running session", agentName)
		}

		if err := tm.KillSession(agentName); err != nil {
			return fmt.Errorf("failed to stop session: %w", err)
		}

		// Clean up state file so monitor doesn't show stale state
		agent, err := f.GetAgent(agentName)
		if err == nil && agent.StateFilePath != "" {
			if err := os.Remove(agent.StateFilePath); err != nil {
				fmt.Printf("Warning: could not remove state file: %v\n", err)
			}
			f.UpdateAgentStateFile(agentName, "")
		}

		// Remove fleet hooks so they don't fire after the session ends
		if err := hooks.Remove(agent.WorktreePath); err != nil {
			fmt.Printf("Warning: could not remove hooks: %v\n", err)
		}
		f.UpdateAgentHooks(agentName, false)

		f.UpdateAgent(agentName, "stopped", 0)
		fmt.Printf("Stopped agent '%s'\n", agentName)
		return nil
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove [agent-name]",
	Short: "Remove an agent and clean up its worktree",
	Long: `Remove an agent, kill its tmux session, and clean up the git worktree.

The branch is NOT deleted — it stays in git for PRs and review.
Use --branch to also delete the branch.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := args[0]
		deleteBranch, _ := cmd.Flags().GetBool("branch")

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		agent, err := f.GetAgent(agentName)
		if err != nil {
			return err
		}

		// Kill tmux session if running
		tm := tmux.NewManager(f.TmuxPrefix())
		if tm.SessionExists(agentName) {
			fmt.Printf("Killing tmux session for '%s'...\n", agentName)
			tm.KillSession(agentName)
		}

		// Remove fleet hooks from the worktree
		if err := hooks.Remove(agent.WorktreePath); err != nil {
			fmt.Printf("Warning: could not remove hooks: %v\n", err)
		}

		// Remove state file if present
		if agent.StateFilePath != "" {
			if err := os.Remove(agent.StateFilePath); err != nil {
				fmt.Printf("Warning: could not remove state file: %v\n", err)
			}
		}

		// Remove git worktree — best effort. The directory may exist but not
		// be a valid worktree (e.g. already pruned with leftover files).
		fmt.Printf("Removing worktree at %s...\n", agent.WorktreePath)
		removeWorktree := exec.Command("git", "worktree", "remove", agent.WorktreePath)
		removeWorktree.Dir = f.RepoPath
		if _, err := removeWorktree.CombinedOutput(); err != nil {
			// Try force remove
			forceRemove := exec.Command("git", "worktree", "remove", "--force", agent.WorktreePath)
			forceRemove.Dir = f.RepoPath
			if _, err2 := forceRemove.CombinedOutput(); err2 != nil {
				// Git doesn't recognize it as a worktree — just remove the leftover directory
				if removeErr := os.RemoveAll(agent.WorktreePath); removeErr != nil {
					fmt.Printf("Warning: could not clean up %s: %v\n", agent.WorktreePath, removeErr)
				}
			}
		}

		// Optionally delete branch
		if deleteBranch {
			fmt.Printf("Deleting branch '%s'...\n", agent.Branch)
			deleteBr := exec.Command("git", "branch", "-D", agent.Branch)
			deleteBr.Dir = f.RepoPath
			if out, err := deleteBr.CombinedOutput(); err != nil {
				fmt.Printf("Warning: could not delete branch: %s\n", string(out))
			}
		}

		// Remove agent from fleet config
		if err := f.RemoveAgent(agentName); err != nil {
			return err
		}

		fmt.Printf("✅ Agent '%s' removed\n", agentName)
		if !deleteBranch {
			fmt.Printf("Branch '%s' preserved (push it for a PR, or use --branch to delete)\n", agent.Branch)
		}
		return nil
	},
}

var renameCmd = &cobra.Command{
	Use:   "rename [old-name] [new-name]",
	Short: "Rename an agent and its worktree",
	Long:  `Rename an agent, moving its git worktree and updating fleet config. The agent must be stopped first.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName := args[0]
		newName := args[1]

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		// Require the agent to be stopped (no active tmux session)
		tm := tmux.NewManager(f.TmuxPrefix())
		if tm.SessionExists(oldName) {
			return fmt.Errorf("agent '%s' has a running tmux session — stop it first with 'fleet stop %s'", oldName, oldName)
		}

		if err := f.RenameAgent(oldName, newName); err != nil {
			return fmt.Errorf("failed to rename agent: %w", err)
		}

		agent, _ := f.GetAgent(newName)
		fmt.Printf("Renamed agent '%s' → '%s'\n", oldName, newName)
		fmt.Printf("Worktree: %s\n", agent.WorktreePath)
		return nil
	},
}
