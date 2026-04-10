package driver

import (
	"fmt"
	"sort"

	"github.com/MrBenJ/fleet-commander/internal/fleet"
)

var drivers = map[string]Driver{
	"claude-code": &ClaudeCodeDriver{},
	"codex":       &CodexDriver{},
	"aider":       &AiderDriver{},
	// "generic" is not a singleton — it's constructed per-agent via GetForAgent.
}

// Get returns the driver with the given name.
// An empty name defaults to "claude-code".
// For "generic" drivers, use GetForAgent instead (requires agent config).
func Get(name string) (Driver, error) {
	if name == "" {
		name = "claude-code"
	}
	if name == "generic" {
		return nil, fmt.Errorf("generic driver requires agent-specific config; use GetForAgent instead")
	}
	d, ok := drivers[name]
	if !ok {
		return nil, fmt.Errorf("unknown driver %q (available: %v)", name, Available())
	}
	return d, nil
}

// GetForAgent returns the driver for an agent, constructing a GenericDriver
// with agent-specific config when needed.
func GetForAgent(agent *fleet.Agent) (Driver, error) {
	if agent.Driver == "generic" {
		if agent.DriverConfig == nil {
			return nil, fmt.Errorf("generic driver requires driver_config on the agent")
		}
		config, err := ParseGenericConfig(agent.DriverConfig)
		if err != nil {
			return nil, fmt.Errorf("invalid driver_config: %w", err)
		}
		return NewGenericDriver(config), nil
	}
	return Get(agent.Driver)
}

// Available returns a sorted list of registered driver names.
func Available() []string {
	names := make([]string, 0, len(drivers)+1)
	for name := range drivers {
		names = append(names, name)
	}
	names = append(names, "generic")
	sort.Strings(names)
	return names
}
