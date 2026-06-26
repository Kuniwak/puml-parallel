package csdfreplcmdcmd

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/animation/proto"
	"github.com/Kuniwak/puml-parallel/tools"
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

// startDaemon serves an in-process proto.Service on a temp Unix socket and
// returns its path.
func startDaemon(t *testing.T) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "s.sock")
	listener, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	service := proto.NewService("client-test", false)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				req, err := proto.ReadRequest(conn)
				if err != nil {
					return
				}
				_ = proto.WriteResponse(conn, service.Handle(req))
			}()
		}
	}()
	t.Cleanup(func() { _ = listener.Close() })
	return sock
}

// run invokes the csdfreplcmd command tree with the daemon socket in the env.
func run(t *testing.T, sock string, args ...string) (int, string, string) {
	t.Helper()
	spy := cli.SpyProcInout()
	spy.Env = map[string]string{tools.SocketEnv: sock}
	cmd := tools.NewSubcommandFunc("csdfreplcmd", "", Subcommands())
	code := cmd(args, spy.New())
	return code, spy.Stdout.String(), spy.Stderr.String()
}

func writeDiagram(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "diagram.puml")
	if err := os.WriteFile(path, []byte(twoStateDiagram), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// newSession starts the daemon, creates one session, and returns the socket and
// the new session id (with the trailing newline trimmed).
func newSession(t *testing.T) (string, string) {
	t.Helper()
	sock := startDaemon(t)
	code, stdout, stderr := run(t, sock, "session", "new", writeDiagram(t))
	if code != 0 {
		t.Fatalf("session new exit = %d, stderr = %q", code, stderr)
	}
	return sock, strings.TrimSpace(stdout)
}

func TestSessionNewPrintsID(t *testing.T) {
	sock, id := newSession(t)
	if id != "1" {
		t.Errorf("session new id = %q, want 1", id)
	}
	_ = sock
}

func TestReadResolvesSingleSession(t *testing.T) {
	sock, _ := newSession(t)
	code, stdout, stderr := run(t, sock, "read")
	if code != 0 {
		t.Fatalf("read exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "Post State Group:") {
		t.Errorf("read stdout = %q, want value prompt", stdout)
	}
}

func TestStatevarAndReadFlow(t *testing.T) {
	sock, _ := newSession(t)
	if code, _, stderr := run(t, sock, "statevar", "-json", "[0]"); code != 0 {
		t.Fatalf("statevar exit = %d, stderr = %q", code, stderr)
	}
	code, stdout, stderr := run(t, sock, "read")
	if code != 0 {
		t.Fatalf("read exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "State: Initial (s0)") || !strings.Contains(stdout, "Transitions:") {
		t.Errorf("read stdout = %q, want state + transitions", stdout)
	}
}

func TestJSONOutput(t *testing.T) {
	sock, _ := newSession(t)
	run(t, sock, "statevar", "-json", "[0]")
	code, stdout, stderr := run(t, sock, "read", "-json")
	if code != 0 {
		t.Fatalf("read -json exit = %d, stderr = %q", code, stderr)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout), "{") || !strings.Contains(stdout, `"mode":"command"`) {
		t.Errorf("read -json stdout = %q, want structured JSON", stdout)
	}
}

func TestSelectStatevarTraceJumpFlow(t *testing.T) {
	sock, _ := newSession(t)
	run(t, sock, "statevar", "-json", "[0]")

	if code, _, stderr := run(t, sock, "select", "0"); code != 0 {
		t.Fatalf("select exit = %d, stderr = %q", code, stderr)
	}
	if code, _, stderr := run(t, sock, "statevar", "-json", `["ok"]`); code != 0 {
		t.Fatalf("statevar exit = %d, stderr = %q", code, stderr)
	}

	code, stdout, stderr := run(t, sock, "trace")
	if code != 0 {
		t.Fatalf("trace exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "insert(coin)") {
		t.Errorf("trace stdout = %q, want insert(coin)", stdout)
	}

	if code, _, stderr := run(t, sock, "jump", "0"); code != 0 {
		t.Fatalf("jump exit = %d, stderr = %q", code, stderr)
	}
}

func TestStatevarFromFile(t *testing.T) {
	sock, _ := newSession(t)
	valuesFile := filepath.Join(t.TempDir(), "values.json")
	if err := os.WriteFile(valuesFile, []byte("[0]"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run(t, sock, "statevar", "-json-file", valuesFile)
	if code != 0 {
		t.Fatalf("statevar -json-file exit = %d, stderr = %q", code, stderr)
	}
}

func TestStatevarRequiresValues(t *testing.T) {
	sock, _ := newSession(t)
	code, _, stderr := run(t, sock, "statevar")
	if code == 0 || !strings.Contains(stderr, "requires -json") {
		t.Errorf("statevar without values = (exit %d, stderr %q), want failure", code, stderr)
	}
}

func TestServerVersion(t *testing.T) {
	sock := startDaemon(t)
	code, stdout, stderr := run(t, sock, "serverversion")
	if code != 0 {
		t.Fatalf("serverversion exit = %d, stderr = %q", code, stderr)
	}
	if strings.TrimSpace(stdout) != "client-test" {
		t.Errorf("serverversion stdout = %q, want client-test", stdout)
	}
}

func TestSessionListAndRm(t *testing.T) {
	sock, id := newSession(t)

	code, stdout, stderr := run(t, sock, "session", "list")
	if code != 0 {
		t.Fatalf("session list exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, id) {
		t.Errorf("session list stdout = %q, want session %q", stdout, id)
	}

	if code, _, stderr := run(t, sock, "session", "rm", "-s", id); code != 0 {
		t.Fatalf("session rm exit = %d, stderr = %q", code, stderr)
	}
	if code, _, _ := run(t, sock, "read"); code == 0 {
		t.Error("read after rm exit = 0, want failure (no sessions)")
	}
}

func TestDialFailureIsFriendly(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "absent.sock")
	code, _, stderr := run(t, sock, "serverversion")
	if code == 0 || !strings.Contains(stderr, "cannot connect to csdfrepld") {
		t.Errorf("serverversion without daemon = (exit %d, stderr %q), want friendly connect error", code, stderr)
	}
}

func TestDaemonErrorSurfaces(t *testing.T) {
	sock := startDaemon(t)
	code, _, stderr := run(t, sock, "read")
	if code == 0 || !strings.Contains(stderr, "no sessions") {
		t.Errorf("read with no sessions = (exit %d, stderr %q), want \"no sessions\"", code, stderr)
	}
}

func TestHelp(t *testing.T) {
	sock := startDaemon(t)
	code, stdout, _ := run(t, sock, "help")
	if code != 0 || !strings.Contains(stdout, "Commands:") || !strings.Contains(stdout, "statevar") {
		t.Errorf("help = (exit %d, stdout %q), want command listing", code, stdout)
	}
}

func TestHelpAndUnknownDispatch(t *testing.T) {
	exec := func(args ...string) (int, string, string) {
		spy := cli.SpyProcInout()
		code := tools.NewSubcommandFunc("csdfreplcmd", "", Subcommands())(args, spy.New())
		return code, spy.Stdout.String(), spy.Stderr.String()
	}

	// A command group's -h and "help" both print group help and exit 0.
	if code, out, _ := exec("session", "-h"); code != 0 || !strings.Contains(out, "Commands:") {
		t.Errorf("session -h = (exit %d, stdout %q), want group help exit 0", code, out)
	}
	if code, out, _ := exec("session", "help"); code != 0 || !strings.Contains(out, "Commands:") {
		t.Errorf("session help = (exit %d, stdout %q), want group help exit 0", code, out)
	}

	// An unknown command prints help and exits 1 with no internal identifier leak.
	code, _, stderr := exec("frobnicate")
	if code != 1 || !strings.Contains(stderr, `unknown command "frobnicate"`) {
		t.Errorf("frobnicate = (exit %d, stderr %q), want unknown-command exit 1", code, stderr)
	}
	if strings.Contains(stderr, "tools.") || strings.Contains(stderr, "no such subcommand") {
		t.Errorf("frobnicate stderr leaks internals: %q", stderr)
	}

	// "help <command>" delegates to that command's own -h (exit 0).
	if code, _, _ := exec("help", "select"); code != 0 {
		t.Errorf("help select exit = %d, want 0", code)
	}
}
