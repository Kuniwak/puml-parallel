package tools

import "errors"

// UserFacingError formats err for end-user display. With debug it returns the
// full wrapped chain (err.Error()); otherwise it unwraps to the deepest error
// and returns its message, hiding internal package-qualified context. A nil
// error yields the empty string.
func UserFacingError(err error, debug bool) string {
	if err == nil {
		return ""
	}
	if debug {
		return err.Error()
	}
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err.Error()
		}
		err = unwrapped
	}
}
