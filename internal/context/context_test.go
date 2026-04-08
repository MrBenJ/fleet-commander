//go:build !windows

package context_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
)

func TestLoadMissingFileReturnsEmptyContext(t *testing.T) {
	dir := t.TempDir()
	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Shared != "" {
		t.Errorf("expected empty shared, got %q", ctx.Shared)
	}
	if len(ctx.Agents) != 0 {
		t.Errorf("expected empty agents map, got %v", ctx.Agents)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	ctx := &fleetctx.Context{
		Shared: "use JWT auth",
		Agents: map[string]string{
			"auth-agent": "User model done",
		},
	}
	if err := fleetctx.Save(dir, ctx); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Shared != "use JWT auth" {
		t.Errorf("shared mismatch: got %q", loaded.Shared)
	}
	if loaded.Agents["auth-agent"] != "User model done" {
		t.Errorf("agent mismatch: got %q", loaded.Agents["auth-agent"])
	}
}

func TestWriteAgent(t *testing.T) {
	dir := t.TempDir()

	// First agent writes
	if err := fleetctx.WriteAgent(dir, "auth-agent", "User model done"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	// Second agent writes — should not clobber first
	if err := fleetctx.WriteAgent(dir, "api-agent", "Endpoints defined"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Agents["auth-agent"] != "User model done" {
		t.Errorf("auth-agent mismatch: got %q", ctx.Agents["auth-agent"])
	}
	if ctx.Agents["api-agent"] != "Endpoints defined" {
		t.Errorf("api-agent mismatch: got %q", ctx.Agents["api-agent"])
	}
}

func TestWriteShared(t *testing.T) {
	dir := t.TempDir()

	// Write an agent first
	if err := fleetctx.WriteAgent(dir, "auth-agent", "User model done"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	// Write shared — should not clobber agent
	if err := fleetctx.WriteShared(dir, "API uses JWT"); err != nil {
		t.Fatalf("WriteShared failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Shared != "API uses JWT" {
		t.Errorf("shared mismatch: got %q", ctx.Shared)
	}
	if ctx.Agents["auth-agent"] != "User model done" {
		t.Errorf("auth-agent was clobbered: got %q", ctx.Agents["auth-agent"])
	}
}

func TestLoadMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "context.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("failed to write test fixture: %v", err)
	}

	_, err := fleetctx.Load(dir)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestAppendLog(t *testing.T) {
	dir := t.TempDir()

	if err := fleetctx.AppendLog(dir, "auth-agent", "found auth bug"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Log) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(ctx.Log))
	}
	if ctx.Log[0].Agent != "auth-agent" {
		t.Errorf("agent mismatch: got %q", ctx.Log[0].Agent)
	}
	if ctx.Log[0].Message != "found auth bug" {
		t.Errorf("message mismatch: got %q", ctx.Log[0].Message)
	}
	if ctx.Log[0].Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}

func TestAppendLogPreservesExistingData(t *testing.T) {
	dir := t.TempDir()

	if err := fleetctx.WriteAgent(dir, "auth-agent", "working on auth"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	if err := fleetctx.AppendLog(dir, "api-agent", "endpoints ready"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Agents["auth-agent"] != "working on auth" {
		t.Errorf("agent data clobbered: got %q", ctx.Agents["auth-agent"])
	}
	if len(ctx.Log) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(ctx.Log))
	}
}

func TestAppendLogMultipleEntries(t *testing.T) {
	dir := t.TempDir()

	if err := fleetctx.AppendLog(dir, "agent-a", "first"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}
	if err := fleetctx.AppendLog(dir, "agent-b", "second"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Log) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(ctx.Log))
	}
	if ctx.Log[0].Message != "first" {
		t.Errorf("first entry: got %q", ctx.Log[0].Message)
	}
	if ctx.Log[1].Message != "second" {
		t.Errorf("second entry: got %q", ctx.Log[1].Message)
	}
}

