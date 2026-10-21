package clause

import (
	"strings"
	"testing"
)

// assertErrorContains fails unless err says want, so that a case refused
// for another reason than the one it tests does not pass.
func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want one saying %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want one saying %q", err, want)
	}
}
