package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/spf13/cobra"
)

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
