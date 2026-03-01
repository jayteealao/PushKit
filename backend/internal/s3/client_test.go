package s3

import "testing"

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"report.pdf", "report.pdf"},
		{"my file (1).pdf", "my_file__1_.pdf"},
		{"../../etc/passwd", ".._.._etc_passwd"},
		{"path/to\\file.txt", "path_to_file.txt"},
		{"   spaces   ", "spaces"},
		{"", "file"},
		{"hello world!@#$.txt", "hello_world____.txt"},
		{"résumé.pdf", "r_sum_.pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildS3Key_Format(t *testing.T) {
	key := BuildS3Key("user1", "report.pdf")

	if len(key) == 0 {
		t.Fatal("expected non-empty key")
	}

	// Must start with uploads/user1/
	prefix := "uploads/user1/"
	if key[:len(prefix)] != prefix {
		t.Errorf("expected key to start with %q, got %q", prefix, key)
	}

	// Must end with sanitized filename.
	if key[len(key)-len("report.pdf"):] != "report.pdf" {
		t.Errorf("expected key to end with report.pdf, got %q", key)
	}
}

func TestSanitizeFilename_LongName(t *testing.T) {
	long := ""
	for i := 0; i < 200; i++ {
		long += "a"
	}
	key := BuildS3Key("user1", long)
	// The sanitized portion should be truncated.
	if len(key) > 300 {
		t.Errorf("key too long: %d", len(key))
	}
}
