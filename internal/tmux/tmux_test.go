package tmux

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// fakeCall records a single command invocation.
type fakeCall struct {
	Name string
	Args []string
}

// fakeRunner implements CommandRunner for testing.
type fakeRunner struct {
	calls      []fakeCall
	runErr     error
	outputData []byte
	outputErr  error
	lookPath   map[string]error
}

func (f *fakeRunner) Run(name string, args ...string) error {
	f.calls = append(f.calls, fakeCall{Name: name, Args: args})
	return f.runErr
}

func (f *fakeRunner) Output(name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, fakeCall{Name: name, Args: args})
	return f.outputData, f.outputErr
}

func (f *fakeRunner) RunInteractive(name string, args ...string) error {
	f.calls = append(f.calls, fakeCall{Name: name, Args: args})
	return f.runErr
}

func (f *fakeRunner) LookPath(name string) (string, error) {
	if f.lookPath != nil {
		if err, ok := f.lookPath[name]; ok {
			return "", err
		}
	}
	return "/usr/bin/" + name, nil
}

// contains checks whether a string slice contains the given item.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// --- SessionName ---

func TestSessionName_DefaultPrefix(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	got := m.SessionName("auth")
	if got != "fleet-auth" {
		t.Errorf("SessionName = %q, want %q", got, "fleet-auth")
	}
}

func TestSessionName_CustomPrefix(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("myprefix", f)
	got := m.SessionName("worker")
	if got != "myprefix-worker" {
		t.Errorf("SessionName = %q, want %q", got, "myprefix-worker")
	}
}

func TestSessionName_EmptyPrefixDefaultsToFleet(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("", f)
	got := m.SessionName("db")
	if got != "fleet-db" {
		t.Errorf("SessionName = %q, want %q", got, "fleet-db")
	}
}

// --- IsAvailable ---

func TestIsAvailable_True(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	if !m.IsAvailable() {
		t.Error("IsAvailable should be true when tmux is in PATH")
	}
}

func TestIsAvailable_False(t *testing.T) {
	f := &fakeRunner{
		lookPath: map[string]error{
			"tmux": errors.New("not found"),
		},
	}
	m := NewManagerWithRunner("fleet", f)
	if m.IsAvailable() {
		t.Error("IsAvailable should be false when tmux is not in PATH")
	}
}

// --- SessionExists ---

func TestSessionExists_True(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	if !m.SessionExists("auth") {
		t.Error("SessionExists should be true when Run returns nil")
	}
	if len(f.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(f.calls))
	}
	c := f.calls[0]
	if c.Name != "tmux" {
		t.Errorf("expected command 'tmux', got %q", c.Name)
	}
	if !contains(c.Args, "has-session") {
		t.Error("expected args to contain 'has-session'")
	}
	if !contains(c.Args, "fleet-auth") {
		t.Error("expected args to contain session name 'fleet-auth'")
	}
}

func TestSessionExists_False(t *testing.T) {
	f := &fakeRunner{runErr: errors.New("no session")}
	m := NewManagerWithRunner("fleet", f)
	if m.SessionExists("auth") {
		t.Error("SessionExists should be false when Run returns error")
	}
}

// --- KillSession ---

func TestKillSession_Args(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.KillSession("worker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(f.calls))
	}
	c := f.calls[0]
	if c.Name != "tmux" {
		t.Errorf("expected command 'tmux', got %q", c.Name)
	}
	if !contains(c.Args, "kill-session") {
		t.Error("expected args to contain 'kill-session'")
	}
	if !contains(c.Args, "fleet-worker") {
		t.Error("expected args to contain session name 'fleet-worker'")
	}
}

// --- CreateSession ---

