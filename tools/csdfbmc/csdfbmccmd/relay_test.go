package csdfbmccmd

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestASignalTheLauncherGetsGoesToTheChecker(t *testing.T) {
	needPython(t)
	// A checker that would wait half a minute.
	cmd := exec.Command("python3", "-c", "import time; time.sleep(30)")
	signals := make(chan os.Signal, 1)
	signals <- syscall.SIGTERM
	done := make(chan error, 1)
	go func() { done <- runRelaying(cmd, signals) }()

	select {
	case err := <-done:
		if err == nil {
			t.Errorf("the checker ended well; want it ended by SIGTERM")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the checker did not get the signal")
	}
}
