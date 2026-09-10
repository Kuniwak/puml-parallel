// Package usererr turns an error into the one line to show its reader. It sits
// below both the CLI layer and the checks, because both have to say the same
// thing about the same error: a tool printing "Error: ..." and a lint finding
// quoting a parse failure differ in where the message goes, not in which
// message it is.
package usererr

import "errors"

// UserFacing is implemented by an error whose own message is the one to show.
// Unwrapping stops there: it has already been written for the reader, and going
// past it would drop what it adds. csdf.PromotionHintError is one - it wraps a
// parse error so that a caller can still reach it, and says what the author is
// missing on top.
type UserFacing interface{ UserFacing() }

// Message unwraps err to the deepest error that is either the last one or a
// UserFacing one, and returns its message, hiding the internal
// package-qualified context of the wrapping. A nil error yields the empty
// string.
func Message(err error) string {
	if err == nil {
		return ""
	}
	for {
		if _, ok := err.(UserFacing); ok {
			return err.Error()
		}
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err.Error()
		}
		err = unwrapped
	}
}
