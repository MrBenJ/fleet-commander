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
	"generic":     &GenericDriver{},
}

// Get returns the driver with the given name.
// An empty name defaults to "claude-code".
func Get(name string) (Driver, error) {
	if name == "" {
		name = "claude-code"
	}
	d, ok := drivers[name]
	if !ok {
		return nil, fmt.Errorf("unknown driver %q (available: %v)", name, Available())
	}
	return d, nil
}

// GetForAgent returns the appropriate driver for an agent. For "generic" agents,
// it constructs a GenericDriver with the agent's DriverConfig. For all other
// driver types, it delegates to Get().
func GetForAgent(agent *fleet.Agent) (Driver, error) {
	name := agent.Driver
	if name == "generic" {
		if agent.DriverConfig == nil {
			return nil, fmt.Errorf("agent %q uses generic driver but has no driver_config", agent.Name)
		}
		return NewGenericDriver(agent.DriverConfig), nil
	}
	return Get(name)
}

// Available returns a sorted list of registered driver names.
func Available() []string {
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
