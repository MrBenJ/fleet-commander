package driver

import (
	"fmt"
	"sort"
)

var drivers = map[string]Driver{
	"claude-code": &ClaudeCodeDriver{},
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

// Available returns a sorted list of registered driver names.
func Available() []string {
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
