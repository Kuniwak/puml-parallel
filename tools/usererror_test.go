package tools

import (
	"errors"
	"fmt"
	"testing"
)

func TestUserFacingError(t *testing.T) {
	wrapped := fmt.Errorf("csdf.SolveJSON: invalid JSON array: %w", errors.New("top-level value must be an array"))
	deep := fmt.Errorf("a: %w", fmt.Errorf("b: %w", errors.New("c")))

	tests := []struct {
		name  string
		err   error
		debug bool
		want  string
	}{
		{name: "nil", err: nil, debug: false, want: ""},
		{name: "nil debug", err: nil, debug: true, want: ""},
		{name: "single wrap, no debug, unwraps to leaf", err: wrapped, debug: false, want: "top-level value must be an array"},
		{name: "single wrap, debug, full chain", err: wrapped, debug: true, want: "csdf.SolveJSON: invalid JSON array: top-level value must be an array"},
		{name: "multi wrap, no debug, deepest", err: deep, debug: false, want: "c"},
		{name: "multi wrap, debug, full chain", err: deep, debug: true, want: "a: b: c"},
		{name: "unwrapped leaf, no debug", err: errors.New("plain"), debug: false, want: "plain"},
		{name: "unwrapped leaf, debug", err: errors.New("plain"), debug: true, want: "plain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UserFacingError(tt.err, tt.debug); got != tt.want {
				t.Errorf("UserFacingError(%v, %v) = %q, want %q", tt.err, tt.debug, got, tt.want)
			}
		})
	}
}
