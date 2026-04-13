package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ---------------------------------------------------------------------------
// SSE (Server-Sent Events) types
// ---------------------------------------------------------------------------

// SSESink implements EventSink for HTTP SSE connections.
type SSESink struct {
	w       http.ResponseWriter
	flusher http.Flusher
	closed  bool
}

// NewSSESink creates an SSE sink from an HTTP response writer.
func NewSSESink(w http.ResponseWriter) *SSESink {
	flusher, _ := w.(http.Flusher)
	return &SSESink{w: w, flusher: flusher}
}

// Send writes an event as JSON to the SSE stream.
func (s *SSESink) Send(event interface{}) bool {
	if s.closed {
		return false
	}
	data, err := json.Marshal(event)
	if err != nil {
		return false
	}
	fmt.Fprintf(s.w, "data: %s\n\n", data)
	if s.flusher != nil {
		s.flusher.Flush()
	}
	return true
}

// Close terminates the SSE connection.
func (s *SSESink) Close() {
	s.closed = true
}

// SSEHeaders writes standard SSE response headers.
func SSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
}
