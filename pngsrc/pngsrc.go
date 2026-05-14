// Package pngsrc extracts PlantUML source from raw input bytes.
//
// If the input begins with the PNG signature, Extract reads the embedded
// PlantUML source from a tEXt/zTXt/iTXt chunk whose keyword is "plantuml".
// Otherwise Extract returns the input as a UTF-8 string.
package pngsrc

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const pngSignature = "\x89PNG\r\n\x1a\n"

// Keyword PlantUML writes into the PNG text chunk.
const plantumlKeyword = "plantuml"

// Maximum length, in bytes, of the chunk-length field we accept. PNG spec caps
// chunk data at 2^31-1 bytes; anything beyond that is malformed.
const maxChunkLen = 0x7FFFFFFF

// MaxDecompressedSize bounds the size of a decompressed text chunk, guarding
// against deflate-bomb inputs. PlantUML diagrams in practice are far smaller.
const MaxDecompressedSize = 16 << 20 // 16 MiB

// ErrNoPlantUMLChunk is returned when raw is a PNG that does not contain a
// text chunk with the "plantuml" keyword.
var ErrNoPlantUMLChunk = errors.New("pngsrc: no \"plantuml\" text chunk found in PNG")

// Extract returns the PlantUML source contained in raw.
//
// When raw is not a PNG, the bytes are returned verbatim as a string.
// When raw is a PNG, the first tEXt/zTXt/iTXt chunk with keyword "plantuml"
// is decoded and returned. PNG inputs without such a chunk yield
// ErrNoPlantUMLChunk.
func Extract(raw []byte) (string, error) {
	if !bytes.HasPrefix(raw, []byte(pngSignature)) {
		return string(raw), nil
	}
	return extractFromPNG(raw[len(pngSignature):])
}

func extractFromPNG(body []byte) (string, error) {
	for pos := 0; pos < len(body); {
		if len(body)-pos < 8 {
			return "", fmt.Errorf("pngsrc: truncated PNG chunk header at offset %d", pos)
		}
		length := binary.BigEndian.Uint32(body[pos : pos+4])
		if length > maxChunkLen {
			return "", fmt.Errorf("pngsrc: chunk length %d exceeds PNG maximum", length)
		}
		typ := string(body[pos+4 : pos+8])
		dataStart := pos + 8
		if uint64(dataStart)+uint64(length)+4 > uint64(len(body)) {
			return "", fmt.Errorf("pngsrc: chunk %q length %d overruns input", typ, length)
		}
		data := body[dataStart : dataStart+int(length)]

		if typ == "IEND" {
			return "", ErrNoPlantUMLChunk
		}

		switch typ {
		case "tEXt":
			if src, ok, err := readTEXt(data); err != nil {
				return "", err
			} else if ok {
				return src, nil
			}
		case "zTXt":
			if src, ok, err := readZTXt(data); err != nil {
				return "", err
			} else if ok {
				return src, nil
			}
		case "iTXt":
			if src, ok, err := readITXt(data); err != nil {
				return "", err
			} else if ok {
				return src, nil
			}
		}

		pos = dataStart + int(length) + 4 // data + CRC
	}
	return "", ErrNoPlantUMLChunk
}

// readTEXt parses a PNG tEXt chunk payload and returns its text iff the
// keyword equals "plantuml". The second return is false when the keyword
// does not match (caller should keep scanning).
func readTEXt(data []byte) (string, bool, error) {
	keyword, rest, err := splitKeyword(data)
	if err != nil {
		return "", false, fmt.Errorf("pngsrc: tEXt: %w", err)
	}
	if keyword != plantumlKeyword {
		return "", false, nil
	}
	return string(rest), true, nil
}

// splitKeyword splits a chunk payload on its first NUL byte and returns the
// keyword and the remaining bytes.
func splitKeyword(data []byte) (string, []byte, error) {
	i := bytes.IndexByte(data, 0)
	if i < 0 {
		return "", nil, errors.New("missing NUL terminator after keyword")
	}
	return string(data[:i]), data[i+1:], nil
}

// readZTXt parses a PNG zTXt chunk payload and returns its inflated text iff
// the keyword equals "plantuml".
func readZTXt(data []byte) (string, bool, error) {
	keyword, rest, err := splitKeyword(data)
	if err != nil {
		return "", false, fmt.Errorf("pngsrc: zTXt: %w", err)
	}
	if keyword != plantumlKeyword {
		return "", false, nil
	}
	if len(rest) < 1 {
		return "", false, errors.New("pngsrc: zTXt: missing compression method byte")
	}
	if rest[0] != 0 {
		return "", false, fmt.Errorf("pngsrc: zTXt: unsupported compression method %d", rest[0])
	}
	text, err := inflateBounded(rest[1:])
	if err != nil {
		return "", false, fmt.Errorf("pngsrc: zTXt: %w", err)
	}
	return text, true, nil
}

// readITXt parses a PNG iTXt chunk payload and returns its text iff the
// keyword equals "plantuml". The text is inflated when compressionFlag is 1.
func readITXt(data []byte) (string, bool, error) {
	keyword, rest, err := splitKeyword(data)
	if err != nil {
		return "", false, fmt.Errorf("pngsrc: iTXt: %w", err)
	}
	if keyword != plantumlKeyword {
		return "", false, nil
	}
	if len(rest) < 2 {
		return "", false, errors.New("pngsrc: iTXt: missing compression flag/method")
	}
	compFlag, compMethod := rest[0], rest[1]
	rest = rest[2:]
	if compFlag != 0 && compFlag != 1 {
		return "", false, fmt.Errorf("pngsrc: iTXt: invalid compression flag %d", compFlag)
	}
	if compFlag == 1 && compMethod != 0 {
		return "", false, fmt.Errorf("pngsrc: iTXt: unsupported compression method %d", compMethod)
	}
	// Skip language tag (null-terminated) and translated keyword (null-terminated).
	_, rest, err = splitKeyword(rest)
	if err != nil {
		return "", false, fmt.Errorf("pngsrc: iTXt: language tag: %w", err)
	}
	_, rest, err = splitKeyword(rest)
	if err != nil {
		return "", false, fmt.Errorf("pngsrc: iTXt: translated keyword: %w", err)
	}
	if compFlag == 0 {
		return string(rest), true, nil
	}
	text, err := inflateBounded(rest)
	if err != nil {
		return "", false, fmt.Errorf("pngsrc: iTXt: %w", err)
	}
	return text, true, nil
}

// inflateBounded decompresses zlib-encoded bytes with a hard cap on output
// length, returning an error if the output would exceed MaxDecompressedSize.
func inflateBounded(compressed []byte) (string, error) {
	r, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()
	// Read one byte past the limit so we can distinguish "exactly at limit"
	// from "exceeds limit".
	limited := io.LimitReader(r, MaxDecompressedSize+1)
	out, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if len(out) > MaxDecompressedSize {
		return "", fmt.Errorf("decompressed text exceeds %d bytes", MaxDecompressedSize)
	}
	return string(out), nil
}