func TestTrimLog(t *testing.T) {
	dir := t.TempDir()

	for i := 0; i < 10; i++ {
		if err := fleetctx.AppendLog(dir, "agent", fmt.Sprintf("msg-%d", i)); err != nil {
			t.Fatalf("AppendLog failed: %v", err)
		}
	}

	if err := fleetctx.TrimLog(dir, 3); err != nil {
		t.Fatalf("TrimLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Log) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(ctx.Log))
	}
	// Should keep the last 3
	if ctx.Log[0].Message != "msg-7" {
		t.Errorf("first kept: got %q", ctx.Log[0].Message)
	}
	if ctx.Log[2].Message != "msg-9" {
		t.Errorf("last kept: got %q", ctx.Log[2].Message)
	}
}

func TestTrimLogNoOp(t *testing.T) {
	dir := t.TempDir()

	if err := fleetctx.AppendLog(dir, "agent", "only one"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	if err := fleetctx.TrimLog(dir, 500); err != nil {
		t.Fatalf("TrimLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Log) != 1 {
		t.Fatalf("expected 1 entry (no-op), got %d", len(ctx.Log))
	}
}

func TestTrimLogClearAll(t *testing.T) {
	dir := t.TempDir()

	for i := 0; i < 5; i++ {
		if err := fleetctx.AppendLog(dir, "agent", fmt.Sprintf("msg-%d", i)); err != nil {
			t.Fatalf("AppendLog failed: %v", err)
		}
	}

	if err := fleetctx.TrimLog(dir, 0); err != nil {
		t.Fatalf("TrimLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Log) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(ctx.Log))
	}
}

func TestAppendLogEmptyMessage(t *testing.T) {
	dir := t.TempDir()
	err := fleetctx.AppendLog(dir, "agent-a", "")
	if err == nil {
		t.Fatal("expected error for empty message, got nil")
	}
}

func TestCreateChannelDM(t *testing.T) {
	dir := t.TempDir()

	name, err := fleetctx.CreateChannel(dir, "ignored", "auth discussion", []string{"alice", "bob"})
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}
	if name != "dm-[alice]-[bob]" {
		t.Errorf("expected dm-[alice]-[bob], got %q", name)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ch, ok := ctx.Channels["dm-[alice]-[bob]"]
	if !ok {
		t.Fatal("channel not found in context")
	}
	if ch.Description != "auth discussion" {
		t.Errorf("description: got %q", ch.Description)
	}
	if len(ch.Members) != 2 || ch.Members[0] != "alice" || ch.Members[1] != "bob" {
		t.Errorf("members: got %v", ch.Members)
	}
}

func TestCreateChannelGroup(t *testing.T) {
	dir := t.TempDir()

	name, err := fleetctx.CreateChannel(dir, "backend-crew", "backend sync", []string{"alice", "bob", "charlie"})
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}
	if name != "backend-crew" {
		t.Errorf("expected backend-crew, got %q", name)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ch, ok := ctx.Channels["backend-crew"]
	if !ok {
		t.Fatal("channel not found")
	}
	if len(ch.Members) != 3 {
		t.Errorf("expected 3 members, got %d", len(ch.Members))
	}
}

func TestCreateChannelDuplicate(t *testing.T) {
	dir := t.TempDir()

	_, err := fleetctx.CreateChannel(dir, "backend-crew", "first", []string{"alice", "bob", "charlie"})
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	_, err = fleetctx.CreateChannel(dir, "backend-crew", "second", []string{"alice", "bob", "charlie"})
	if err == nil {
		t.Fatal("expected error for duplicate channel, got nil")
	}
}

func TestCreateChannelTooFewMembers(t *testing.T) {
	dir := t.TempDir()
	_, err := fleetctx.CreateChannel(dir, "solo", "alone", []string{"alice"})
	if err == nil {
		t.Fatal("expected error for < 2 members, got nil")
	}
}

func TestCreateChannelEmptyMember(t *testing.T) {
	dir := t.TempDir()
	_, err := fleetctx.CreateChannel(dir, "bad", "empty member", []string{"alice", ""})
	if err == nil {
		t.Fatal("expected error for empty member name, got nil")
	}
}

func TestCreateChannelPreservesExistingData(t *testing.T) {
	dir := t.TempDir()

	if err := fleetctx.WriteShared(dir, "shared stuff"); err != nil {
		t.Fatalf("WriteShared failed: %v", err)
	}
	if err := fleetctx.AppendLog(dir, "agent", "log entry"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	_, err := fleetctx.CreateChannel(dir, "ignored", "dm", []string{"alice", "bob"})
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Shared != "shared stuff" {
		t.Errorf("shared clobbered: got %q", ctx.Shared)
	}
	if len(ctx.Log) != 1 {
		t.Errorf("log clobbered: got %d entries", len(ctx.Log))
	}
}

func TestSendToChannel(t *testing.T) {
	dir := t.TempDir()

	name, err := fleetctx.CreateChannel(dir, "ignored", "dm", []string{"alice", "bob"})
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}

	if err := fleetctx.SendToChannel(dir, name, "alice", "hey bob"); err != nil {
		t.Fatalf("SendToChannel failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ch := ctx.Channels[name]
	if len(ch.Log) != 1 {
		t.Fatalf("expected 1 message, got %d", len(ch.Log))
	}
	if ch.Log[0].Agent != "alice" {
		t.Errorf("agent: got %q", ch.Log[0].Agent)
	}
	if ch.Log[0].Message != "hey bob" {
		t.Errorf("message: got %q", ch.Log[0].Message)
	}
}

func TestSendToChannelNonMember(t *testing.T) {
	dir := t.TempDir()

	name, _ := fleetctx.CreateChannel(dir, "ignored", "dm", []string{"alice", "bob"})

	err := fleetctx.SendToChannel(dir, name, "charlie", "let me in")
	if err == nil {
		t.Fatal("expected error for non-member, got nil")
	}
}

func TestSendToChannelNotExists(t *testing.T) {
	dir := t.TempDir()
	err := fleetctx.SendToChannel(dir, "no-such-channel", "alice", "hello")
	if err == nil {
		t.Fatal("expected error for missing channel, got nil")
	}
}

func TestSendToChannelEmptyMessage(t *testing.T) {
	dir := t.TempDir()
	name, _ := fleetctx.CreateChannel(dir, "ignored", "dm", []string{"alice", "bob"})

	err := fleetctx.SendToChannel(dir, name, "alice", "")
	if err == nil {
		t.Fatal("expected error for empty message, got nil")
	}
}

func TestTrimChannel(t *testing.T) {
	dir := t.TempDir()

	name, _ := fleetctx.CreateChannel(dir, "ignored", "dm", []string{"alice", "bob"})
	for i := 0; i < 10; i++ {
		if err := fleetctx.SendToChannel(dir, name, "alice", fmt.Sprintf("msg-%d", i)); err != nil {
			t.Fatalf("SendToChannel failed: %v", err)
		}
	}

	if err := fleetctx.TrimChannel(dir, name, 3); err != nil {
		t.Fatalf("TrimChannel failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ch := ctx.Channels[name]
	if len(ch.Log) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(ch.Log))
	}
	if ch.Log[0].Message != "msg-7" {
		t.Errorf("first kept: got %q", ch.Log[0].Message)
	}
}

func TestTrimChannelNotExists(t *testing.T) {
	dir := t.TempDir()
	err := fleetctx.TrimChannel(dir, "no-such-channel", 10)
	if err == nil {
		t.Fatal("expected error for missing channel, got nil")
	}
}

func TestClearContextDefaultClearsLog(t *testing.T) {
	dir := t.TempDir()

	for i := 0; i < 5; i++ {
		if err := fleetctx.AppendLog(dir, "agent", fmt.Sprintf("msg-%d", i)); err != nil {
			t.Fatalf("AppendLog failed: %v", err)
		}
	}
	if err := fleetctx.WriteShared(dir, "stay"); err != nil {
		t.Fatalf("WriteShared failed: %v", err)
	}
	if err := fleetctx.WriteAgent(dir, "a", "keep me"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	result, err := fleetctx.ClearContext(dir, fleetctx.ClearOptions{})
	if err != nil {
		t.Fatalf("ClearContext failed: %v", err)
	}
	if result.LogCleared != 5 {
		t.Errorf("expected LogCleared=5, got %d", result.LogCleared)
	}
	if result.SharedCleared {
		t.Error("SharedCleared should be false")
	}
	if result.AgentsCleared != 0 {
		t.Errorf("expected AgentsCleared=0, got %d", result.AgentsCleared)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Log) != 0 {
		t.Errorf("expected empty log, got %d entries", len(ctx.Log))
	}
	if ctx.Shared != "stay" {
		t.Errorf("shared was clobbered: got %q", ctx.Shared)
	}
	if ctx.Agents["a"] != "keep me" {
		t.Errorf("agent was clobbered: got %q", ctx.Agents["a"])
	}
}

func TestClearContextAll(t *testing.T) {
	dir := t.TempDir()

	if err := fleetctx.WriteShared(dir, "clear me"); err != nil {
		t.Fatalf("WriteShared failed: %v", err)
	}
	if err := fleetctx.WriteAgent(dir, "a", "also clear"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}
	if err := fleetctx.WriteAgent(dir, "b", "also clear"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}
	if err := fleetctx.AppendLog(dir, "a", "log entry"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	result, err := fleetctx.ClearContext(dir, fleetctx.ClearOptions{All: true})
	if err != nil {
		t.Fatalf("ClearContext failed: %v", err)
	}
	if !result.SharedCleared {
		t.Error("expected SharedCleared=true")
	}
	if result.AgentsCleared != 2 {
		t.Errorf("expected AgentsCleared=2, got %d", result.AgentsCleared)
	}
	if result.LogCleared != 1 {
		t.Errorf("expected LogCleared=1, got %d", result.LogCleared)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Shared != "" {
		t.Errorf("shared not cleared: got %q", ctx.Shared)
	}
	if len(ctx.Agents) != 0 {
		t.Errorf("agents not cleared: got %v", ctx.Agents)
	}
	if len(ctx.Log) != 0 {
		t.Errorf("log not cleared: got %d entries", len(ctx.Log))
	}
}

func TestClearContextChannel(t *testing.T) {
	dir := t.TempDir()

	chName, err := fleetctx.CreateChannel(dir, "ignored", "dm", []string{"alice", "bob"})
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := fleetctx.SendToChannel(dir, chName, "alice", fmt.Sprintf("msg-%d", i)); err != nil {
			t.Fatalf("SendToChannel failed: %v", err)
		}
	}
	if err := fleetctx.AppendLog(dir, "agent", "shared log"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	result, err := fleetctx.ClearContext(dir, fleetctx.ClearOptions{Channels: []string{chName}})
	if err != nil {
		t.Fatalf("ClearContext failed: %v", err)
	}
	if len(result.ChannelsCleared) != 1 || result.ChannelsCleared[0] != chName {
		t.Errorf("expected channel cleared, got %v", result.ChannelsCleared)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Channels[chName].Log) != 0 {
		t.Errorf("channel log not cleared: got %d entries", len(ctx.Channels[chName].Log))
	}
	// Default behavior also clears ctx.Log
	if len(ctx.Log) != 0 {
		t.Errorf("shared log should also be cleared: got %d entries", len(ctx.Log))
	}
}

func TestClearContextAllChannels(t *testing.T) {
	dir := t.TempDir()

	ch1, _ := fleetctx.CreateChannel(dir, "ignored", "dm", []string{"a", "b"})
	ch2, _ := fleetctx.CreateChannel(dir, "group", "group chat", []string{"a", "b", "c"})
	if err := fleetctx.SendToChannel(dir, ch1, "a", "hello"); err != nil {
		t.Fatalf("SendToChannel failed: %v", err)
	}
	if err := fleetctx.SendToChannel(dir, ch2, "a", "hi"); err != nil {
		t.Fatalf("SendToChannel failed: %v", err)
	}

	result, err := fleetctx.ClearContext(dir, fleetctx.ClearOptions{AllChannels: true})
	if err != nil {
		t.Fatalf("ClearContext failed: %v", err)
	}
	if len(result.ChannelsCleared) != 2 {
		t.Errorf("expected 2 channels cleared, got %d", len(result.ChannelsCleared))
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Channels[ch1].Log) != 0 {
		t.Errorf("ch1 log not cleared")
	}
	if len(ctx.Channels[ch2].Log) != 0 {
		t.Errorf("ch2 log not cleared")
	}
}

func TestClearContextChannelNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := fleetctx.ClearContext(dir, fleetctx.ClearOptions{Channels: []string{"no-such-channel"}})
	if err == nil {
		t.Fatal("expected error for missing channel, got nil")
	}
}

func TestClearContextPreservesChannelStructure(t *testing.T) {
	dir := t.TempDir()

	chName, _ := fleetctx.CreateChannel(dir, "ignored", "dm", []string{"alice", "bob"})
	if err := fleetctx.SendToChannel(dir, chName, "alice", "msg"); err != nil {
		t.Fatalf("SendToChannel failed: %v", err)
	}

	_, err := fleetctx.ClearContext(dir, fleetctx.ClearOptions{Channels: []string{chName}})
	if err != nil {
		t.Fatalf("ClearContext failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ch, ok := ctx.Channels[chName]
	if !ok {
		t.Fatal("channel was deleted, expected only log cleared")
	}
	if len(ch.Members) != 2 {
		t.Errorf("members cleared unexpectedly: got %v", ch.Members)
	}
}

func TestMultiAgentWorkflow(t *testing.T) {
	dir := t.TempDir()

	// User sets shared context
	if err := fleetctx.WriteShared(dir, "API uses JWT. Base path /v2."); err != nil {
		t.Fatalf("WriteShared failed: %v", err)
	}

	// auth-agent writes its section
	if err := fleetctx.WriteAgent(dir, "auth-agent", "User model at internal/auth/user.go. @api-agent merge fleet/auth"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	// api-agent writes its section
	if err := fleetctx.WriteAgent(dir, "api-agent", "Endpoints defined. Waiting on auth model."); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	// Read full context
	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if ctx.Shared != "API uses JWT. Base path /v2." {
		t.Errorf("shared: got %q", ctx.Shared)
	}
	if ctx.Agents["auth-agent"] != "User model at internal/auth/user.go. @api-agent merge fleet/auth" {
		t.Errorf("auth-agent: got %q", ctx.Agents["auth-agent"])
	}
	if ctx.Agents["api-agent"] != "Endpoints defined. Waiting on auth model." {
		t.Errorf("api-agent: got %q", ctx.Agents["api-agent"])
	}

	// auth-agent overwrites its section
	if err := fleetctx.WriteAgent(dir, "auth-agent", "Auth complete. All tests passing."); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	ctx, err = fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Agents["auth-agent"] != "Auth complete. All tests passing." {
		t.Errorf("auth-agent after overwrite: got %q", ctx.Agents["auth-agent"])
	}
	// api-agent should be untouched
	if ctx.Agents["api-agent"] != "Endpoints defined. Waiting on auth model." {
		t.Errorf("api-agent was clobbered: got %q", ctx.Agents["api-agent"])
	}
}
