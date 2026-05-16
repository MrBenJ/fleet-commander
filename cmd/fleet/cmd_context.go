package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Read and write shared context between agents",
}

var contextReadCmd = &cobra.Command{
	Use:   "read [agent-name]",
	Short: "Read shared context (optionally for a specific agent)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		ctx, err := fleetctx.Load(f.FleetDir)
		if err != nil {
			return fmt.Errorf("failed to load context: %w", err)
		}

		sharedOnly, _ := cmd.Flags().GetBool("shared")

		// Specific agent requested
		if len(args) == 1 {
			if sharedOnly {
				return fmt.Errorf("cannot use --shared with an agent name")
			}
			fmt.Print(ctx.Agents[args[0]])
			if ctx.Agents[args[0]] != "" {
				fmt.Println()
			}
			return nil
		}

		// Shared only
		if sharedOnly {
			fmt.Print(ctx.Shared)
			if ctx.Shared != "" {
				fmt.Println()
			}
			return nil
		}

		// Full dump
		if ctx.Shared != "" {
			fmt.Println("== Shared Context ==")
			fmt.Println(ctx.Shared)
			fmt.Println()
		}
		names := make([]string, 0, len(ctx.Agents))
		for name := range ctx.Agents {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Printf("== %s ==\n", name)
			fmt.Println(ctx.Agents[name])
			fmt.Println()
		}

		// Agent Log section
		if len(ctx.Log) > 0 {
			fmt.Println("== Agent Log ==")
			for _, entry := range ctx.Log {
				fmt.Printf("[%s] [%s] %s\n", entry.Timestamp.Format(time.RFC3339), entry.Agent, entry.Message)
			}
			fmt.Println()
		}
		return nil
	},
}

var contextWriteCmd = &cobra.Command{
	Use:   "write [message]",
	Short: "Write to your agent's context section",
	Long:  "Updates this agent's section in shared context. Must be run from within a fleet agent session (FLEET_AGENT_NAME must be set).",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := os.Getenv("FLEET_AGENT_NAME")
		if agentName == "" {
			return fmt.Errorf("must be run from within a fleet agent session (FLEET_AGENT_NAME not set)")
		}

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		if err := fleetctx.WriteAgent(f.FleetDir, agentName, args[0]); err != nil {
			return fmt.Errorf("failed to write context: %w", err)
		}
		fmt.Printf("Updated context for agent '%s'\n", agentName)
		return nil
	},
}

var contextLogCmd = &cobra.Command{
	Use:   "log [message]",
	Short: "Append a message to the shared agent log",
	Long:  "Adds an attributed entry to the shared agent log. Must be run from within a fleet agent session (FLEET_AGENT_NAME must be set).",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := os.Getenv("FLEET_AGENT_NAME")
		if agentName == "" {
			return fmt.Errorf("must be run from within a fleet agent session (FLEET_AGENT_NAME not set)")
		}

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		if err := fleetctx.AppendLog(f.FleetDir, agentName, args[0]); err != nil {
			return fmt.Errorf("failed to append log: %w", err)
		}
		fmt.Printf("Logged by '%s'\n", agentName)
		return nil
	},
}

var contextTrimCmd = &cobra.Command{
	Use:   "trim",
	Short: "Trim the shared log or a channel log to the most recent entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		keep, _ := cmd.Flags().GetInt("keep")
		channelName, _ := cmd.Flags().GetString("channel")

		if channelName != "" {
			if err := fleetctx.TrimChannel(f.FleetDir, channelName, keep); err != nil {
				return fmt.Errorf("failed to trim channel: %w", err)
			}
			ctx, err := fleetctx.Load(f.FleetDir)
			if err != nil {
				return fmt.Errorf("failed to load context: %w", err)
			}
			ch := ctx.Channels[channelName]
			fmt.Printf("Channel '%s' trimmed: %d entries remain\n", channelName, len(ch.Log))
			return nil
		}

		// Trim shared log
		ctx, err := fleetctx.Load(f.FleetDir)
		if err != nil {
			return fmt.Errorf("failed to load context: %w", err)
		}
		before := len(ctx.Log)
		if before <= keep {
			fmt.Println("Log already within limit")
			return nil
		}

		if err := fleetctx.TrimLog(f.FleetDir, keep); err != nil {
			return fmt.Errorf("failed to trim log: %w", err)
		}
		after := keep
		if before < keep {
			after = before
		}
		fmt.Printf("Log trimmed: %d entries remain\n", after)
		return nil
	},
}

var contextClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear log entries (and optionally more) from the context store",
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		clearAll, _ := cmd.Flags().GetBool("all")
		channels, _ := cmd.Flags().GetStringArray("channel")
		allChannels, _ := cmd.Flags().GetBool("all-channels")
		yes, _ := cmd.Flags().GetBool("yes")

		if !yes {
			// Build a summary of what will be deleted
			ctx, err := fleetctx.Load(f.FleetDir)
			if err != nil {
				return fmt.Errorf("failed to load context: %w", err)
			}

			fmt.Println("The following will be cleared:")
			fmt.Printf("  - Log: %d entries\n", len(ctx.Log))
			if clearAll {
				fmt.Printf("  - Shared context: %q\n", ctx.Shared)
				fmt.Printf("  - Agent sections: %d agents\n", len(ctx.Agents))
			}
			if allChannels {
				for name, ch := range ctx.Channels {
					fmt.Printf("  - Channel %q log: %d entries\n", name, len(ch.Log))
				}
			} else {
				for _, name := range channels {
					ch, ok := ctx.Channels[name]
					if !ok {
						return fmt.Errorf("channel not found: %s", name)
					}
					fmt.Printf("  - Channel %q log: %d entries\n", name, len(ch.Log))
				}
			}

			fmt.Print("Proceed? [y/N] ")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			answer := strings.TrimSpace(scanner.Text())
			if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
				fmt.Println("Aborted.")
				return nil
			}
		}

		opts := fleetctx.ClearOptions{
			All:         clearAll,
			Channels:    channels,
			AllChannels: allChannels,
		}
		result, err := fleetctx.ClearContext(f.FleetDir, opts)
		if err != nil {
			return fmt.Errorf("failed to clear context: %w", err)
		}

		fmt.Printf("Cleared %d log entries\n", result.LogCleared)
		if result.SharedCleared {
			fmt.Println("Cleared shared context")
		}
		if result.AgentsCleared > 0 {
			fmt.Printf("Cleared %d agent sections\n", result.AgentsCleared)
		}
		for _, name := range result.ChannelsCleared {
			fmt.Printf("Cleared channel %q log\n", name)
		}
		return nil
	},
}

var contextSetSharedCmd = &cobra.Command{
	Use:   "set-shared [message]",
	Short: "Set the shared context section",
	Long:  "Updates the shared context section. Cannot be run from within a fleet agent session.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := os.Getenv("FLEET_AGENT_NAME")
		if agentName != "" {
			return fmt.Errorf("shared context can only be set from outside agent sessions (FLEET_AGENT_NAME is set to '%s')", agentName)
		}

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		if err := fleetctx.WriteShared(f.FleetDir, args[0]); err != nil {
			return fmt.Errorf("failed to write shared context: %w", err)
		}
		fmt.Println("Updated shared context")
		return nil
	},
}
