package driver

import "github.com/MrBenJ/fleet-commander/internal/fleet"

// GenericDriver is a stub for the generic/custom driver.
// The full implementation is provided by the generic-driver agent.
type GenericDriver struct {
	Config *fleet.DriverConfig
}

func NewGenericDriver(config *fleet.DriverConfig) *GenericDriver {
	return &GenericDriver{Config: config}
}

func (d *GenericDriver) Name() string                        { return "generic" }
func (d *GenericDriver) InteractiveCommand() []string         { return nil }
func (d *GenericDriver) BuildCommand(opts LaunchOpts) string  { return "" }
func (d *GenericDriver) PlanCommand(prompt string) ([]byte, error) { return nil, nil }
func (d *GenericDriver) DetectState(bottomLines []string, fullContent string) *AgentState {
	return nil
}
func (d *GenericDriver) InjectHooks(worktreePath string) error  { return nil }
func (d *GenericDriver) RemoveHooks(worktreePath string) error  { return nil }
func (d *GenericDriver) CheckAvailable() error                  { return nil }
