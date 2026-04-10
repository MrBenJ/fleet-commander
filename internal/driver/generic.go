package driver

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/MrBenJ/fleet-commander/internal/fleet"
)

// GenericConfig holds the compiled configuration for a generic driver instance.
// Regex patterns are compiled once at construction for performance — state
// detection runs every 2 seconds per agent.
type GenericConfig struct {
	Command         string
	Args            []string
	YoloArgs        []string
	PromptFlag      string
	PromptFromFile  bool
	WaitingPatterns []*regexp.Regexp
	WorkingPatterns []*regexp.Regexp
}

// GenericDriver implements Driver for arbitrary terminal-based coding agents.
// It is the most flexible driver but also the least reliable for state detection,
// since it relies on user-provided regex patterns rather than built-in heuristics.
// Users can improve state detection by having their agent call
// "fleet signal waiting" and "fleet signal working" manually.
type GenericDriver struct {
	config GenericConfig
}

// NewGenericDriver creates a GenericDriver with the given config.
func NewGenericDriver(config GenericConfig) *GenericDriver {
	return &GenericDriver{config: config}
}

func (d *GenericDriver) Name() string { return "generic" }

func (d *GenericDriver) InteractiveCommand() []string {
	args := make([]string, 0, 1+len(d.config.Args))
	args = append(args, d.config.Command)
	args = append(args, d.config.Args...)
	return args
}

func (d *GenericDriver) PlanCommand(prompt string) ([]byte, error) {
	return nil, fmt.Errorf("generic driver does not support plan mode")
}

func (d *GenericDriver) BuildCommand(opts LaunchOpts) string {
	var sb strings.Builder
	sb.WriteString("#!/usr/bin/env bash\n")

	args := make([]string, len(d.config.Args))
	copy(args, d.config.Args)

	if opts.YoloMode {
		args = append(args, d.config.YoloArgs...)
	}

	if d.config.PromptFromFile {
		// Pass the prompt file path directly to the command
		if d.config.PromptFlag != "" {
			args = append(args, d.config.PromptFlag, quoteArg(opts.PromptFile))
		} else {
			args = append(args, quoteArg(opts.PromptFile))
		}
		sb.WriteString(fmt.Sprintf("exec %s %s\n", d.config.Command, strings.Join(args, " ")))
	} else {
		// Read prompt from file into a variable and pass as argument
		sb.WriteString(fmt.Sprintf("prompt=$(cat %q)\n", opts.PromptFile))
		if d.config.PromptFlag != "" {
			args = append(args, d.config.PromptFlag, "\"$prompt\"")
		} else {
			args = append(args, "\"$prompt\"")
		}
		sb.WriteString(fmt.Sprintf("exec %s %s\n", d.config.Command, strings.Join(args, " ")))
	}

	return sb.String()
}

// DetectState matches user-configured regex patterns against pane content.
// Without patterns configured, returns nil so the caller falls back to
// state file or default behavior (shows as "working").
func (d *GenericDriver) DetectState(bottomLines []string, fullContent string) *AgentState {
	if len(d.config.WaitingPatterns) == 0 && len(d.config.WorkingPatterns) == 0 {
		return nil
	}

	bottomText := strings.Join(bottomLines, "\n")

	// Check waiting patterns first (higher priority)
	for _, pat := range d.config.WaitingPatterns {
		if pat.MatchString(bottomText) {
			state := StateWaiting
			return &state
		}
	}

	for _, pat := range d.config.WorkingPatterns {
		if pat.MatchString(bottomText) {
			state := StateWorking
			return &state
		}
	}

	return nil
}

func (d *GenericDriver) InjectHooks(worktreePath string) error { return nil }
func (d *GenericDriver) RemoveHooks(worktreePath string) error { return nil }

func (d *GenericDriver) CheckAvailable() error {
	if d.config.Command == "" {
		return fmt.Errorf("generic driver requires a 'command' in driver_config")
	}
	if _, err := exec.LookPath(d.config.Command); err != nil {
		return fmt.Errorf("%s command not found in PATH", d.config.Command)
	}
	return nil
}

// quoteArg wraps a string in single quotes for shell safety.
func quoteArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// ParseGenericConfig converts a fleet.DriverConfig into a GenericConfig,
// compiling regex patterns. Returns an error if any pattern is invalid.
func ParseGenericConfig(dc *fleet.DriverConfig) (GenericConfig, error) {
	config := GenericConfig{
		Command:        dc.Command,
		Args:           dc.Args,
		YoloArgs:       dc.YoloArgs,
		PromptFlag:     dc.PromptFlag,
		PromptFromFile: dc.PromptFromFile,
	}

	for _, p := range dc.WaitingPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return config, fmt.Errorf("invalid waiting pattern %q: %w", p, err)
		}
		config.WaitingPatterns = append(config.WaitingPatterns, re)
	}
	for _, p := range dc.WorkingPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return config, fmt.Errorf("invalid working pattern %q: %w", p, err)
		}
		config.WorkingPatterns = append(config.WorkingPatterns, re)
	}

	return config, nil
}
