package execx

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// Runner executes external commands with a consistent timeout and error shape.
type Runner interface {
	Run(ctx context.Context, opts Options) error
	Output(ctx context.Context, opts Options) ([]byte, error)
	CombinedOutput(ctx context.Context, opts Options) ([]byte, error)
	LookPath(file string) (string, error)
}

type Options struct {
	Name   string
	Args   []string
	Dir    string
	Stdout io.Writer
	Stderr io.Writer
}

type CommandRunner struct {
	Timeout time.Duration
}

func NewRunner(timeout time.Duration) *CommandRunner {
	return &CommandRunner{Timeout: timeout}
}

func DefaultRunner() *CommandRunner {
	return NewRunner(2 * time.Minute)
}

func (r *CommandRunner) Run(ctx context.Context, opts Options) error {
	_, err := r.run(ctx, opts, outputRun)
	return err
}

func (r *CommandRunner) Output(ctx context.Context, opts Options) ([]byte, error) {
	return r.run(ctx, opts, outputStdout)
}

func (r *CommandRunner) CombinedOutput(ctx context.Context, opts Options) ([]byte, error) {
	return r.run(ctx, opts, outputCombined)
}

func (r *CommandRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

type outputMode int

const (
	outputRun outputMode = iota
	outputStdout
	outputCombined
)

func (r *CommandRunner) run(ctx context.Context, opts Options, mode outputMode) ([]byte, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("command name is required")
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, opts.Name, opts.Args...)
	cmd.Dir = opts.Dir
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	var out []byte
	var err error
	switch mode {
	case outputRun:
		err = cmd.Run()
	case outputStdout:
		out, err = cmd.Output()
	case outputCombined:
		out, err = cmd.CombinedOutput()
	}
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("%s timed out after %s", commandString(opts), timeout)
	}
	if err != nil {
		return out, fmt.Errorf("%s failed: %w", commandString(opts), err)
	}
	return out, nil
}

func commandString(opts Options) string {
	parts := append([]string{opts.Name}, opts.Args...)
	return strings.Join(parts, " ")
}