func TestCreateSession_DefaultCommand(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.CreateSession("auth", "/tmp/worktree", nil, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First call should be the new-session command
	// (there may be a second call for source-file, best-effort)
	if len(f.calls) < 1 {
		t.Fatal("expected at least 1 call")
	}
	c := f.calls[0]
	if c.Name != "tmux" {
		t.Errorf("expected command 'tmux', got %q", c.Name)
	}
	if !contains(c.Args, "-d") {
		t.Error("expected args to contain '-d'")
	}
	if !contains(c.Args, "fleet-auth") {
		t.Error("expected args to contain session name 'fleet-auth'")
	}
	if !contains(c.Args, "/tmp/worktree") {
		t.Error("expected args to contain worktree path")
	}
	// When no command is given, tmux starts the user's default shell — no "claude" in args
	if contains(c.Args, "claude") {
		t.Error("args should not contain 'claude' when no command is given (driver handles this now)")
	}
}

func TestCreateSession_WithStateFile(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.CreateSession("auth", "/tmp/worktree", nil, "/tmp/state.json", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.calls) < 1 {
		t.Fatal("expected at least 1 call")
	}
	c := f.calls[0]
	if !contains(c.Args, "FLEET_AGENT_NAME=auth") {
		t.Error("expected args to contain FLEET_AGENT_NAME env var")
	}
	if !contains(c.Args, "FLEET_STATE_FILE=/tmp/state.json") {
		t.Error("expected args to contain FLEET_STATE_FILE env var")
	}
}

func TestCreateSession_CustomCommand(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.CreateSession("worker", "/tmp/worktree", []string{"bash", "-c", "echo hello"}, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.calls) < 1 {
		t.Fatal("expected at least 1 call")
	}
	c := f.calls[0]
	if !contains(c.Args, "bash") {
		t.Error("expected args to contain 'bash'")
	}
	if !contains(c.Args, "-c") {
		t.Error("expected args to contain '-c'")
	}
	if !contains(c.Args, "echo hello") {
		t.Error("expected args to contain 'echo hello'")
	}
	// Should NOT contain "claude" since custom command was provided
	if contains(c.Args, "claude") {
		t.Error("args should not contain 'claude' when custom command is used")
	}
}

func TestCreateSession_NoCommandStartsShell(t *testing.T) {
	// When no command is provided and claude is not in PATH,
	// CreateSession should still succeed — the driver layer handles
	// availability checks, not tmux.
	f := &fakeRunner{
		lookPath: map[string]error{
			"claude": errors.New("not found"),
		},
	}
	m := NewManagerWithRunner("fleet", f)
	err := m.CreateSession("auth", "/tmp/worktree", nil, "", "")
	if err != nil {
		t.Fatalf("expected no error when no command given (tmux uses default shell), got: %v", err)
	}
}

func TestCreateSession_WindowTitleWithSquadron(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.CreateSession("auth", "/tmp/worktree", nil, "", "my-squad")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the rename-window call
	found := false
	for _, c := range f.calls {
		if c.Name == "tmux" && len(c.Args) >= 3 && c.Args[0] == "rename-window" {
			found = true
			if c.Args[2] != "fleet-auth" {
				t.Errorf("rename-window target = %q, want %q", c.Args[2], "fleet-auth")
			}
			if c.Args[3] != "auth[my-squad]" {
				t.Errorf("window title = %q, want %q", c.Args[3], "auth[my-squad]")
			}
		}
	}
	if !found {
		t.Error("expected a rename-window call when squadronName is set")
	}
}

func TestCreateSession_WindowTitleWithoutSquadron(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.CreateSession("auth", "/tmp/worktree", nil, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the rename-window call — should use just the agent name
	found := false
	for _, c := range f.calls {
		if c.Name == "tmux" && len(c.Args) >= 3 && c.Args[0] == "rename-window" {
			found = true
			if c.Args[3] != "auth" {
				t.Errorf("window title = %q, want %q", c.Args[3], "auth")
			}
		}
	}
	if !found {
		t.Error("expected a rename-window call even without squadron name")
	}
}

// --- ListSessions ---

func TestListSessions_FiltersByPrefix(t *testing.T) {
	f := &fakeRunner{
		outputData: []byte("fleet-auth\nfleet-worker\nother-session\n"),
	}
	m := NewManagerWithRunner("fleet", f)
	sessions, err := m.ListSessions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d: %v", len(sessions), sessions)
	}
	if sessions[0] != "fleet-auth" {
		t.Errorf("sessions[0] = %q, want %q", sessions[0], "fleet-auth")
	}
	if sessions[1] != "fleet-worker" {
		t.Errorf("sessions[1] = %q, want %q", sessions[1], "fleet-worker")
	}
}

func TestListSessions_NoServerRunning(t *testing.T) {
	f := &fakeRunner{
		outputErr: errors.New("no server running on /tmp/tmux-1000/default"),
	}
	m := NewManagerWithRunner("fleet", f)
	sessions, err := m.ListSessions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected empty sessions, got %v", sessions)
	}
}

