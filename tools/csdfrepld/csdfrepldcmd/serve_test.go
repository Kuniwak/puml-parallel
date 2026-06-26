package csdfrepldcmd

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/animation/proto"
)

const twoStateDiagram = `@startuml
state "Initial" as s0
s0: count ; number
state "Done" as s1
s1: result ; string
[*] --> s0 : count starts at zero
s0 --> s1 : insert(coin) ; count >= 0 ; result is done
@enduml
`

// waitForDaemon polls by dialing until the daemon is actually accepting, which
// (unlike os.Stat) also tolerates a stale socket file being replaced.
func waitForDaemon(t *testing.T, sock string) {
	t.Helper()
	for i := 0; i < 1000; i++ {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("daemon at %q did not become ready", sock)
}

func do(t *testing.T, sock string, req proto.Request) proto.Response {
	t.Helper()
	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer func() { _ = conn.Close() }()
	resp, err := proto.Do(conn, req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	return resp
}

func TestServeRoundTripAndShutdown(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "test.sock")
	interrupts := make(chan os.Signal, 1)
	spy := cli.SpyProcInout()
	done := make(chan error, 1)
	go func() { done <- serve(sock, proto.NewService("test-version"), spy.New(), interrupts) }()
	waitForDaemon(t, sock)

	if resp := do(t, sock, proto.Request{Command: proto.CommandSessionNew, Content: []byte(twoStateDiagram)}); !resp.OK || resp.Session != "1" {
		t.Fatalf("session_new = (ok %v, session %q, error %q)", resp.OK, resp.Session, resp.Error)
	}
	if resp := do(t, sock, proto.Request{Command: proto.CommandRead}); !resp.OK || !strings.Contains(resp.Output, "Post State Group:") {
		t.Fatalf("read = (ok %v, output %q, error %q)", resp.OK, resp.Output, resp.Error)
	}
	if resp := do(t, sock, proto.Request{Command: proto.CommandServerVersion}); !resp.OK || resp.Output != "test-version\n" {
		t.Fatalf("server_version = (ok %v, output %q)", resp.OK, resp.Output)
	}

	interrupts <- os.Interrupt
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve() did not return after interrupt")
	}
	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Errorf("socket %q was not removed on shutdown (err = %v)", sock, err)
	}
}

func TestServeRemovesStaleSocket(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "stale.sock")
	if err := os.WriteFile(sock, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	interrupts := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() { done <- serve(sock, proto.NewService("test"), cli.SpyProcInout().New(), interrupts) }()
	waitForDaemon(t, sock)

	if resp := do(t, sock, proto.Request{Command: proto.CommandServerVersion}); !resp.OK {
		t.Fatalf("server_version after stale removal failed: %s", resp.Error)
	}

	interrupts <- os.Interrupt
	if err := <-done; err != nil {
		t.Fatalf("serve() error = %v", err)
	}
}

func TestServeRejectsLiveDaemon(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "live.sock")
	interrupts := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() { done <- serve(sock, proto.NewService("test"), cli.SpyProcInout().New(), interrupts) }()
	waitForDaemon(t, sock)
	defer func() {
		interrupts <- os.Interrupt
		<-done
	}()

	err := serve(sock, proto.NewService("test"), cli.SpyProcInout().New(), make(chan os.Signal, 1))
	if err == nil || !strings.Contains(err.Error(), "already listening") {
		t.Errorf("second serve() error = %v, want \"already listening\"", err)
	}
}
