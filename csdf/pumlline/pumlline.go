// Package pumlline holds the shapes of the PlantUML lines a global diagram
// is written in (docs/PROMOTION.md), so that every reader of one reads them
// alike. A line is matched after its surrounding space is trimmed.
package pumlline

import (
	"regexp"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/ident"
)

const id = ident.Pattern

var (
	// CompositeOpen opens a composite state: the title, then the alias.
	CompositeOpen = regexp.MustCompile(`^state\s+"((?:[^"\\]|\\.)*)"\s+as\s+(` + id + `)\s*\{$`)
	// Promote opens a <<promote>> block: the title, the alias, and "{" when
	// the block has a body.
	Promote = regexp.MustCompile(`^state\s+"((?:[^"\\]|\\.)*)"\s+as\s+(` + id + `)\s+<<promote>>\s*(\{?)$`)
	// PromoteTitle is the title of a <<promote>> block: the map, the
	// parameter an instance ID is written as, and the type of one instance.
	PromoteTitle = regexp.MustCompile(`^(\S+)\s*:\s*(.+?)\s*(?:⇸|->>)\s*(\S+)$`)
	// Close closes a composite state or a block.
	Close = regexp.MustCompile(`^}$`)
	// Include names a file to include.
	Include = regexp.MustCompile(`^!include\s+(.+)$`)
	// NoteFloating opens a note that points at nothing: its ID.
	NoteFloating = regexp.MustCompile(`^note\s+as\s+(` + id + `)$`)
	// NoteAnchored opens a note that points at a state: the state's alias.
	NoteAnchored = regexp.MustCompile(`^note\s+(?:left|right|top|bottom)\s+of\s+(` + id + `)$`)
	// NoteEnd closes a note.
	NoteEnd = regexp.MustCompile(`^end\s+note$`)
)

// Unquote is s without the double quotes around it, if it has them.
func Unquote(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		return s[1 : len(s)-1]
	}
	return s
}
