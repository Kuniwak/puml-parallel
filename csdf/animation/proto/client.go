package proto

import "io"

// Do performs one request/response round trip over conn: it writes the request
// line and reads the single response line. It is transport-agnostic; callers
// supply a connected stream (e.g. a dialed Unix socket).
func Do(conn io.ReadWriter, req Request) (Response, error) {
	if err := WriteRequest(conn, req); err != nil {
		return Response{}, err
	}
	return ReadResponse(conn)
}
