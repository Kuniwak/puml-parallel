//go:build !embedengine

package engineload

// Embedded is the Source of the engine embedded in the binary, or nil: a
// binary built without the embedengine tag embeds none.
func Embedded() Source { return nil }
