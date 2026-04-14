package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// HTTPTransport — net/http implementation of Transport
// ---------------------------------------------------------------------------

// route matches a method+path to a handler.
type route struct {
	method  string
	path    string
	handler HandlerFunc
}

// streamRoute matches a path to a stream handler.
type streamRoute struct {
	path    string
	handler StreamHandlerFunc
}

// HTTPTransport implements Transport using the standard net/http package.
type HTTPTransport struct {
	mu           sync.Mutex
	routes       []*route
	streamRoutes []*streamRoute
	middlewares  []func(http.Handler) http.Handler
	staticFS     http.FileSystem
	staticPrefix string // e.g. "/"
}

// NewHTTPTransport creates a new HTTP transport.
func NewHTTPTransport() *HTTPTransport {
	return &HTTPTransport{}
}

// Use adds middleware to the transport (applied in order).
func (t *HTTPTransport) Use(mw func(http.Handler) http.Handler) *HTTPTransport {
	t.middlewares = append(t.middlewares, mw)
	return t
}

// Static configures a static file server for the given path prefix.
func (t *HTTPTransport) Static(pathPrefix string, fs http.FileSystem) *HTTPTransport {
	t.staticFS = fs
	t.staticPrefix = pathPrefix
	return t
}

// RegisterHandler registers a handler for method+path.
// Patterns with trailing "/" are prefix matches (Go net/http convention).
func (t *HTTPTransport) RegisterHandler(method, path string, handler HandlerFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.routes = append(t.routes, &route{
		method:  method,
		path:    path,
		handler: handler,
	})
}

// RegisterStream registers a streaming handler for path.
func (t *HTTPTransport) RegisterStream(path string, handler StreamHandlerFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.streamRoutes = append(t.streamRoutes, &streamRoute{
		path:    path,
		handler: handler,
	})
}

// Serve starts the HTTP server on addr.
func (t *HTTPTransport) Serve(addr string) error {
	mux := http.NewServeMux()

	t.mu.Lock()
	for _, r := range t.routes {
		r := r // capture
		mux.HandleFunc(r.path, func(w http.ResponseWriter, req *http.Request) {
			if req.Method != r.method && r.method != "*" {
				writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed", "method_not_allowed")
				return
			}
			ctx := t.buildContext(req)
			resp := r.handler(ctx)
			writeResponse(w, resp)
		})
	}
	for _, sr := range t.streamRoutes {
		sr := sr // capture
		mux.HandleFunc(sr.path, func(w http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodGet {
				writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed", "method_not_allowed")
				return
			}
			ctx := t.buildContext(req)
			sink := NewSSESink(w)
			SSEHeaders(w)
			sr.handler(ctx, sink)
		})
	}
	t.mu.Unlock()

	// Static file server
	if t.staticFS != nil && t.staticPrefix != "" {
		fs := http.FileServer(t.staticFS)
		mux.Handle(t.staticPrefix, fs)
	}

	// Apply middlewares
	var handler http.Handler = mux
	for i := len(t.middlewares) - 1; i >= 0; i-- {
		handler = t.middlewares[i](handler)
	}

	server := &http.Server{Addr: addr, Handler: handler}
	fmt.Printf("=== skill-go Spell Demo ===\nOpen http://localhost:%s in your browser\n", addr)
	return server.ListenAndServe()
}

// ServeTLS starts the HTTPS server on addr.
func (t *HTTPTransport) ServeTLS(addr, certFile, keyFile string) error {
	mux := http.NewServeMux()

	t.mu.Lock()
	for _, r := range t.routes {
		r := r
		mux.HandleFunc(r.path, func(w http.ResponseWriter, req *http.Request) {
			if req.Method != r.method && r.method != "*" {
				writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed", "method_not_allowed")
				return
			}
			ctx := t.buildContext(req)
			resp := r.handler(ctx)
			writeResponse(w, resp)
		})
	}
	for _, sr := range t.streamRoutes {
		sr := sr
		mux.HandleFunc(sr.path, func(w http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodGet {
				writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed", "method_not_allowed")
				return
			}
			ctx := t.buildContext(req)
			sink := NewSSESink(w)
			SSEHeaders(w)
			sr.handler(ctx, sink)
		})
	}
	t.mu.Unlock()

	if t.staticFS != nil && t.staticPrefix != "" {
		fs := http.FileServer(t.staticFS)
		mux.Handle(t.staticPrefix, fs)
	}

	var handler http.Handler = mux
	for i := len(t.middlewares) - 1; i >= 0; i-- {
		handler = t.middlewares[i](handler)
	}

	server := &http.Server{Addr: addr, Handler: handler}
	return server.ListenAndServeTLS(certFile, keyFile)
}

// buildContext creates a RequestContext from an HTTP request.
func (t *HTTPTransport) buildContext(req *http.Request) *RequestContext {
	ctx := &RequestContext{
		Method: req.Method,
		Path:   req.URL.Path,
	}

	// Parse query parameters
	if req.URL.RawQuery != "" {
		ctx.Query = make(map[string]string)
		for k, v := range req.URL.Query() {
			if len(v) > 0 {
				ctx.Query[k] = v[0]
			}
		}
	}

	// Read body
	if req.Body != nil && req.ContentLength > 0 {
		body, err := io.ReadAll(io.LimitReader(req.Body, 1<<20)) // 1MB limit
		if err == nil {
			ctx.Body = body
		}
	}

	// Extract path params (simple: trim prefix)
	// More complex routing can be added later
	if strings.Contains(req.URL.Path, "/") {
		// Extract trailing ID segments like /api/units/123 or /api/spells/456
		parts := strings.Split(strings.TrimSuffix(req.URL.Path, "/"), "/")
		if len(parts) >= 4 {
			// /api/units/{guid} or /api/spells/{id}
			ctx.Params = map[string]string{"id": parts[3]}
		}
	}

	return ctx
}

// writeResponse writes a Response to the HTTP response writer.
func writeResponse(w http.ResponseWriter, resp *Response) {
	if resp.Error != nil {
		writeHTTPError(w, resp.Status, resp.Error.Message, resp.Error.Code)
		return
	}

	data, err := json.Marshal(resp.Data)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, "marshal failed", ErrCodeInternal)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(resp.Status)
	w.Write(data)
}

// writeHTTPError writes a structured error response.
func writeHTTPError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