func TestListSessions_OtherError(t *testing.T) {
	f := &fakeRunner{
		outputErr: errors.New("some other error"),
	}
	m := NewManagerWithRunner("fleet", f)
	_, err := m.ListSessions()
	if err == nil {
		t.Fatal("expected error for non-'no server running' errors")
	}
}

// --- GetPID ---

func TestGetPID_ParsesOutput(t *testing.T) {
	f := &fakeRunner{
		outputData: []byte("12345\n"),
	}
	m := NewManagerWithRunner("fleet", f)
	pid, err := m.GetPID("auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != 12345 {
		t.Errorf("pid = %d, want 12345", pid)
	}
}

func TestGetPID_BadOutput(t *testing.T) {
	f := &fakeRunner{
		outputData: []byte("not-a-number\n"),
	}
	m := NewManagerWithRunner("fleet", f)
	_, err := m.GetPID("auth")
	if err == nil {
		t.Fatal("expected error for non-numeric PID output")
	}
}

func TestGetPID_OutputError(t *testing.T) {
	f := &fakeRunner{
		outputErr: errors.New("session not found"),
	}
	m := NewManagerWithRunner("fleet", f)
	_, err := m.GetPID("auth")
	if err == nil {
		t.Fatal("expected error when Output fails")
	}
}

// --- CapturePane ---

func TestCapturePane_ReturnsContent(t *testing.T) {
	f := &fakeRunner{
		outputData: []byte("line1\nline2\nline3\n"),
	}
	m := NewManagerWithRunner("fleet", f)
	content, err := m.CapturePane("auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "line1\nline2\nline3\n" {
		t.Errorf("content = %q, want %q", content, "line1\nline2\nline3\n")
	}
	if len(f.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(f.calls))
	}
	c := f.calls[0]
	if !contains(c.Args, "capture-pane") {
		t.Error("expected args to contain 'capture-pane'")
	}
	if !contains(c.Args, "fleet-auth") {
		t.Error("expected args to contain session name")
	}
	if !contains(c.Args, "-p") {
		t.Error("expected args to contain '-p'")
	}
}

func TestCapturePane_Error(t *testing.T) {
	f := &fakeRunner{
		outputErr: errors.New("session not found"),
	}
	m := NewManagerWithRunner("fleet", f)
	_, err := m.CapturePane("auth")
	if err == nil {
		t.Fatal("expected error when Output fails")
	}
}

func TestCreateSession_RejectsUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)

	badNames := []string{
		"foo:bar",
		"foo.bar",
		"../etc/passwd",
		"name with spaces",
		"",
		"-starts-with-dash",
		"has;semicolon",
		"has$dollar",
	}
	for _, name := range badNames {
		err := m.CreateSession(name, "/tmp/worktree", nil, "", "")
		if err == nil {
			t.Errorf("expected error for agent name %q, got nil", name)
		}
	}
	if len(f.calls) != 0 {
		t.Errorf("expected 0 tmux calls for invalid names, got %d", len(f.calls))
	}
}

func TestKillSession_RejectsUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.KillSession("bad;name")
	if err == nil {
		t.Fatal("expected error for unsafe agent name")
	}
	if len(f.calls) != 0 {
		t.Error("no tmux commands should execute for invalid names")
	}
}

func TestAttach_RejectsUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.Attach("bad;name")
	if err == nil {
		t.Fatal("expected error for unsafe agent name")
	}
	if len(f.calls) != 0 {
		t.Error("no tmux commands should execute for invalid names")
	}
}

func TestSendKeys_RejectsUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.SendKeys("bad;name", "echo hello")
	if err == nil {
		t.Fatal("expected error for unsafe agent name")
	}
	if len(f.calls) != 0 {
		t.Error("no tmux commands should execute for invalid names")
	}
}

func TestSessionExists_ReturnsFalseForUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	if m.SessionExists("bad;name") {
		t.Fatal("SessionExists should return false for unsafe name")
	}
	if len(f.calls) != 0 {
		t.Error("no tmux commands should execute for invalid names")
	}
}

func TestCapturePane_RejectsUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	_, err := m.CapturePane("bad;name")
	if err == nil {
		t.Fatal("expected error for unsafe agent name")
	}
	if len(f.calls) != 0 {
		t.Error("no tmux commands should execute for invalid names")
	}
}

func TestGetPID_RejectsUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	_, err := m.GetPID("bad;name")
	if err == nil {
		t.Fatal("expected error for unsafe agent name")
	}
	if len(f.calls) != 0 {
		t.Error("no tmux commands should execute for invalid names")
	}
}

