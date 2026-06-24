package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const stateLockFileName = "state.json.lock"

// staleLockThreshold bounds how long a lockfile may sit before it is treated
// as abandoned (left by a killed/crashed process). It is far above the
// millisecond hold time of a normal state write, so a live lock is never
// removed.
const staleLockThreshold = 30 * time.Second

// acquireStateLock takes an OS-level advisory lock on the state file by creating
// a sentinel lockfile exclusively. It retries until the lock is acquired or the
// deadline (5 s) is exceeded. Returns a release func.
//
// Stale-lock recovery: if the lockfile already exists but its mtime is older
// than staleLockThreshold, it is treated as abandoned (left by a killed or
// crashed process) and removed so the current run can proceed immediately.
//
// This guards concurrent CLI sessions on the same ~/.pushkit/state.json: without
// it, two sessions racing on the read-modify-write-rename can silently drop each
// other's updates (last rename wins). The lockfile approach is portable across
// Linux, macOS, and Windows without any additional dependency.
//
// Limitation: the lock is advisory — a process that does not call acquireStateLock
// (e.g. an external editor) is not blocked. This is acceptable for a
// single-binary CLI; the assumption is that only pushkit CLI instances write this
// file.
func acquireStateLock(dir string) (release func(), err error) {
	lockPath := filepath.Join(dir, stateLockFileName)
	deadline := time.Now().Add(5 * time.Second)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			return func() { os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("acquire state lock: %w", err)
		}
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > staleLockThreshold {
			_ = os.Remove(lockPath) // reclaim a lock abandoned by a dead process
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("acquire state lock: timed out waiting for %s", lockPath)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

const stateFileName = "state.json"

// pushkitHome returns the ~/.pushkit directory used for the pulled-ID state file
// and the pull download fallback. Callers create it lazily.
func pushkitHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".pushkit"), nil
}

// onDisk is the JSON layout of ~/.pushkit/state.json. State is namespaced by API
// base URL so distinct backends do not share pulled-ID history. Only file IDs are
// stored — never API keys or any other secret.
type onDisk struct {
	Namespaces map[string]*namespaceState `json:"namespaces"`
}

type namespaceState struct {
	Pulled []string `json:"pulled"`
}

// State tracks client-side file-ID history for one API base URL. Pulled IDs are
// persisted so the pushkit_pull "all-unpulled" selector spans Claude Code
// sessions; pushed IDs are an in-memory, short-lived set recorded for self-echo
// de-dup (consumed by the auto-refresh slice).
type State struct {
	mu      sync.Mutex
	dir     string // ~/.pushkit
	baseURL string
	pulled  map[string]struct{}
	pushed  map[string]time.Time
}

// LoadState reads the pulled-ID set for baseURL from dir/state.json. It is
// best-effort: a missing or malformed file yields an empty set (a later
// MarkPulled still persists correctly). It never returns an error.
func LoadState(dir, baseURL string) *State {
	s := &State{
		dir:     dir,
		baseURL: baseURL,
		pulled:  map[string]struct{}{},
		pushed:  map[string]time.Time{},
	}
	data, err := os.ReadFile(filepath.Join(dir, stateFileName))
	if err != nil {
		return s
	}
	var disk onDisk
	if json.Unmarshal(data, &disk) != nil {
		return s
	}
	if ns := disk.Namespaces[baseURL]; ns != nil {
		for _, id := range ns.Pulled {
			s.pulled[id] = struct{}{}
		}
	}
	return s
}

// IsPulled reports whether fileID has already been pulled for this base URL.
func (s *State) IsPulled(fileID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.pulled[fileID]
	return ok
}

// MarkPulled adds fileIDs to the persisted pulled set, merging the other
// namespaces in the file untouched. Persistence is best-effort from the caller's
// point of view: a returned error means the pull itself still succeeded but the
// ID may be offered again by a future "all-unpulled" call.
func (s *State) MarkPulled(fileIDs ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for _, id := range fileIDs {
		if _, ok := s.pulled[id]; !ok {
			s.pulled[id] = struct{}{}
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.persistLocked()
}

// RecordPushed remembers that fileID was just pushed by this server (in-memory
// only, for self-echo de-dup). Not persisted.
func (s *State) RecordPushed(fileID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pushed[fileID] = time.Now()
}

// WasRecentlyPushed reports whether fileID was pushed within the given window.
func (s *State) WasRecentlyPushed(fileID string, window time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.pushed[fileID]
	return ok && time.Since(t) <= window
}

// persistLocked writes the merged state file atomically (temp + rename, 0600).
// It acquires an OS-level advisory lockfile so concurrent CLI sessions do not
// silently drop each other's updates (last-rename-wins race). The caller must
// hold s.mu.
func (s *State) persistLocked() error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	release, err := acquireStateLock(s.dir)
	if err != nil {
		return err
	}
	defer release()

	// Re-read so we merge namespaces this State instance does not own.
	disk := onDisk{Namespaces: map[string]*namespaceState{}}
	if data, err := os.ReadFile(filepath.Join(s.dir, stateFileName)); err == nil {
		_ = json.Unmarshal(data, &disk)
		if disk.Namespaces == nil {
			disk.Namespaces = map[string]*namespaceState{}
		}
	}

	ids := make([]string, 0, len(s.pulled))
	for id := range s.pulled {
		ids = append(ids, id)
	}
	sort.Strings(ids) // deterministic on-disk order
	disk.Namespaces[s.baseURL] = &namespaceState{Pulled: ids}

	data, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	tmp, err := os.CreateTemp(s.dir, stateFileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp state: %w", err)
	}
	tmpName := tmp.Name()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp state: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp state: %w", err)
	}
	// Sync before rename so the file is durable on power loss.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("sync temp state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp state: %w", err)
	}
	if err := os.Rename(tmpName, filepath.Join(s.dir, stateFileName)); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename state: %w", err)
	}
	return nil
}
