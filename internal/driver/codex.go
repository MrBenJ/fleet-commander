package driver

// CodexDriver is a stub for the Codex CLI driver.
// The full implementation is provided by the codex-driver agent.
type CodexDriver struct{}

func (d *CodexDriver) Name() string                        { return "codex" }
func (d *CodexDriver) InteractiveCommand() []string         { return []string{"codex"} }
func (d *CodexDriver) BuildCommand(opts LaunchOpts) string  { return "" }
func (d *CodexDriver) PlanCommand(prompt string) ([]byte, error) { return nil, nil }
func (d *CodexDriver) DetectState(bottomLines []string, fullContent string) *AgentState {
	return nil
}
func (d *CodexDriver) InjectHooks(worktreePath string) error  { return nil }
func (d *CodexDriver) RemoveHooks(worktreePath string) error  { return nil }
func (d *CodexDriver) CheckAvailable() error                  { return nil }
