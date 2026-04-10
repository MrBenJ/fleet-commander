package driver

// AiderDriver is a stub for the Aider driver.
// The full implementation is provided by the aider-driver agent.
type AiderDriver struct{}

func (d *AiderDriver) Name() string                        { return "aider" }
func (d *AiderDriver) InteractiveCommand() []string         { return []string{"aider"} }
func (d *AiderDriver) BuildCommand(opts LaunchOpts) string  { return "" }
func (d *AiderDriver) PlanCommand(prompt string) ([]byte, error) { return nil, nil }
func (d *AiderDriver) DetectState(bottomLines []string, fullContent string) *AgentState {
	return nil
}
func (d *AiderDriver) InjectHooks(worktreePath string) error  { return nil }
func (d *AiderDriver) RemoveHooks(worktreePath string) error  { return nil }
func (d *AiderDriver) CheckAvailable() error                  { return nil }
