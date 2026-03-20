package hooks

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	settingsFile      = "settings.json"
	claudeDir         = ".claude"
	cmdWaiting        = "fleet signal waiting"
	cmdWorking        = "fleet signal working"
	eventStop         = "Stop"
	eventPreToolUse   = "PreToolUse"
)

// fleetEntries maps each event to the hook entry Fleet Commander injects.
var fleetEntries = map[string]map[string]interface{}{
	eventStop: {
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": cmdWaiting,
			},
		},
	},
	eventPreToolUse: {
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": cmdWorking,
			},
		},
	},
}

// Inject merges Fleet Commander hook entries into <worktreePath>/.claude/settings.json.
// It is idempotent and will not clobber existing hooks at those events.
func Inject(worktreePath string) error {
	settings, err := loadSettings(worktreePath)
	if err != nil {
		return err
	}

	hooksMap := getOrCreateHooksMap(settings)

	for event, entry := range fleetEntries {
		entries := getEventEntries(hooksMap, event)
		if !containsEntry(entries, entry) {
			entries = append(entries, entry)
		}
		hooksMap[event] = entries
	}

	settings["hooks"] = hooksMap
	return saveSettings(worktreePath, settings)
}

// Remove strips any Fleet Commander hook entries from <worktreePath>/.claude/settings.json.
// It is a no-op if the file does not exist.
func Remove(worktreePath string) error {
	path := settingsPath(worktreePath)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	settings, err := loadSettings(worktreePath)
	if err != nil {
		return err
	}

	hooksMap := getOrCreateHooksMap(settings)

	for _, event := range []string{eventStop, eventPreToolUse} {
		entries := getEventEntries(hooksMap, event)
		filtered := filterFleetEntries(entries)
		hooksMap[event] = filtered
	}

	settings["hooks"] = hooksMap
	return saveSettings(worktreePath, settings)
}

// settingsPath returns the full path to the settings file.
func settingsPath(worktreePath string) string {
	return filepath.Join(worktreePath, claudeDir, settingsFile)
}

// loadSettings reads and parses settings.json, creating an empty map if missing.
// Returns an error if the file exists but contains malformed JSON.
func loadSettings(worktreePath string) (map[string]interface{}, error) {
	path := settingsPath(worktreePath)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]interface{}{}, nil
	}
	if err != nil {
		return nil, err
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}
	return settings, nil
}

// saveSettings writes settings back to disk, creating the .claude dir if needed.
func saveSettings(worktreePath string, settings map[string]interface{}) error {
	dir := filepath.Join(worktreePath, claudeDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath(worktreePath), data, 0644)
}

// getOrCreateHooksMap extracts the top-level "hooks" map from settings,
// creating it if it does not exist or is not a map.
func getOrCreateHooksMap(settings map[string]interface{}) map[string]interface{} {
	if raw, ok := settings["hooks"]; ok {
		if m, ok := raw.(map[string]interface{}); ok {
			return m
		}
	}
	return map[string]interface{}{}
}

// getEventEntries returns the slice of hook entries for a given event.
func getEventEntries(hooksMap map[string]interface{}, event string) []interface{} {
	raw, ok := hooksMap[event]
	if !ok {
		return []interface{}{}
	}
	entries, ok := raw.([]interface{})
	if !ok {
		return []interface{}{}
	}
	return entries
}

// entryCommandSet returns the sorted set of "command" values from an entry's inner hooks array.
func entryCommandSet(entry map[string]interface{}) []string {
	innerRaw, ok := entry["hooks"]
	if !ok {
		return nil
	}
	inner, ok := innerRaw.([]interface{})
	if !ok {
		return nil
	}
	var cmds []string
	for _, ih := range inner {
		ihMap, ok := ih.(map[string]interface{})
		if !ok {
			continue
		}
		if cmd, ok := ihMap["command"].(string); ok {
			cmds = append(cmds, cmd)
		}
	}
	sort.Strings(cmds)
	return cmds
}

// sameCommandSet returns true if two sorted command slices are equal.
func sameCommandSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// containsEntry returns true if entries already contains an entry with the same command set.
func containsEntry(entries []interface{}, candidate map[string]interface{}) bool {
	candidateCmds := entryCommandSet(candidate)
	for _, e := range entries {
		eMap, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		if sameCommandSet(entryCommandSet(eMap), candidateCmds) {
			return true
		}
	}
	return false
}

// isFleetEntry returns true if all commands in the entry's inner hooks are fleet signal commands.
func isFleetEntry(entry map[string]interface{}) bool {
	cmds := entryCommandSet(entry)
	if len(cmds) == 0 {
		return false
	}
	for _, cmd := range cmds {
		if !strings.HasPrefix(cmd, "fleet signal") {
			return false
		}
	}
	return true
}

// filterFleetEntries removes entries that are exclusively fleet signal commands.
func filterFleetEntries(entries []interface{}) []interface{} {
	var result []interface{}
	for _, e := range entries {
		eMap, ok := e.(map[string]interface{})
		if ok && isFleetEntry(eMap) {
			continue
		}
		result = append(result, e)
	}
	return result
}
