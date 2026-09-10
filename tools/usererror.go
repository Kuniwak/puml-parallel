package tools

import (
	"errors"
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/usererr"
)

// UserFacing is implemented by an error whose own message is the one to show.
// It is usererr.UserFacing under the name this layer has always called it.
type UserFacing = usererr.UserFacing

// UserFacingError formats err for end-user display. With debug it returns the
// full wrapped chain (err.Error()); otherwise it unwraps to the deepest error
// that is either the last one or a UserFacing one, and returns its message,
// hiding internal package-qualified context. A nil error yields the empty
// string.
func UserFacingError(err error, debug bool) string {
	if err == nil {
		return ""
	}
	if debug {
		return err.Error()
	}
	return withPromotionAdvice(usererr.Message(err), err)
}

// withPromotionAdvice names the tool that expands a promotion. csdf reports the
// fact - the source still holds directives - and which command to run is this
// layer's to know.
func withPromotionAdvice(message string, err error) string {
	var hinted *csdf.PromotionHintError
	if errors.As(err, &hinted) {
		return fmt.Sprintf("%s; run csdfpromote on it first", message)
	}
	return message
}
