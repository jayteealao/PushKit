package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// TestPrintVersion asserts the exact output format "pushkit-server <version>\n".
func TestPrintVersion(t *testing.T) {
	var buf bytes.Buffer
	printVersion(&buf, "1.2.3")
	got := buf.String()
	want := "pushkit-server 1.2.3\n"
	if got != want {
		t.Errorf("printVersion: got %q, want %q", got, want)
	}
}

// TestPrintVersionDefault asserts the exact format is preserved for the
// default "dev" version string.
func TestPrintVersionDefault(t *testing.T) {
	var buf bytes.Buffer
	printVersion(&buf, "dev")
	got := buf.String()
	want := "pushkit-server dev\n"
	if got != want {
		t.Errorf("printVersion default: got %q, want %q", got, want)
	}
}

// runVersionFlag runs versionBinary with the given flag and checks that:
//   - the process exits with code 0,
//   - stdout is exactly "pushkit-server dev\n".
func runVersionFlag(t *testing.T, flag string) {
	t.Helper()
	cmd := exec.Command(versionBinary, flag)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("%s flag: process failed: %v\nstderr: %s", flag, err, stderr.String())
	}
	// cmd.Run returns nil on exit-code 0; any non-zero exits via ExitError.
	// Explicitly verify the exit code for clarity.
	if code := cmd.ProcessState.ExitCode(); code != 0 {
		t.Errorf("%s flag: expected exit code 0, got %d", flag, code)
	}

	got := stdout.String()
	want := "pushkit-server dev\n"
	if got != want {
		t.Errorf("%s flag output: got %q, want %q", flag, got, want)
	}
}

func TestVersionFlag_LongForm(t *testing.T) {
	runVersionFlag(t, "--version")
}

func TestVersionFlag_ShortAlias(t *testing.T) {
	runVersionFlag(t, "-v")
}

// TestVersionFlag_LdflagsInjected verifies that -ldflags version injection
// produces exactly "pushkit-server <injected>\n" with exit code 0.
func TestVersionFlag_LdflagsInjected(t *testing.T) {
	dir, _ := os.MkdirTemp("", "pushkit-server-ldflags-*")
	defer os.RemoveAll(dir)

	bin := filepath.Join(dir, "pushkit-server")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	injected := "v9.8.7-test"
	buildCmd := exec.Command("go", "build", "-ldflags", "-X main.Version="+injected, "-o", bin, ".")
	buildCmd.Dir = "."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build with ldflags failed: %s", out)
	}

	for _, flag := range []string{"--version", "-v"} {
		t.Run(flag, func(t *testing.T) {
			cmd := exec.Command(bin, flag)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			if err := cmd.Run(); err != nil {
				t.Fatalf("%s on ldflags binary failed: %v\nstderr: %s", flag, err, stderr.String())
			}
			if code := cmd.ProcessState.ExitCode(); code != 0 {
				t.Errorf("%s: expected exit code 0, got %d", flag, code)
			}
			got := stdout.String()
			want := "pushkit-server " + injected + "\n"
			if got != want {
				t.Errorf("ldflags injection via %s: got %q, want %q", flag, got, want)
			}
		})
	}
}
