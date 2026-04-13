package api

import (
	"net/http"
)

// ---------------------------------------------------------------------------
// Transport — abstract network transport layer
// ---------------------------------------------------------------------------

// RequestContext carries all information about an incoming request.
type RequestContext struct {
	Method string
	Path   string
	Params map[string]string // URL path parameters (e.g. guid from /api/units/{guid})
	Query  map[string]string // URL query parameters
	Body   []byte            // raw request body
}

// Response is the unified response structure for all handlers.
type Response struct {
	Status int         `json:"-"`
	Data   interface{} `json:"data,omitempty"`
	Error  *ErrorBody  `json:"error,omitempty"`
}

// ErrorBody represents a structured error response.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HandlerFunc is the signature for all request handlers.
// Returns a Response; the transport layer handles serialization.
type HandlerFunc func(ctx *RequestContext) *Response

// StreamHandlerFunc is the signature for SSE/streaming handlers.
type StreamHandlerFunc func(ctx *RequestContext, sink EventSink)

// EventSink is the interface for pushing events to a stream subscriber.
type EventSink interface {
	// Send pushes an event to the subscriber.
	// Returns false if the subscriber is no longer active.
	Send(event interface{}) bool

	// Close terminates the stream connection.
	Close()
}

// Transport defines the abstract network transport interface.
// Implementations can be HTTP, WebSocket, or any custom protocol.
type Transport interface {
	// RegisterHandler registers a handler for a method+path combination.
	RegisterHandler(method, path string, handler HandlerFunc)

	// RegisterStream registers a streaming handler (e.g. SSE).
	RegisterStream(path string, handler StreamHandlerFunc)

	// Serve starts the transport on the given address.
	Serve(addr string) error

	// ServeTLS starts the transport with TLS on the given address.
	ServeTLS(addr, certFile, keyFile string) error
}

// HTTPStatusFromCode maps error codes to HTTP status codes.
func HTTPStatusFromCode(code string) int {
	switch code {
	case "not_found":
		return http.StatusNotFound
	case "method_not_allowed":
		return http.StatusMethodNotAllowed
	case "bad_request":
		return http.StatusBadRequest
	case "unauthorized":
		return http.StatusUnauthorized
	case "forbidden":
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

// Error codes used throughout the API.
const (
	ErrCodeBadRequest      = "bad_request"
	ErrCodeNotFound        = "not_found"
	ErrCodeMethodNotAllowed = "method_not_allowed"
	ErrCodeInternal        = "internal_error"
	ErrCodeCastFailed      = "cast_failed"
	ErrCodeInvalidJSON     = "invalid_json"
	ErrCodeMissingField    = "missing_field"
)

// ErrorResponse is a convenience function to create an error Response.
func ErrorResponse(code, message string) *Response {
	return &Response{
		Status: HTTPStatusFromCode(code),
		Error:  &ErrorBody{Code: code, Message: message},
	}
}

// SuccessResponse is a convenience function to create a success Response.
func SuccessResponse(data interface{}) *Response {
	return &Response{
		Status: http.StatusOK,
		Data:   data,
	}
}

// SuccessCreatedResponse is a convenience function for 201 responses.
func SuccessCreatedResponse(data interface{}) *Response {
	return &Response{
		Status: http.StatusCreated,
		Data:   data,
	}
}
