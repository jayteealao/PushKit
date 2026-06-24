package mcp

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStatePulledRoundTrip(t *testing.T) {
	dir := t.TempDir()
	const base = "http://localhost:8080"

	s := LoadState(dir, base)
	if s.IsPulled("file-x") {
		t.Fatal("fresh state should report nothing pulled")
	}
	if err := s.MarkPulled("file-x", "file-y"); err != nil {
		t.Fatalf("MarkPulled: %v", err)
	}

	// Reload from disk: the marks must survive.
	reloaded := LoadState(dir, base)
	if !reloaded.IsPulled("file-x") || !reloaded.IsPulled("file-y") {
		t.Error("pulled IDs did not survive reload")
	}
	if reloaded.IsPulled("file-z") {
		t.Error("file-z was never pulled")
	}
}

func TestStateNamespaceIsolation(t *testing.T) {
	dir := t.TempDir()
	a := LoadState(dir, "http://a.example")
	if err := a.MarkPulled("shared-id"); err != nil {
		t.Fatalf("MarkPulled: %v", err)
	}
	// A different base URL must not see A's pulled IDs.
	b := LoadState(dir, "http://b.example")
	if b.IsPulled("shared-id") {
		t.Error("namespaces must not share pulled history")
	}
}

func TestStateFileContentsAndPerms(t *testing.T) {
	dir := t.TempDir()
	s := LoadState(dir, "http://localhost:8080")
	if err := s.MarkPulled("only-an-id"); err != nil {
		t.Fatalf("MarkPulled: %v", err)
	}

	path := filepath.Join(dir, stateFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	contents := string(data)
	if !strings.Contains(contents, "only-an-id") {
		t.Errorf("state file should contain the pulled ID, got: %s", contents)
	}
	// The store must never persist secrets — only file IDs.
	for _, leak := range []string{"apiKey", "api_key", "X-API-Key", "secret", "Authorization"} {
		if strings.Contains(contents, leak) {
			t.Errorf("state file leaked a secret-like token %q: %s", leak, contents)
		}
	}

	if runtime.GOOS != "windows" { // Go maps file perms loosely on Windows
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat state file: %v", err)
		}
		if perm := fi.Mode().Perm(); perm != 0o600 {
			t.Errorf("state file perms = %o, want 600", perm)
		}
	}
}

func TestAcquireStateLock_RecoversStaleLock(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, stateLockFileName)

	// Write a lockfile and backdate its mtime to simulate an abandoned lock.
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatalf("create stale lockfile: %v", err)
	}
	oldTime := time.Now().Add(-1 * time.Minute)
	if err := os.Chtimes(lockPath, oldTime, oldTime); err != nil {
		t.Fatalf("backdate lockfile mtime: %v", err)
	}

	release, err := acquireStateLock(dir)
	if err != nil {
		t.Fatalf("acquireStateLock should recover a stale lock, got: %v", err)
	}
	if release == nil {
		t.Fatal("acquireStateLock returned nil release func")
	}
	release()

	// After release the lockfile must be gone.
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lockfile should be removed after release")
	}
}

func TestStatePushedWindow(t *testing.T) {
	s := LoadState(t.TempDir(), "http://localhost:8080")
	s.RecordPushed("just-pushed")

	if !s.WasRecentlyPushed("just-pushed", time.Minute) {
		t.Error("a just-pushed ID should be recently pushed within a minute")
	}
	if s.WasRecentlyPushed("never-pushed", time.Minute) {
		t.Error("an unrecorded ID is not recently pushed")
	}
	// A negative window proves the expiry branch without sleeping.
	if s.WasRecentlyPushed("just-pushed", -time.Second) {
		t.Error("a negative window must treat everything as expired")
	}
}
