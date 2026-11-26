package engineload

import (
	"path/filepath"
	"testing"

	"github.com/Kuniwak/puml-parallel/v7/tools/replweb"
)

func sourceOf(name string) Source {
	return func() (replweb.Engine, error) { return replweb.Engine{Glue: []byte(name)}, nil }
}

func TestChoose(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.wasm")
	tests := map[string]struct {
		path     string
		embedded Source
		want     string
	}{
		"the embedded engine":            {embedded: sourceOf("embedded"), want: "embedded"},
		"a build without one embedded":   {want: "build"},
		"the file over the embedded one": {path: missing, embedded: sourceOf("embedded"), want: "file"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			engine, err := Choose(tt.path, tt.embedded, sourceOf("build"))()

			// Assert
			got := string(engine.Glue)
			if err != nil {
				got = "file"
			}
			if got != tt.want {
				t.Errorf("chose %q, want %q", got, tt.want)
			}
		})
	}
}
