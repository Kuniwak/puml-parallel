// Package pngsrc extracts PlantUML source from raw input bytes.
//
// At this commit the function is a pass-through; later commits add PNG
// metadata parsing for inputs that begin with the PNG signature.
package pngsrc

// Extract returns the PlantUML source contained in raw.
func Extract(raw []byte) (string, error) {
	return string(raw), nil
}
