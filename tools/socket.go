package tools

import (
	"os"
	"path/filepath"

	"github.com/Kuniwak/puml-parallel/cli"
)

// SocketEnv is the environment variable that overrides the csdfrepld socket path.
const SocketEnv = "CSDFREPLD_SOCK"

// SocketName is the socket's base file name within a runtime directory.
const SocketName = "csdfrepld.sock"

// ResolveSocketPath determines the csdfrepld Unix socket path shared by the
// daemon and the client. Precedence: the -sock flag value, then $CSDFREPLD_SOCK,
// then $XDG_RUNTIME_DIR/csdfrepld.sock, then <tmp>/csdfrepld.sock.
func ResolveSocketPath(flagVal string, env cli.EnvFunc) string {
	if flagVal != "" {
		return flagVal
	}
	if fromEnv := env(SocketEnv); fromEnv != "" {
		return fromEnv
	}
	if runtimeDir := env("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, SocketName)
	}
	return filepath.Join(os.TempDir(), SocketName)
}
