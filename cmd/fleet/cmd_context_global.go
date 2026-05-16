package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/global"
	"github.com/spf13/cobra"
)

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

// exportEnvelope wraps context output with metadata.
type exportEnvelope struct {
	ExportedAt string            `json:"exported_at"`
	FleetPath  string            `json:"fleet_path"`
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
			return writeLogOnlyExport(out, f.RepoPath, ctx, format)
		}
		if format == "text" {
			return writeTextExport(out, f.RepoPath, ctx)
		}

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

func writeLogOnlyExport(out *os.File, repoPath string, ctx *fleetctx.Context, format string) error {
	if format == "text" {
		fmt.Fprintf(out, "# Agent Log (exported %s)\n", time.Now().UTC().Format(time.RFC3339))
		fmt.Fprintf(out, "# Fleet: %s\n\n", repoPath)
		for _, entry := range ctx.Log {
			fmt.Fprintf(out, "[%s] [%s] %s\n", entry.Timestamp.Format(time.RFC3339), entry.Agent, entry.Message)
		}
		return nil
	}

	env := struct {
		ExportedAt string              `json:"exported_at"`
		FleetPath  string              `json:"fleet_path"`
		Log        []fleetctx.LogEntry `json:"log"`
	}{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		FleetPath:  repoPath,
		Log:        ctx.Log,
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(env); err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}
	return nil
}

func writeTextExport(out *os.File, repoPath string, ctx *fleetctx.Context) error {
	fmt.Fprintf(out, "# Fleet Context Export\n")
	fmt.Fprintf(out, "# Exported: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(out, "# Fleet: %s\n\n", repoPath)
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
