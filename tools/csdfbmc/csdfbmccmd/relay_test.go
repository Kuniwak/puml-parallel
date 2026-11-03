//go:build !windows

package csdfbmccmd

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestASignalTheLauncherGetsGoesToTheChecker(t *testing.T) {
	needPython(t)

	// Setup: a checker that would wait half a minute, and a SIGTERM.
	cmd := exec.Command("python3", "-c", "import time; time.sleep(30)")
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	signals := make(chan os.Signal, 1)
	signals <- syscall.SIGTERM
	done := make(chan error, 1)

	// Execute
	go func() { done <- runRelaying(cmd, signals) }()

	// Assert
	select {
	case err := <-done:
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("err = %v, want the checker ended by SIGTERM", err)
		}
		if ws, ok := exit.Sys().(syscall.WaitStatus); !ok || ws.Signal() != syscall.SIGTERM {
			t.Errorf("the checker ended by %v, want SIGTERM", exit)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the checker did not get the signal")
	}
}
