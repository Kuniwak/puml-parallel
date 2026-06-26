package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
)

func TestResolveSocketPath(t *testing.T) {
	tests := []struct {
		name    string
		flagVal string
		env     map[string]string
		want    string
	}{
		{
			name:    "flag wins over everything",
			flagVal: "/custom/csdfrepld.sock",
			env:     map[string]string{SocketEnv: "/env.sock", "XDG_RUNTIME_DIR": "/run/user/1000"},
			want:    "/custom/csdfrepld.sock",
		},
		{
			name: "env wins over XDG",
			env:  map[string]string{SocketEnv: "/env.sock", "XDG_RUNTIME_DIR": "/run/user/1000"},
			want: "/env.sock",
		},
		{
			name: "XDG runtime dir",
			env:  map[string]string{"XDG_RUNTIME_DIR": "/run/user/1000"},
			want: "/run/user/1000/" + SocketName,
		},
		{
			name: "tmp fallback",
			env:  map[string]string{},
			want: filepath.Join(os.TempDir(), SocketName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSocketPath(tt.flagVal, cli.NewEnvFunc(tt.env))
			if got != tt.want {
				t.Errorf("ResolveSocketPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
