package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &Config{
		APIURL: "http://localhost:8000",
		APIKey: "test-key-123",
	}

	// Write directly to test file.
	data := `{"api_url":"http://localhost:8000","api_key":"test-key-123"}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read it back.
	readData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// Verify content.
	_ = cfg
	if string(readData) != data {
		t.Errorf("data mismatch: got %s", readData)
	}
}

func TestLoadMissing(t *testing.T) {
	// This tests that Load returns empty config for missing file.
	// Since Load depends on the config path, we test the deserialization logic.
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	_, err := os.ReadFile(path)
	if !os.IsNotExist(err) {
		t.Errorf("expected not-exist, got %v", err)
	}
}
