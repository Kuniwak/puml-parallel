package proto

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRequestRoundTrip(t *testing.T) {
	idx := 3
	req := Request{Command: CommandSelect, Session: "2", Index: &idx, Content: []byte("@startuml\n@enduml\n")}

	var buf bytes.Buffer
	if err := WriteRequest(&buf, req); err != nil {
		t.Fatalf("WriteRequest() error = %v", err)
	}
	if !bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
		t.Errorf("WriteRequest() did not terminate the line with a newline: %q", buf.String())
	}

	got, err := ReadRequest(&buf)
	if err != nil {
		t.Fatalf("ReadRequest() error = %v", err)
	}
	if diff := cmp.Diff(req, got); diff != "" {
		t.Error(diff)
	}
}

func TestResponseRoundTrip(t *testing.T) {
	resp := Response{OK: true, Session: "1", Output: "State: Initial (s0)\n", Data: mustData(VersionData{Version: "dev"})}

	var buf bytes.Buffer
	if err := WriteResponse(&buf, resp); err != nil {
		t.Fatalf("WriteResponse() error = %v", err)
	}

	got, err := ReadResponse(&buf)
	if err != nil {
		t.Fatalf("ReadResponse() error = %v", err)
	}
	if diff := cmp.Diff(resp, got); diff != "" {
		t.Error(diff)
	}
}
