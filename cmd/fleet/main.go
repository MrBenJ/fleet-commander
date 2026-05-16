package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/hangar"
	"github.com/MrBenJ/fleet-commander/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// Set via -ldflags at build time. Falls back to "dev" if unset.
var (
	version = "dev"
	commit  = "unknown"
)

var rootCmd = &cobra.Command{
	Use:     "fleet",
	Short:   "Fleet Commander - Multi-agent orchestration for AI coding",
	Version: version + " (" + commit + ")",
	Long: `Fleet Commander lets you run multiple AI coding agents in parallel,
each on different branches. When agents need input, they queue up.

Quick start:
  fleet init ~/projects/my-app
  fleet add feat-auth "feature/auth"
  fleet add bug-fix "bugfix/login" --driver codex
  fleet queue`,
}

var initCmd = &cobra.Command{
	Use:   "init [repo-path]",
	Short: "Initialize a fleet for a repository",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		shortName, _ := cmd.Flags().GetString("name")
		f, err := fleet.Init(repoPath, shortName)
		if err != nil {
			return fmt.Errorf("failed to initialize fleet: %w", err)
		}
		fmt.Printf("Fleet initialized for %s\n", f.RepoPath)
		fmt.Printf("Fleet directory: %s\n", f.FleetDir)
		fmt.Printf("Short name: %s\n", f.ShortName)
		return nil
	},
}

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Open the queue TUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		allRepos, _ := cmd.Flags().GetBool("all")

		if allRepos {
			return tui.RunMultiRepo()
		}

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		return tui.Run(f)
	},
}

var hangarCmd = &cobra.Command{
	Use:   "hangar",
	Short: "Launch the web-based squadron mission control",
	Long:  "Start a web server and open the Fleet Hangar in your browser — a visual interface for configuring, launching, and monitoring squadrons.",
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("no fleet found — run `fleet init` first: %w", err)
		}

		port, _ := cmd.Flags().GetInt("port")
		noOpen, _ := cmd.Flags().GetBool("no-open")
		devMode, _ := cmd.Flags().GetBool("dev")
		controlSquadron, _ := cmd.Flags().GetString("control")
		listen, _ := cmd.Flags().GetString("listen")

		cfg := hangar.Config{
			Port:            port,
			Listen:          listen,
			DevMode:         devMode,
			RepoPath:        f.RepoPath,
			FleetDir:        f.FleetDir,
			TmuxPrefix:      f.TmuxPrefix(),
			ControlSquadron: controlSquadron,
		}

		if !devMode {
			webFS, err := getWebFS()
			if err != nil {
				return fmt.Errorf("failed to load embedded web UI: %w", err)
			}
			cfg.WebFS = webFS
		}

		srv := hangar.NewServer(cfg)
		url := fmt.Sprintf("http://%s:%d", browserHost(listen), port)
		if controlSquadron != "" {
			url += "?squadron=" + controlSquadron
		}

		// Inherit the root signal-aware context so SIGINT/SIGTERM trigger a
		// clean shutdown of the HTTP server and background goroutines.
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		errCh := make(chan error, 1)
		go func() { errCh <- srv.Start(ctx) }()

		if !noOpen {
			openBrowser(url)
		}

		tuiModel := hangar.NewTUIModel(url)
		p := tea.NewProgram(tuiModel)

		// Feed server logs to TUI. The loop exits when LogCh closes (after
		// srv.Start returns), so no goroutine leak.
		go func() {
			for msg := range srv.LogCh {
				p.Send(hangar.LogMsg{Message: msg})
			}
		}()

		runErr := func() error {
			_, err := p.Run()
			return err
		}()

		// Trigger shutdown and wait for the server to finish so deferred
		// state writes and tmux cleanup complete before main returns.
		cancel()
		if srvErr := <-errCh; srvErr != nil && !errors.Is(srvErr, http.ErrServerClosed) {
			if runErr == nil {
				return srvErr
			}
			// Surface the original TUI error; log the secondary server error.
			fmt.Fprintf(os.Stderr, "hangar server shutdown error: %v\n", srvErr)
		}
		return runErr
	},
}

// browserHost converts the listen host into something a browser can resolve.
// "0.0.0.0" or empty becomes "localhost"; everything else passes through.
func browserHost(listen string) string {
	if listen == "" || listen == "0.0.0.0" || listen == "::" {
		return "localhost"
	}
	return listen
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	cmd.Start()
}

