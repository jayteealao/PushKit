package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

// buildTree creates a small fixture tree under a fresh temp dir and returns it:
//
//	top.txt
//	a/x.txt
//	a/b/y.txt
//	a/b/z.md
func buildTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := []string{"top.txt", "a/x.txt", "a/b/y.txt", "a/b/z.md"}
	for _, rel := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// baseSet returns the set of base filenames in paths.
func baseSet(paths []string) map[string]bool {
	s := make(map[string]bool, len(paths))
	for _, p := range paths {
		s[filepath.Base(p)] = true
	}
	return s
}

func TestExpand(t *testing.T) {
	root := buildTree(t)

	tests := []struct {
		name      string
		patterns  []string
		wantBases []string
		wantCount int
	}{
		{name: "explicit file", patterns: []string{"top.txt"}, wantBases: []string{"top.txt"}, wantCount: 1},
		{name: "directory recurses", patterns: []string{"a"}, wantBases: []string{"x.txt", "y.txt", "z.md"}, wantCount: 3},
		{name: "doublestar txt", patterns: []string{"**/*.txt"}, wantBases: []string{"top.txt", "x.txt", "y.txt"}, wantCount: 3},
		{name: "nested md glob", patterns: []string{"a/**/*.md"}, wantBases: []string{"z.md"}, wantCount: 1},
		{name: "brace expansion", patterns: []string{"{a/x.txt,top.txt}"}, wantBases: []string{"x.txt", "top.txt"}, wantCount: 2},
		{name: "dedup across patterns", patterns: []string{"top.txt", "top.txt"}, wantBases: []string{"top.txt"}, wantCount: 1},
		{name: "missing path is empty", patterns: []string{"nope.txt"}, wantBases: []string{}, wantCount: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expand(root, tt.patterns)
			if err != nil {
				t.Fatalf("expand: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Fatalf("got %d matches %v, want %d", len(got), got, tt.wantCount)
			}
			bases := baseSet(got)
			for _, b := range tt.wantBases {
				if !bases[b] {
					t.Errorf("missing expected file %q in %v", b, got)
				}
			}
		})
	}
}

func TestExpandRejectsUnsafeRoot(t *testing.T) {
	root := buildTree(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	if _, err := expand(root, []string{filepath.Join(home, "**")}); err == nil {
		t.Error("expected an error globbing from the home directory")
	}
}

func TestExpandMatchCap(t *testing.T) {
	root := buildTree(t)
	saved := maxMatches
	maxMatches = 2
	defer func() { maxMatches = saved }()

	if _, err := expand(root, []string{"a"}); err == nil {
		t.Error("expected an error when matches exceed the cap")
	}
}
