package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/teknal/fleet-commander/internal/global"
)

var reposCmd = &cobra.Command{
	Use:   "repos",
	Short: "Manage registered repositories in the global fleet index",
}

var reposListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered fleet repositories",
	RunE: func(cmd *cobra.Command, args []string) error {
		repos, err := global.List()
		if err != nil {
			return fmt.Errorf("failed to list repos: %w", err)
		}

		if len(repos) == 0 {
			fmt.Println("No repositories registered. Use 'fleet init' to initialize a fleet.")
			return nil
		}

		fmt.Println("NAME\t\t\tPATH\t\t\t\t\t\tADDED")
		fmt.Println("────\t\t\t────\t\t\t\t\t\t─────")
		for _, r := range repos {
			fmt.Printf("%-20s %-45s %s\n", r.ShortName, r.Path, r.AddedAt.Format(time.RFC3339))
		}
		return nil
	},
}

var reposAddCmd = &cobra.Command{
	Use:   "add [repo-path]",
	Short: "Register an existing fleet repo in the global index",
	Long:  "Register a repo that already has .fleet/ initialized. Use --name to set a short name.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		shortName, _ := cmd.Flags().GetString("name")

		resolved, err := global.Register(repoPath, shortName)
		if err != nil {
			return fmt.Errorf("failed to register repo: %w", err)
		}
		fmt.Printf("Registered repo '%s' at %s\n", resolved, repoPath)
		return nil
	},
}

var reposRemoveCmd = &cobra.Command{
	Use:   "remove [path-or-name]",
	Short: "Unregister a repo from the global index",
	Long:  "Remove a repo from the global fleet index. Does not delete the fleet directory or any data.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := global.Unregister(args[0]); err != nil {
			return fmt.Errorf("failed to unregister repo: %w", err)
		}
		fmt.Printf("Unregistered '%s' from global index\n", args[0])
		return nil
	},
}
