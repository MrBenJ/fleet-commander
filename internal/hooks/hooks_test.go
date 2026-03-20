package hooks_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teknal/fleet-commander/internal/hooks"
)

func settingsPath(dir string) string {
	return filepath.Join(dir, ".claude", "settings.json")
}

func readSettings(t *testing.T, dir string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(settingsPath(dir))
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to parse settings.json: %v", err)
	}
	return m
}

func fleetCommandsInEvent(t *testing.T, settings map[string]interface{}, event string) []string {
	t.Helper()
	hooksRaw, ok := settings["hooks"]
	if !ok {
		return nil
	}
	hooksMap, ok := hooksRaw.(map[string]interface{})
	if !ok {
		return nil
	}
	eventRaw, ok := hooksMap[event]
	if !ok {
		return nil
	}
	entries, ok := eventRaw.([]interface{})
	if !ok {
		return nil
	}
	var cmds []string
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		innerRaw, ok := entryMap["hooks"]
		if !ok {
			continue
		}
		innerHooks, ok := innerRaw.([]interface{})
		if !ok {
			continue
		}
		for _, ih := range innerHooks {
			ihMap, ok := ih.(map[string]interface{})
			if !ok {
				continue
			}
			cmd, _ := ihMap["command"].(string)
			if strings.HasPrefix(cmd, "fleet signal") {
				cmds = append(cmds, cmd)
			}
		}
	}
	return cmds
}

// TestInjectCreatesFile verifies that Inject creates .claude/settings.json
// with Stop and PreToolUse keys when the directory is empty.
func TestInjectCreatesFile(t *testing.T) {
	dir := t.TempDir()

	if err := hooks.Inject(dir); err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	if _, err := os.Stat(settingsPath(dir)); err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}

	settings := readSettings(t, dir)
	hooksRaw, ok := settings["hooks"]
	if !ok {
		t.Fatal("settings.json missing 'hooks' key")
	}
	hooksMap, ok := hooksRaw.(map[string]interface{})
	if !ok {
		t.Fatal("'hooks' is not a map")
	}
	if _, ok := hooksMap["Stop"]; !ok {
		t.Error("missing 'Stop' key in hooks")
	}
	if _, ok := hooksMap["PreToolUse"]; !ok {
		t.Error("missing 'PreToolUse' key in hooks")
	}
}

// TestInjectIsIdempotent verifies that injecting twice does not duplicate fleet entries.
func TestInjectIsIdempotent(t *testing.T) {
	dir := t.TempDir()

	if err := hooks.Inject(dir); err != nil {
		t.Fatalf("first Inject failed: %v", err)
	}
	if err := hooks.Inject(dir); err != nil {
		t.Fatalf("second Inject failed: %v", err)
	}

	settings := readSettings(t, dir)
	cmds := fleetCommandsInEvent(t, settings, "Stop")
	count := 0
	for _, c := range cmds {
		if c == "fleet signal waiting" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 'fleet signal waiting' entry in Stop, got %d", count)
	}
}

// TestInjectPreservesExistingHooks verifies that existing Stop hooks are not clobbered.
func TestInjectPreservesExistingHooks(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create .claude dir: %v", err)
	}

	// Write existing settings with a custom Stop hook
	existing := map[string]interface{}{
		"hooks": map[string]interface{}{
			"Stop": []interface{}{
				map[string]interface{}{
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "echo done",
						},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(existing)
	if err := os.WriteFile(settingsPath(dir), data, 0644); err != nil {
		t.Fatalf("failed to write existing settings: %v", err)
	}

	if err := hooks.Inject(dir); err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	settings := readSettings(t, dir)
	hooksMap := settings["hooks"].(map[string]interface{})
	stopEntries := hooksMap["Stop"].([]interface{})
	if len(stopEntries) < 2 {
		t.Errorf("expected at least 2 Stop entries (existing + fleet), got %d", len(stopEntries))
	}
}

// TestInjectMalformedJSON verifies that Inject returns an error for malformed JSON.
func TestInjectMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create .claude dir: %v", err)
	}
	if err := os.WriteFile(settingsPath(dir), []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("failed to write malformed JSON: %v", err)
	}

	err := hooks.Inject(dir)
	if err == nil {
		t.Error("Inject should return an error for malformed JSON")
	}
}

// TestRemove verifies that Remove strips all fleet signal commands.
func TestRemove(t *testing.T) {
	dir := t.TempDir()

	if err := hooks.Inject(dir); err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	if err := hooks.Remove(dir); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	settings := readSettings(t, dir)
	for _, event := range []string{"Stop", "PreToolUse"} {
		cmds := fleetCommandsInEvent(t, settings, event)
		if len(cmds) != 0 {
			t.Errorf("expected no fleet signal commands in %s after Remove, got: %v", event, cmds)
		}
	}
}
