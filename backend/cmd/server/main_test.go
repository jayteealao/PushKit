package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// versionBinary holds the path to the built test binary (set by TestMain).
var versionBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pushkit-server-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	bin := filepath.Join(dir, "pushkit-server")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("build failed: " + string(out))
	}
	versionBinary = bin
	os.Exit(m.Run())
}

func TestPrintVersion(t *testing.T) {
	var buf bytes.Buffer
	printVersion(&buf, "1.2.3")
	got := buf.String()
	want := "pushkit-server 1.2.3\n"
	if got != want {
		t.Errorf("printVersion: got %q, want %q", got, want)
	}
}

func TestPrintVersionDefault(t *testing.T) {
	var buf bytes.Buffer
	printVersion(&buf, "dev")
	got := buf.String()
	if !strings.HasPrefix(got, "pushkit-server ") {
		t.Errorf("printVersion default: unexpected output %q", got)
	}
}

func TestVersionFlag_Binary(t *testing.T) {
	out, err := exec.Command(versionBinary, "--version").Output()
	if err != nil {
		t.Fatalf("--version flag failed: %v", err)
	}
	got := strings.TrimSpace(string(out))
	want := "pushkit-server dev"
	if got != want {
		t.Errorf("--version output: got %q, want %q", got, want)
	}
}

func TestVersionFlag_LdflagsInjected(t *testing.T) {
	dir, _ := os.MkdirTemp("", "pushkit-server-ldflags-*")
	defer os.RemoveAll(dir)

	bin := filepath.Join(dir, "pushkit-server")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	injected := "v9.8.7-test"
	cmd := exec.Command("go", "build", "-ldflags", "-X main.Version="+injected, "-o", bin, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build with ldflags failed: %s", out)
	}

	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		t.Fatalf("--version on ldflags binary failed: %v", err)
	}
	got := strings.TrimSpace(string(out))
	want := "pushkit-server " + injected
	if got != want {
		t.Errorf("ldflags injection: got %q, want %q", got, want)
	}
}
