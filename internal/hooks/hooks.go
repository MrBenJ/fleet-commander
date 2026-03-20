package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	settingsFile    = "settings.json"
	claudeDir       = ".claude"
	cmdWaiting      = "fleet signal waiting"
	cmdWorking      = "fleet signal working"
	eventStop       = "Stop"
	eventPreToolUse = "PreToolUse"
)

// fleetHookEntries maps each event to the hook entry Fleet Commander injects.
// Each outer entry includes a "_fleet": true sentinel for unambiguous identification.
var fleetHookEntries = map[string][]map[string]interface{}{
	"Stop": {{
		"_fleet": true,
		"hooks": []map[string]interface{}{
			{"type": "command", "command": "fleet signal waiting"},
		},
	}},
	"PreToolUse": {{
		"_fleet": true,
		"hooks": []map[string]interface{}{
			{"type": "command", "command": "fleet signal working"},
		},
	}},
}

// Inject merges Fleet Commander hook entries into <worktreePath>/.claude/settings.json.
// It is idempotent and will not clobber existing hooks at those events.
func Inject(worktreePath string) error {
	settings, err := loadSettings(worktreePath)
	if err != nil {
		return err
	}

	hooksMap, err := getHooksMap(settings)
	if err != nil {
		return err
	}

	for event, fleetEntries := range fleetHookEntries {
		entries := getEventEntries(hooksMap, event)
		for _, fe := range fleetEntries {
			if !containsEntry(entries, fe) {
				entries = append(entries, fe)
			}
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

	hooksMap, err := getHooksMap(settings)
	if err != nil {
		return err
	}

	changed := false
	for _, event := range []string{eventStop, eventPreToolUse} {
		entries := getEventEntries(hooksMap, event)
		filtered := filterFleetEntries(entries)
		if len(filtered) != len(entries) {
			changed = true
		}
		hooksMap[event] = filtered
	}

	if !changed {
		return nil
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
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&settings); err != nil {
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

// getHooksMap extracts the top-level "hooks" map from settings.
// Returns an empty map if the key does not exist, or an error if it is not a map.
func getHooksMap(settings map[string]interface{}) (map[string]interface{}, error) {
	raw, exists := settings["hooks"]
	if !exists {
		return map[string]interface{}{}, nil
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("settings.json 'hooks' field is not an object (got %T)", raw)
	}
	return m, nil
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

// containsEntry returns true if entries already contains a fleet entry (identified by _fleet sentinel).
func containsEntry(entries []interface{}, _ map[string]interface{}) bool {
	for _, e := range entries {
		if m, ok := e.(map[string]interface{}); ok {
			if m["_fleet"] == true {
				return true
			}
		}
	}
	return false
}

// isFleetEntry returns true if the entry has the _fleet sentinel field set to true.
func isFleetEntry(e interface{}) bool {
	m, ok := e.(map[string]interface{})
	if !ok {
		return false
	}
	return m["_fleet"] == true
}

// filterFleetEntries removes entries that are fleet entries (identified by _fleet sentinel).
func filterFleetEntries(entries []interface{}) []interface{} {
	var result []interface{}
	for _, e := range entries {
		if isFleetEntry(e) {
			continue
		}
		result = append(result, e)
	}
	return result
}
