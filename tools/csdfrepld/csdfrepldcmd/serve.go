package csdfrepldcmd

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/animation/proto"
)

// serve listens on the Unix socket and serves one request per connection until
// an interrupt arrives, at which point it stops accepting and removes the socket.
func serve(sock string, service *proto.Service, inout *cli.ProcInout, interrupts <-chan os.Signal) error {
	if err := ensureSocketFree(sock); err != nil {
		return err
	}

	listener, err := net.Listen("unix", sock)
	if err != nil {
		return fmt.Errorf("csdfrepldcmd.serve: listening on %q: %w", sock, err)
	}
	defer func() { _ = os.Remove(sock) }()

	if err := os.Chmod(sock, 0o600); err != nil {
		_ = listener.Close()
		return fmt.Errorf("csdfrepldcmd.serve: chmod %q: %w", sock, err)
	}

	fmt.Fprintf(inout.Stderr, "csdfrepld listening on %s\n", sock)

	var shuttingDown atomic.Bool
	go func() {
		<-interrupts
		shuttingDown.Store(true)
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if shuttingDown.Load() {
				return nil
			}
			return fmt.Errorf("csdfrepldcmd.serve: accept: %w", err)
		}
		go handleConn(conn, service)
	}
}

// ensureSocketFree clears a leftover socket file, refusing to start when another
// daemon is already listening on it.
func ensureSocketFree(sock string) error {
	if _, err := os.Stat(sock); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("csdfrepldcmd.ensureSocketFree: stat %q: %w", sock, err)
	}

	if conn, err := net.DialTimeout("unix", sock, 200*time.Millisecond); err == nil {
		_ = conn.Close()
		return fmt.Errorf("csdfrepldcmd.ensureSocketFree: another csdfrepld is already listening on %q", sock)
	}

	if err := os.Remove(sock); err != nil {
		return fmt.Errorf("csdfrepldcmd.ensureSocketFree: removing stale socket %q: %w", sock, err)
	}
	return nil
}

func handleConn(conn net.Conn, service *proto.Service) {
	defer func() { _ = conn.Close() }()

	req, err := proto.ReadRequest(conn)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			_ = proto.WriteResponse(conn, proto.Response{OK: false, Error: fmt.Sprintf("malformed request: %v", err)})
		}
		return
	}
	_ = proto.WriteResponse(conn, service.Handle(req))
}
