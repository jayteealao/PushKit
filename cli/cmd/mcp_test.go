package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestIsCleanShutdown(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, true},
		{"eof", io.EOF, true},
		{"wrapped eof", fmt.Errorf("read failed: %w", io.EOF), true},
		{"context canceled", context.Canceled, true},
		{"jsonrpc server closing (post-request EOF)", errors.New("server is closing: EOF"), true},
		{"jsonrpc client closing", errors.New("client is closing"), true},
		{"connection refused is a real failure", errors.New("dial tcp: connection refused"), false},
		{"protocol error is a real failure", errors.New("invalid params"), false},
	}
	for _, tt := range tests {
		if got := isCleanShutdown(tt.err); got != tt.want {
			t.Errorf("%s: isCleanShutdown(%v) = %v, want %v", tt.name, tt.err, got, tt.want)
		}
	}
}
