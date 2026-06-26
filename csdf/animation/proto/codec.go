package proto

import (
	"encoding/json"
	"io"
)

// WriteRequest writes a request as one JSON line.
func WriteRequest(w io.Writer, req Request) error {
	return writeLine(w, req)
}

// WriteResponse writes a response as one JSON line.
func WriteResponse(w io.Writer, resp Response) error {
	return writeLine(w, resp)
}

func writeLine(w io.Writer, v any) error {
	encoded, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return nil
}

// ReadRequest reads one JSON request value from r.
func ReadRequest(r io.Reader) (Request, error) {
	var req Request
	err := json.NewDecoder(r).Decode(&req)
	return req, err
}

// ReadResponse reads one JSON response value from r.
func ReadResponse(r io.Reader) (Response, error) {
	var resp Response
	err := json.NewDecoder(r).Decode(&resp)
	return resp, err
}