func init() {
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	initCmd.Flags().String("name", "", "Short name for this repo (defaults to directory name)")
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().String("driver", "claude-code", "Coding agent driver (claude-code, codex, aider, kimi-code, generic)")
	addCmd.Flags().String("command", "", "Command to run (required for --driver generic)")
	addCmd.Flags().String("prompt-flag", "", "Flag for passing prompt to command (generic driver)")
	addCmd.Flags().StringSlice("yolo-args", nil, "Extra args for yolo mode (generic driver)")
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().Bool("agent-list", false, "Print only agent names, one per line (useful for piping to xargs)")
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(queueCmd)
	rootCmd.AddCommand(launchCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(clearCmd)
	clearCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	rootCmd.AddCommand(renameCmd)
	rootCmd.AddCommand(hintCmd)
	rootCmd.AddCommand(signalCmd)
	rootCmd.AddCommand(unlockCmd)

	contextReadCmd.Flags().Bool("shared", false, "Read only the shared context section")
	contextCmd.AddCommand(contextReadCmd)
	contextCmd.AddCommand(contextWriteCmd)
	contextCmd.AddCommand(contextSetSharedCmd)
	contextCmd.AddCommand(contextLogCmd)
	contextTrimCmd.Flags().Int("keep", 500, "Number of entries to keep")
	contextTrimCmd.Flags().String("channel", "", "Trim a specific channel's log instead of the shared log")
	contextCmd.AddCommand(contextTrimCmd)
	channelCreateCmd.Flags().String("description", "", "Channel description")
	contextCmd.AddCommand(channelCreateCmd)
	contextCmd.AddCommand(channelSendCmd)
	contextCmd.AddCommand(channelReadCmd)
	contextCmd.AddCommand(channelListCmd)
	contextCmd.AddCommand(contextGlobalLogCmd)
	contextCmd.AddCommand(contextGlobalReadCmd)
	contextExportCmd.Flags().String("format", "json", "Output format: json or text")
	contextExportCmd.Flags().Bool("log-only", false, "Export only the log entries")
	contextExportCmd.Flags().StringP("output", "o", "", "Write output to file instead of stdout")
	contextCmd.AddCommand(contextExportCmd)
	contextClearCmd.Flags().Bool("all", false, "Also clear shared context and agent sections")
	contextClearCmd.Flags().StringArray("channel", nil, "Clear a specific channel's log (repeatable)")
	contextClearCmd.Flags().Bool("all-channels", false, "Clear all channel logs")
	contextClearCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	contextCmd.AddCommand(contextClearCmd)
	rootCmd.AddCommand(contextCmd)

	reposAddCmd.Flags().String("name", "", "Short name for the repo (defaults to directory name)")
	reposCmd.AddCommand(reposListCmd)
	reposCmd.AddCommand(reposAddCmd)
	reposCmd.AddCommand(reposRemoveCmd)
	rootCmd.AddCommand(reposCmd)

	listCmd.Flags().Bool("all", false, "List agents across all registered repositories")
	queueCmd.Flags().Bool("all", false, "Show agents from all registered repositories")

	removeCmd.Flags().Bool("branch", false, "Also delete the git branch")
	launchCmd.Flags().Bool("ultra-dangerous-yolo-mode", false, "Skip all reviews, pass --dangerously-skip-permissions to Claude, and instruct agents to merge on completion")
	launchCmd.Flags().Bool("i-know-what-im-doing", false, "Skip the yolo mode confirmation prompt")
	launchCmd.Flags().Bool("no-auto-merge", false, "Disable auto-merge instructions; agents stop when done and leave worktrees intact for review")
	launchCmd.Flags().Bool("use-jump-sh", false, "Include jump.sh local dev server instructions in the system prompt")

	hangarCmd.Flags().Int("port", 4242, "Port to listen on")
	hangarCmd.Flags().String("listen", hangar.DefaultListenHost, "Address to bind on (use 0.0.0.0 to expose to the LAN — not recommended)")
	hangarCmd.Flags().Bool("no-open", false, "Don't auto-open the browser")
	hangarCmd.Flags().Bool("dev", false, "Proxy to Vite dev server for hot reload")
	hangarCmd.Flags().String("control", "", "Open directly to mission control for the named squadron")
	rootCmd.AddCommand(hangarCmd)
}

// run executes the root command with a signal-aware context and returns the
// process exit code. SIGINT/SIGTERM cancel the context so background
// goroutines (HTTP server, hub poll loop) can shut down gracefully before
// the process exits.
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run())
}
