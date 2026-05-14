// Package pngsrc extracts PlantUML source from raw input bytes.
//
// If the input begins with the PNG signature, Extract reads the embedded
// PlantUML source from a tEXt/zTXt/iTXt chunk whose keyword is "plantuml".
// Otherwise Extract returns the input as a UTF-8 string.
//
// PNGTextChunks is exposed as a general-purpose iterator over a PNG image's
// text chunks; Extract is a thin wrapper around it.
package pngsrc

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"iter"
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

// PNGTextChunk is one decoded text chunk from a PNG image.
type PNGTextChunk struct {
	Keyword string
	Text    string
}

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
	for chunk, err := range PNGTextChunks(raw) {
		if err != nil {
			return "", err
		}
		if chunk.Keyword == plantumlKeyword {
			return chunk.Text, nil
		}
	}
	return "", ErrNoPlantUMLChunk
}

// PNGTextChunks yields each tEXt/zTXt/iTXt chunk's decoded keyword and text in
// the order they appear. Iteration stops at the IEND chunk or end of input.
//
// For non-PNG inputs the sequence is empty. Malformed PNG structure yields an
// error item and the iterator terminates; per-chunk decode errors do the same.
// Compressed text payloads are inflated up to MaxDecompressedSize bytes.
func PNGTextChunks(raw []byte) iter.Seq2[PNGTextChunk, error] {
	return func(yield func(PNGTextChunk, error) bool) {
		if !bytes.HasPrefix(raw, []byte(pngSignature)) {
			return
		}
		body := raw[len(pngSignature):]
		for pos := 0; pos < len(body); {
			if len(body)-pos < 8 {
				yield(PNGTextChunk{}, fmt.Errorf("pngsrc: truncated PNG chunk header at offset %d", pos))
				return
			}
			length := binary.BigEndian.Uint32(body[pos : pos+4])
			if length > maxChunkLen {
				yield(PNGTextChunk{}, fmt.Errorf("pngsrc: chunk length %d exceeds PNG maximum", length))
				return
			}
			typ := string(body[pos+4 : pos+8])
			dataStart := pos + 8
			if uint64(dataStart)+uint64(length)+4 > uint64(len(body)) {
				yield(PNGTextChunk{}, fmt.Errorf("pngsrc: chunk %q length %d overruns input", typ, length))
				return
			}
			data := body[dataStart : dataStart+int(length)]

			if typ == "IEND" {
				return
			}

			var (
				chunk PNGTextChunk
				ok    bool
				err   error
			)
			switch typ {
			case "tEXt":
				chunk, ok, err = decodeTEXt(data)
			case "zTXt":
				chunk, ok, err = decodeZTXt(data)
			case "iTXt":
				chunk, ok, err = decodeITXt(data)
			}
			if err != nil {
				yield(PNGTextChunk{}, err)
				return
			}
			if ok {
				if !yield(chunk, nil) {
					return
				}
			}

			pos = dataStart + int(length) + 4 // data + CRC
		}
	}
}

func decodeTEXt(data []byte) (PNGTextChunk, bool, error) {
	keyword, rest, err := splitAtNUL(data)
	if err != nil {
		return PNGTextChunk{}, false, fmt.Errorf("pngsrc: tEXt: %w", err)
	}
	return PNGTextChunk{Keyword: keyword, Text: string(rest)}, true, nil
}

func decodeZTXt(data []byte) (PNGTextChunk, bool, error) {
	keyword, rest, err := splitAtNUL(data)
	if err != nil {
		return PNGTextChunk{}, false, fmt.Errorf("pngsrc: zTXt: %w", err)
	}
	if len(rest) < 1 {
		return PNGTextChunk{}, false, errors.New("pngsrc: zTXt: missing compression method byte")
	}
	if rest[0] != 0 {
		return PNGTextChunk{}, false, fmt.Errorf("pngsrc: zTXt: unsupported compression method %d", rest[0])
	}
	text, err := inflateBounded(rest[1:])
	if err != nil {
		return PNGTextChunk{}, false, fmt.Errorf("pngsrc: zTXt: %w", err)
	}
	return PNGTextChunk{Keyword: keyword, Text: text}, true, nil
}

func decodeITXt(data []byte) (PNGTextChunk, bool, error) {
	keyword, rest, err := splitAtNUL(data)
	if err != nil {
		return PNGTextChunk{}, false, fmt.Errorf("pngsrc: iTXt: %w", err)
	}
	if len(rest) < 2 {
		return PNGTextChunk{}, false, errors.New("pngsrc: iTXt: missing compression flag/method")
	}
	compFlag, compMethod := rest[0], rest[1]
	rest = rest[2:]
	if compFlag != 0 && compFlag != 1 {
		return PNGTextChunk{}, false, fmt.Errorf("pngsrc: iTXt: invalid compression flag %d", compFlag)
	}
	if compFlag == 1 && compMethod != 0 {
		return PNGTextChunk{}, false, fmt.Errorf("pngsrc: iTXt: unsupported compression method %d", compMethod)
	}
	// Skip language tag (null-terminated) and translated keyword (null-terminated).
	_, rest, err = splitAtNUL(rest)
	if err != nil {
		return PNGTextChunk{}, false, fmt.Errorf("pngsrc: iTXt: language tag: %w", err)
	}
	_, rest, err = splitAtNUL(rest)
	if err != nil {
		return PNGTextChunk{}, false, fmt.Errorf("pngsrc: iTXt: translated keyword: %w", err)
	}
	if compFlag == 0 {
		return PNGTextChunk{Keyword: keyword, Text: string(rest)}, true, nil
	}
	text, err := inflateBounded(rest)
	if err != nil {
		return PNGTextChunk{}, false, fmt.Errorf("pngsrc: iTXt: %w", err)
	}
	return PNGTextChunk{Keyword: keyword, Text: text}, true, nil
}

func splitAtNUL(data []byte) (string, []byte, error) {
	i := bytes.IndexByte(data, 0)
	if i < 0 {
		return "", nil, errors.New("missing NUL terminator")
	}
	return string(data[:i]), data[i+1:], nil
}

func inflateBounded(compressed []byte) (string, error) {
	r, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()
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
