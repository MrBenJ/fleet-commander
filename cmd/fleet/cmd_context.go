package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"
	fleetctx "github.com/teknal/fleet-commander/internal/context"
	"github.com/teknal/fleet-commander/internal/fleet"
	"github.com/teknal/fleet-commander/internal/global"
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

var channelCreateCmd = &cobra.Command{
	Use:   "channel-create [name] [agent1] [agent2] [agent3...]",
	Short: "Create a private channel between agents",
	Long:  "Creates a named channel with fixed membership. For 2-member channels, the name is auto-set to dm-[agent1]-[agent2].",
	Args:  cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		name := args[0]
		members := args[1:]
		description, _ := cmd.Flags().GetString("description")

		resolved, err := fleetctx.CreateChannel(f.FleetDir, name, description, members)
		if err != nil {
			return fmt.Errorf("failed to create channel: %w", err)
		}
		fmt.Printf("Channel created: %s (members: %v)\n", resolved, members)
		return nil
	},
}

var channelSendCmd = &cobra.Command{
	Use:   "channel-send [channel-name] [message]",
	Short: "Send a message to a channel",
	Long:  "Appends a message to the channel's log. Must be run from within a fleet agent session and the sender must be a channel member.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := os.Getenv("FLEET_AGENT_NAME")
		if agentName == "" {
			return fmt.Errorf("must be run from within a fleet agent session (FLEET_AGENT_NAME not set)")
		}

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		if err := fleetctx.SendToChannel(f.FleetDir, args[0], agentName, args[1]); err != nil {
			return fmt.Errorf("failed to send to channel: %w", err)
		}
		fmt.Printf("Sent to '%s' as '%s'\n", args[0], agentName)
		return nil
	},
}

var channelReadCmd = &cobra.Command{
	Use:   "channel-read [channel-name]",
	Short: "Read a channel's messages",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		ctx, err := fleetctx.Load(f.FleetDir)
		if err != nil {
			return fmt.Errorf("failed to load context: %w", err)
		}

		ch, ok := ctx.Channels[args[0]]
		if !ok {
			return fmt.Errorf("channel not found: %s", args[0])
		}

		fmt.Printf("Channel: %s\n", ch.Name)
		if ch.Description != "" {
			fmt.Printf("Description: %s\n", ch.Description)
		}
		fmt.Printf("Members: %v\n", ch.Members)
		fmt.Println()

		if len(ch.Log) == 0 {
			fmt.Println("(no messages)")
		} else {
			for _, entry := range ch.Log {
				fmt.Printf("[%s] [%s] %s\n", entry.Timestamp.Format(time.RFC3339), entry.Agent, entry.Message)
			}
		}
		return nil
	},
}

var channelListCmd = &cobra.Command{
	Use:   "channel-list",
	Short: "List all channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		ctx, err := fleetctx.Load(f.FleetDir)
		if err != nil {
			return fmt.Errorf("failed to load context: %w", err)
		}

		if len(ctx.Channels) == 0 {
			fmt.Println("No channels")
			return nil
		}

		names := make([]string, 0, len(ctx.Channels))
		for name := range ctx.Channels {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			ch := ctx.Channels[name]
			desc := ch.Description
			if desc == "" {
				desc = "(no description)"
			}
			fmt.Printf("%-30s  %d members  %d messages  %s\n", ch.Name, len(ch.Members), len(ch.Log), desc)
		}
		return nil
	},
}

var contextGlobalLogCmd = &cobra.Command{
	Use:   "global-log [message]",
	Short: "Append a message to the cross-repo global log",
	Long:  "Adds an entry to the global fleet log at ~/.fleet/context.json. Includes repo and agent attribution.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := os.Getenv("FLEET_AGENT_NAME")
		if agentName == "" {
			agentName = "operator"
		}

		// Determine repo short name
		repoName := "unknown"
		f, err := fleet.Load(".")
		if err == nil && f.ShortName != "" {
			repoName = f.ShortName
		}

		if err := global.AppendGlobalLog(repoName, agentName, args[0]); err != nil {
			return fmt.Errorf("failed to append global log: %w", err)
		}
		fmt.Printf("Global log entry added by %s/%s\n", repoName, agentName)
		return nil
	},
}

var contextGlobalReadCmd = &cobra.Command{
	Use:   "global-read",
	Short: "Read the cross-repo global log",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := global.LoadGlobalContext()
		if err != nil {
			return fmt.Errorf("failed to read global context: %w", err)
		}

		if len(ctx.Log) == 0 {
			fmt.Println("(no global log entries)")
			return nil
		}

		fmt.Println("== Global Log ==")
		for _, entry := range ctx.Log {
			fmt.Printf("[%s] [%s/%s] %s\n",
				entry.Timestamp.Format(time.RFC3339),
				entry.Repo,
				entry.Agent,
				entry.Message)
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
