package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	fleetctx "github.com/teknal/fleet-commander/internal/context"
	"github.com/teknal/fleet-commander/internal/fleet"
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

// exportEnvelope wraps context output with metadata.
type exportEnvelope struct {
	ExportedAt string          `json:"exported_at"`
	FleetPath  string          `json:"fleet_path"`
	Context    *fleetctx.Context `json:"context"`
}

var contextExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the full context to JSON or human-readable text",
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		ctx, err := fleetctx.Load(f.FleetDir)
		if err != nil {
			return fmt.Errorf("failed to load context: %w", err)
		}

		logOnly, _ := cmd.Flags().GetBool("log-only")
		format, _ := cmd.Flags().GetString("format")
		outputFile, _ := cmd.Flags().GetString("output")

		var out *os.File
		if outputFile != "" {
			out, err = os.Create(outputFile)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer out.Close()
		} else {
			out = os.Stdout
		}

		if logOnly {
			if format == "text" {
				fmt.Fprintf(out, "# Agent Log (exported %s)\n", time.Now().UTC().Format(time.RFC3339))
				fmt.Fprintf(out, "# Fleet: %s\n\n", f.RepoPath)
				for _, entry := range ctx.Log {
					fmt.Fprintf(out, "[%s] [%s] %s\n", entry.Timestamp.Format(time.RFC3339), entry.Agent, entry.Message)
				}
			} else {
				env := struct {
					ExportedAt string           `json:"exported_at"`
					FleetPath  string           `json:"fleet_path"`
					Log        []fleetctx.LogEntry `json:"log"`
				}{
					ExportedAt: time.Now().UTC().Format(time.RFC3339),
					FleetPath:  f.RepoPath,
					Log:        ctx.Log,
				}
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				if err := enc.Encode(env); err != nil {
					return fmt.Errorf("failed to encode: %w", err)
				}
			}
			return nil
		}

		if format == "text" {
			fmt.Fprintf(out, "# Fleet Context Export\n")
			fmt.Fprintf(out, "# Exported: %s\n", time.Now().UTC().Format(time.RFC3339))
			fmt.Fprintf(out, "# Fleet: %s\n\n", f.RepoPath)
			if ctx.Shared != "" {
				fmt.Fprintf(out, "== Shared Context ==\n%s\n\n", ctx.Shared)
			}
			names := make([]string, 0, len(ctx.Agents))
			for name := range ctx.Agents {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				fmt.Fprintf(out, "== %s ==\n%s\n\n", name, ctx.Agents[name])
			}
			if len(ctx.Log) > 0 {
				fmt.Fprintf(out, "== Agent Log ==\n")
				for _, entry := range ctx.Log {
					fmt.Fprintf(out, "[%s] [%s] %s\n", entry.Timestamp.Format(time.RFC3339), entry.Agent, entry.Message)
				}
				fmt.Fprintln(out)
			}
			chNames := make([]string, 0, len(ctx.Channels))
			for name := range ctx.Channels {
				chNames = append(chNames, name)
			}
			sort.Strings(chNames)
			for _, name := range chNames {
				ch := ctx.Channels[name]
				fmt.Fprintf(out, "== Channel: %s ==\n", name)
				if ch.Description != "" {
					fmt.Fprintf(out, "Description: %s\n", ch.Description)
				}
				fmt.Fprintf(out, "Members: %s\n", strings.Join(ch.Members, ", "))
				for _, entry := range ch.Log {
					fmt.Fprintf(out, "[%s] [%s] %s\n", entry.Timestamp.Format(time.RFC3339), entry.Agent, entry.Message)
				}
				fmt.Fprintln(out)
			}
			return nil
		}

		// JSON (default)
		env := exportEnvelope{
			ExportedAt: time.Now().UTC().Format(time.RFC3339),
			FleetPath:  f.RepoPath,
			Context:    ctx,
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(env)
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
