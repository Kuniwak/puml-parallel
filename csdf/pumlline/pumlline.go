// Package pumlline holds the shapes of what a global diagram writes inside
// the text of a PlantUML element (docs/PROMOTION.md): the title of a
// <<promote>> block, and a path written quoted.
package pumlline

import (
	"regexp"
	"strings"
)

var (
	// PromoteTitle is the title of a <<promote>> block, <map> : (<ID> :
	// <Key>) ⇸ <Type>: the map, the parameter an instance ID is written as,
	// the type of the IDs, and the type of one instance.
	PromoteTitle = regexp.MustCompile(`^(\S+)\s*:\s*\(\s*(\S+?)\s*:\s*(.+?)\s*\)\s*(?:⇸|->>)\s*(.+?)$`)
	// PromoteTitleUntyped is a title of the form before the IDs had a type,
	// <map> : <ID> ⇸ <Type>, which is refused with a hint.
	PromoteTitleUntyped = regexp.MustCompile(`^(\S+)\s*:\s*([^()\s]+?)\s*(?:⇸|->>)\s*(\S+)$`)
)

// Unquote is s without the double quotes around it, if it has them.
func Unquote(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		return s[1 : len(s)-1]
	}
	return s
}
