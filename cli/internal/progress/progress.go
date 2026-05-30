package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type Reader struct {
	reader io.Reader
	total  int64
	read   int64
	mu     sync.Mutex
	last   time.Time
}

func NewReader(r io.Reader, total int64) *Reader {
	return &Reader{
		reader: r,
		total:  total,
		last:   time.Now(),
	}
}

func (pr *Reader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.mu.Lock()
	pr.read += int64(n)
	now := time.Now()
	if now.Sub(pr.last) > 100*time.Millisecond || err == io.EOF {
		pr.render()
		pr.last = now
	}
	pr.mu.Unlock()
	return n, err
}

func (pr *Reader) render() {
	if pr.total <= 0 {
		fmt.Fprintf(os.Stderr, "\r  %s uploaded", formatBytes(pr.read))
		return
	}

	pct := float64(pr.read) / float64(pr.total) * 100
	barWidth := 30
	filled := int(pct / 100 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("=", filled)
	if filled < barWidth {
		bar += ">"
		bar += strings.Repeat(" ", barWidth-filled-1)
	}

	fmt.Fprintf(os.Stderr, "\r  [%s] %3.0f%% %s/%s",
		bar, pct,
		formatBytes(pr.read), formatBytes(pr.total),
	)
}

func (pr *Reader) Finish() {
	fmt.Fprintln(os.Stderr)
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// FormatBytes is the exported version for use in other packages.
func FormatBytes(b int64) string {
	return formatBytes(b)
}

// MaybeNewReader returns a progress-tracking Reader when jsonMode is false,
// or the raw reader unchanged when jsonMode is true (no progress output).
// The second return value is the *Reader; callers must call Finish() on it
// when done if it is non-nil.
func MaybeNewReader(r io.Reader, total int64, jsonMode bool) (io.Reader, *Reader) {
	if jsonMode {
		return r, nil
	}
	pr := NewReader(r, total)
	return pr, pr
}
