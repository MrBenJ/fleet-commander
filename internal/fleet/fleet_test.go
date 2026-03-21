//go:build integration

package fleet_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// findModuleRoot walks up from the current directory to find go.mod.
func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")
	run("git", "commit", "--allow-empty", "-m", "init")
	return dir
}

func TestConcurrentAddAgentNoLostWrites(t *testing.T) {
	repoDir := initTestRepo(t)

	// Build the fleet binary so we can run it as a subprocess.
	// We need to find the module root since this test runs from internal/fleet/.
	modRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("could not find module root: %v", err)
	}
	binaryDir := t.TempDir()
	binaryPath := filepath.Join(binaryDir, "fleet")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/fleet/")
	buildCmd.Dir = modRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	// Initialize the fleet using the binary.
	initCmd := exec.Command(binaryPath, "init", repoDir)
	initCmd.Dir = repoDir
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("fleet init failed: %v\n%s", err, out)
	}

	// Launch N concurrent subprocesses, each adding a different agent.
	// syscall.Flock protects across processes — this tests the actual lock.
	const n = 5
	var wg sync.WaitGroup
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmd := exec.Command(binaryPath, "add",
				fmt.Sprintf("agent%d", i),
				fmt.Sprintf("feature/branch%d", i),
			)
			cmd.Dir = repoDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				errs[i] = fmt.Errorf("fleet add agent%d: %v\noutput: %s", i, err, out)
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("subprocess %d failed: %v", i, err)
		}
	}

	// All n agents must be in the config — no lost writes.
	data, err := os.ReadFile(filepath.Join(repoDir, ".fleet", "config.json"))
	if err != nil {
		t.Fatalf("could not read config.json: %v", err)
	}
	var config struct {
		Agents []struct{ Name string }
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("could not parse config.json: %v\ncontent: %s", err, data)
	}
	if len(config.Agents) != n {
		t.Errorf("expected %d agents after concurrent adds, got %d — possible lost write\nconfig: %s",
			n, len(config.Agents), data)
	}
}