func TestSwitchClient_RejectsUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.SwitchClient("bad;name")
	if err == nil {
		t.Fatal("expected error for unsafe agent name")
	}
	if len(f.calls) != 0 {
		t.Error("no tmux commands should execute for invalid names")
	}
}

// --- Attach ---

func TestAttach_Success(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.Attach("auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First call is has-session (from SessionExists), second is attach
	if len(f.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(f.calls))
	}
	c := f.calls[1]
	if c.Name != "tmux" {
		t.Errorf("expected command 'tmux', got %q", c.Name)
	}
	if !contains(c.Args, "attach") {
		t.Error("expected args to contain 'attach'")
	}
	if !contains(c.Args, "fleet-auth") {
		t.Error("expected args to contain session name 'fleet-auth'")
	}
}

func TestAttach_SessionNotExist(t *testing.T) {
	f := &fakeRunner{runErr: errors.New("no session")}
	m := NewManagerWithRunner("fleet", f)
	err := m.Attach("ghost")
	if err == nil {
		t.Fatal("expected error when session doesn't exist")
	}
}

// --- SendKeys ---

func TestSendKeys_Success(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.SendKeys("worker", "echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(f.calls))
	}
	c := f.calls[0]
	if !contains(c.Args, "send-keys") {
		t.Error("expected args to contain 'send-keys'")
	}
	if !contains(c.Args, "fleet-worker") {
		t.Error("expected args to contain session name")
	}
	if !contains(c.Args, "echo hello") {
		t.Error("expected args to contain keys")
	}
}

// --- Detach ---

func TestDetach(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.Detach()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(f.calls))
	}
	c := f.calls[0]
	if c.Name != "tmux" || !contains(c.Args, "detach") {
		t.Error("expected tmux detach command")
	}
}

// --- SwitchClient ---

func TestSwitchClient_Success(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.SwitchClient("worker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(f.calls))
	}
	c := f.calls[0]
	if !contains(c.Args, "switch-client") {
		t.Error("expected args to contain 'switch-client'")
	}
	if !contains(c.Args, "fleet-worker") {
		t.Error("expected args to contain session name")
	}
}

// --- CreateSession error propagation ---

func TestCreateSession_RunError(t *testing.T) {
	f := &fakeRunner{runErr: errors.New("tmux failed")}
	m := NewManagerWithRunner("fleet", f)
	err := m.CreateSession("worker", "/tmp/wt", nil, "", "")
	if err == nil {
		t.Fatal("expected error when Run fails")
	}
	if !strings.Contains(err.Error(), "failed to create tmux session") {
		t.Errorf("expected wrapped error message, got: %v", err)
	}
}

func TestCreateSession_WithCommandAndStateFile(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.CreateSession("worker", "/tmp/wt", []string{"bash", "-c", "echo hi"}, "/tmp/state.json", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := f.calls[0]
	// Should have both env vars and the command
	if !contains(c.Args, "FLEET_AGENT_NAME=worker") {
		t.Error("expected FLEET_AGENT_NAME env var")
	}
	if !contains(c.Args, "FLEET_STATE_FILE=/tmp/state.json") {
		t.Error("expected FLEET_STATE_FILE env var")
	}
	if !contains(c.Args, "bash") {
		t.Error("expected 'bash' in command args")
	}
}

// --- NewManager ---

func TestNewManager_DefaultPrefix(t *testing.T) {
	m := NewManager("")
	if m.SessionPrefix != "fleet" {
		t.Errorf("expected default prefix 'fleet', got %q", m.SessionPrefix)
	}
}

func TestNewManager_CustomPrefix(t *testing.T) {
	m := NewManager("custom")
	if m.SessionPrefix != "custom" {
		t.Errorf("expected prefix 'custom', got %q", m.SessionPrefix)
	}
}

// --- IsInsideTmux ---

func TestIsInsideTmux_True(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	if !IsInsideTmux() {
		t.Error("IsInsideTmux should be true when TMUX env var is set")
	}
}

func TestIsInsideTmux_False(t *testing.T) {
	// Save and unset
	orig := os.Getenv("TMUX")
	os.Unsetenv("TMUX")
	defer func() {
		if orig != "" {
			os.Setenv("TMUX", orig)
		}
	}()

	if IsInsideTmux() {
		t.Error("IsInsideTmux should be false when TMUX env var is not set")
	}
}
