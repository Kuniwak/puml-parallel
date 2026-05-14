package pngsrc

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"os"
	"strings"
	"testing"
)

func TestExtract(t *testing.T) {
	cases := []struct {
		name    string
		input   []byte
		want    string
		wantErr bool
	}{
		{
			name:  "plain text passes through unchanged",
			input: []byte("@startuml\n@enduml"),
			want:  "@startuml\n@enduml",
		},
		{
			name:    "PNG signature only, truncated",
			input:   []byte(pngSignature),
			wantErr: true,
		},
		{
			name:    "PNG signature followed by partial chunk header",
			input:   append([]byte(pngSignature), 0x00, 0x00, 0x00),
			wantErr: true,
		},
		{
			name:    "PNG with chunk length larger than remaining bytes",
			input:   buildPNG(rawChunk{typ: "IHDR", data: bytes.Repeat([]byte{0}, 13), lenOverride: 0xFFFFFFF0}),
			wantErr: true,
		},
		{
			name:  "PNG with tEXt plantuml chunk returns raw text",
			input: buildPNG(tEXt("plantuml", "@startuml\nA --> B : tick\n@enduml"), iendChunk()),
			want:  "@startuml\nA --> B : tick\n@enduml",
		},
		{
			name:  "PNG with zTXt plantuml chunk returns decompressed text",
			input: buildPNG(zTXt("plantuml", "@startuml\nstate \"S\" as s\n@enduml"), iendChunk()),
			want:  "@startuml\nstate \"S\" as s\n@enduml",
		},
		{
			name:  "PNG with iTXt compressed plantuml chunk returns decompressed text",
			input: buildPNG(iTXt("plantuml", true, "@startuml\n[*] --> A\n@enduml"), iendChunk()),
			want:  "@startuml\n[*] --> A\n@enduml",
		},
		{
			name:  "PNG with iTXt uncompressed plantuml chunk returns text",
			input: buildPNG(iTXt("plantuml", false, "@startuml\n[*] --> B\n@enduml"), iendChunk()),
			want:  "@startuml\n[*] --> B\n@enduml",
		},
		{
			name:    "PNG has text chunks but none with plantuml keyword",
			input:   buildPNG(tEXt("Author", "alice"), tEXt("Description", "diagram"), iendChunk()),
			wantErr: true,
		},
		{
			name:  "scanner skips non-matching keyword and finds plantuml later",
			input: buildPNG(tEXt("Author", "alice"), iTXt("plantuml", true, "@startuml\n@enduml"), iendChunk()),
			want:  "@startuml\n@enduml",
		},
		{
			name: "multiple plantuml chunks: first wins",
			input: buildPNG(
				tEXt("plantuml", "FIRST"),
				iTXt("plantuml", true, "SECOND"),
				iendChunk(),
			),
			want: "FIRST",
		},
		{
			name:    "deflate bomb exceeding MaxDecompressedSize is rejected",
			input:   buildPNG(zTXt("plantuml", strings.Repeat("A", MaxDecompressedSize+1)), iendChunk()),
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Extract(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Extract: want error, got nil (result=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Extract: unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Extract: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestInflateBounded(t *testing.T) {
	compress := func(s string) []byte {
		var buf bytes.Buffer
		w := zlib.NewWriter(&buf)
		_, _ = w.Write([]byte(s))
		_ = w.Close()
		return buf.Bytes()
	}

	cases := []struct {
		name       string
		compressed []byte
		max        int64
		want       string
		wantErr    bool
	}{
		{
			name:       "round-trips a small payload",
			compressed: compress("hello world"),
			max:        1024,
			want:       "hello world",
		},
		{
			name:       "exactly at max is accepted",
			compressed: compress("ABCDE"),
			max:        5,
			want:       "ABCDE",
		},
		{
			name:       "one byte over max is rejected",
			compressed: compress("ABCDEF"),
			max:        5,
			wantErr:    true,
		},
		{
			name:       "garbage input returns zlib error",
			compressed: []byte("not zlib data"),
			max:        1024,
			wantErr:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := InflateBounded(tc.compressed, tc.max)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (result=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPNGTextChunks(t *testing.T) {
	t.Run("non-PNG input yields no chunks", func(t *testing.T) {
		var got []PNGTextChunk
		var gotErr error
		for c, err := range PNGTextChunks([]byte("not a png")) {
			gotErr = err
			got = append(got, c)
		}
		if gotErr != nil {
			t.Fatalf("unexpected error: %v", gotErr)
		}
		if len(got) != 0 {
			t.Fatalf("want empty sequence, got %d chunks", len(got))
		}
	})

	t.Run("yields all text chunks in order", func(t *testing.T) {
		input := buildPNG(
			tEXt("Author", "alice"),
			zTXt("plantuml", "@startuml\n@enduml"),
			iTXt("Note", false, "ignore me"),
			iendChunk(),
		)
		var got []PNGTextChunk
		for c, err := range PNGTextChunks(input) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got = append(got, c)
		}
		want := []PNGTextChunk{
			{Keyword: "Author", Text: "alice"},
			{Keyword: "plantuml", Text: "@startuml\n@enduml"},
			{Keyword: "Note", Text: "ignore me"},
		}
		if len(got) != len(want) {
			t.Fatalf("got %d chunks, want %d (got=%+v)", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("chunk %d: got %+v, want %+v", i, got[i], want[i])
			}
		}
	})

	t.Run("stops at IEND", func(t *testing.T) {
		input := buildPNG(
			tEXt("Author", "alice"),
			iendChunk(),
			tEXt("AfterIEND", "should be ignored"),
		)
		var got []PNGTextChunk
		for c, err := range PNGTextChunks(input) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got = append(got, c)
		}
		if len(got) != 1 || got[0].Keyword != "Author" {
			t.Fatalf("want only the chunk before IEND, got %+v", got)
		}
	})

	t.Run("malformed chunk surfaces error and terminates", func(t *testing.T) {
		input := buildPNG(rawChunk{typ: "IHDR", data: bytes.Repeat([]byte{0}, 13), lenOverride: 0xFFFFFFF0})
		var sawErr bool
		var chunks int
		for _, err := range PNGTextChunks(input) {
			if err != nil {
				sawErr = true
				break
			}
			chunks++
		}
		if !sawErr {
			t.Fatalf("want error item, got none (chunks=%d)", chunks)
		}
	})

	t.Run("early break from caller stops iteration", func(t *testing.T) {
		input := buildPNG(
			tEXt("a", "1"),
			tEXt("b", "2"),
			tEXt("c", "3"),
			iendChunk(),
		)
		var seen []string
		for c, err := range PNGTextChunks(input) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			seen = append(seen, c.Keyword)
			if c.Keyword == "b" {
				break
			}
		}
		if len(seen) != 2 || seen[0] != "a" || seen[1] != "b" {
			t.Fatalf("want [a b], got %v", seen)
		}
	})
}

func TestExtractFromRealPlantUMLPNG(t *testing.T) {
	pumlBytes, err := os.ReadFile("../examples/valid/client.puml")
	if err != nil {
		t.Fatalf("read .puml fixture: %v", err)
	}
	pngBytes, err := os.ReadFile("../examples/valid/client.png")
	if err != nil {
		t.Fatalf("read .png fixture: %v", err)
	}

	got, err := Extract(pngBytes)
	if err != nil {
		t.Fatalf("Extract(.png): %v", err)
	}

	wantTrim := strings.TrimSpace(string(pumlBytes))
	gotTrim := strings.TrimSpace(got)
	// PlantUML appends a version trailer after @enduml in the embedded chunk,
	// so we only require the original source to be preserved as a prefix.
	if !strings.HasPrefix(gotTrim, wantTrim) {
		t.Fatalf("Extract(.png) does not contain the .puml source as a prefix\n--- got ---\n%s\n--- want prefix ---\n%s\n", gotTrim, wantTrim)
	}
}

// --- test helpers: build minimal PNG byte sequences ---

type rawChunk struct {
	typ         string
	data        []byte
	lenOverride uint32 // when non-zero, write this as the length field instead of len(data)
}

func buildPNG(chunks ...rawChunk) []byte {
	var buf bytes.Buffer
	buf.WriteString(pngSignature)
	for _, c := range chunks {
		var lenBuf [4]byte
		l := uint32(len(c.data))
		if c.lenOverride != 0 {
			l = c.lenOverride
		}
		binary.BigEndian.PutUint32(lenBuf[:], l)
		buf.Write(lenBuf[:])
		buf.WriteString(c.typ)
		buf.Write(c.data)
		crc := crc32.NewIEEE()
		crc.Write([]byte(c.typ))
		crc.Write(c.data)
		var crcBuf [4]byte
		binary.BigEndian.PutUint32(crcBuf[:], crc.Sum32())
		buf.Write(crcBuf[:])
	}
	return buf.Bytes()
}

func tEXt(keyword, text string) rawChunk {
	data := append([]byte(keyword), 0)
	data = append(data, []byte(text)...)
	return rawChunk{typ: "tEXt", data: data}
}

func zTXt(keyword, text string) rawChunk {
	data := append([]byte(keyword), 0, 0) // 0 separator, 0 = deflate
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	_, _ = w.Write([]byte(text))
	_ = w.Close()
	data = append(data, z.Bytes()...)
	return rawChunk{typ: "zTXt", data: data}
}

func iTXt(keyword string, compressed bool, text string) rawChunk {
	data := append([]byte(keyword), 0)
	if compressed {
		data = append(data, 1, 0)
	} else {
		data = append(data, 0, 0)
	}
	data = append(data, 0) // language tag (empty) + null
	data = append(data, 0) // translated keyword (empty) + null
	if compressed {
		var z bytes.Buffer
		w := zlib.NewWriter(&z)
		_, _ = w.Write([]byte(text))
		_ = w.Close()
		data = append(data, z.Bytes()...)
	} else {
		data = append(data, []byte(text)...)
	}
	return rawChunk{typ: "iTXt", data: data}
}

func iendChunk() rawChunk {
	return rawChunk{typ: "IEND", data: nil}
}
