package mcp

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// maxMatches caps how many files a single pushkit_push call may expand to. It is
// a var (not a const) so tests can lower it without creating thousands of files.
var maxMatches = 5000

const globMeta = "*?[{"

// expand resolves a list of push patterns into a deduplicated list of regular
// files. Each pattern is one of: an explicit file path, a directory (expanded
// recursively), or a glob (** and {a,b} via doublestar). Relative patterns are
// resolved against projectDir. Expansion is bounded: it refuses obviously-unsafe
// roots (the filesystem root, a drive root, or the home directory) and caps the
// total match count. Non-matching globs and missing paths contribute nothing
// rather than erroring, so the caller decides what an empty result means.
func expand(projectDir string, patterns []string) ([]string, error) {
	seen := make(map[string]struct{})
	var out []string

	add := func(p string) error {
		if _, ok := seen[p]; ok {
			return nil
		}
		seen[p] = struct{}{}
		out = append(out, p)
		if len(out) > maxMatches {
			return fmt.Errorf("too many files matched (>%d); narrow the path or glob", maxMatches)
		}
		return nil
	}

	for _, raw := range patterns {
		if strings.TrimSpace(raw) == "" {
			continue
		}

		p := raw
		if filepath.IsAbs(p) {
			p = filepath.Clean(p)
		} else {
			p = filepath.Join(projectDir, p)
		}

		if err := checkSafeRoot(p); err != nil {
			return nil, err
		}

		if !strings.ContainsAny(p, globMeta) {
			info, err := os.Stat(p)
			if err != nil {
				continue // missing explicit path → no match
			}
			if info.IsDir() {
				if err := walkDirFiles(p, add); err != nil {
					return nil, err
				}
			} else if info.Mode().IsRegular() {
				if err := add(p); err != nil {
					return nil, err
				}
			}
			continue
		}

		matches, err := doublestar.FilepathGlob(p)
		if err != nil {
			return nil, fmt.Errorf("invalid glob %q: %w", raw, err)
		}
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || !info.Mode().IsRegular() {
				continue // skip directories and broken matches
			}
			if err := add(m); err != nil {
				return nil, err
			}
		}
	}

	return out, nil
}

// walkDirFiles adds every regular file under root (no symlink following, since
// WalkDir uses Lstat). The add callback may abort the walk by returning an error
// (e.g. the match cap was hit).
func walkDirFiles(root string, add func(string) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			return add(path)
		}
		return nil
	})
}

// checkSafeRoot rejects patterns whose scan base is the filesystem root, a drive
// root, or the user's home directory — guarding against an unbounded walk like
// "/**" or "~/**". Relative patterns are already rooted under projectDir by the
// time this runs, so only absolute patterns can trip it.
func checkSafeRoot(pattern string) error {
	base := scanBase(pattern)
	if base == "" || base == "." {
		return nil
	}
	base = filepath.Clean(base)
	if isRootish(base) {
		return fmt.Errorf("refusing to expand from an unsafe root %q; choose a narrower path", base)
	}
	return nil
}

// scanBase returns the leading, meta-free directory prefix of a pattern — the
// directory doublestar would start scanning from.
func scanBase(pattern string) string {
	sep := string(os.PathSeparator)
	parts := strings.Split(pattern, sep)
	var base []string
	for _, part := range parts {
		if strings.ContainsAny(part, globMeta) {
			break
		}
		base = append(base, part)
	}
	b := strings.Join(base, sep)
	if b == "" && strings.HasPrefix(pattern, sep) {
		return sep // absolute pattern globbing from the root
	}
	return b
}

// isRootish reports whether p is a filesystem root, a volume/drive root, or the
// user's home directory.
func isRootish(p string) bool {
	if p == filepath.Dir(p) { // "/" and "C:\" are their own parent
		return true
	}
	if home, err := os.UserHomeDir(); err == nil && filepath.Clean(home) == p {
		return true
	}
	return false
}
