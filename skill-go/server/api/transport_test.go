package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPTransport_RegisterHandler(t *testing.T) {
	transport := NewHTTPTransport()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		_ = transport.buildContext(r)
		resp := SuccessResponse(map[string]string{"hello": "world"})
		writeResponse(w, resp)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result map[string]string
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["hello"] != "world" {
		t.Errorf("expected hello=world, got %v", result["hello"])
	}
}

func TestHTTPTransport_MethodNotAllowed(t *testing.T) {
	transport := NewHTTPTransport()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed", "method_not_allowed")
			return
		}
		_ = transport.buildContext(r)
		writeResponse(w, SuccessResponse(nil))
	})

	req := httptest.NewRequest("POST", "/api/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	errObj := result["error"].(map[string]interface{})
	if errObj["code"] != "method_not_allowed" {
		t.Errorf("expected code=method_not_allowed, got %v", errObj["code"])
	}
}

func TestHTTPTransport_QueryParams(t *testing.T) {
	transport := NewHTTPTransport()

	req := httptest.NewRequest("GET", "/api/test?flow_id=123&span=combat", nil)
	ctx := transport.buildContext(req)

	if ctx.Query["flow_id"] != "123" {
		t.Errorf("expected flow_id=123, got %s", ctx.Query["flow_id"])
	}
	if ctx.Query["span"] != "combat" {
		t.Errorf("expected span=combat, got %s", ctx.Query["span"])
	}
}

func TestHTTPTransport_BodyParsing(t *testing.T) {
	transport := NewHTTPTransport()

	body := `{"spellID":38692,"targetIDs":[2,3]}`
	req := httptest.NewRequest("POST", "/api/cast", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	ctx := transport.buildContext(req)

	if string(ctx.Body) != body {
		t.Errorf("body mismatch: got %s", string(ctx.Body))
	}
}

func TestHTTPTransport_PathParams(t *testing.T) {
	transport := NewHTTPTransport()

	req := httptest.NewRequest("DELETE", "/api/units/123", nil)
	ctx := transport.buildContext(req)

	if ctx.Params["id"] != "123" {
		t.Errorf("expected id=123, got %s", ctx.Params["id"])
	}
}

func TestErrorResponse(t *testing.T) {
	resp := ErrorResponse("bad_request", "invalid JSON")

	if resp.Status != 400 {
		t.Errorf("expected status 400, got %d", resp.Status)
	}
	if resp.Error.Code != "bad_request" {
		t.Errorf("expected code=bad_request, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "invalid JSON" {
		t.Errorf("expected message=invalid JSON, got %s", resp.Error.Message)
	}
}

func TestSuccessResponse(t *testing.T) {
	data := map[string]int{"count": 42}
	resp := SuccessResponse(data)

	if resp.Status != 200 {
		t.Errorf("expected status 200, got %d", resp.Status)
	}
}

func TestWriteResponse_Success(t *testing.T) {
	w := httptest.NewRecorder()
	resp := SuccessResponse(map[string]string{"hello": "world"})
	writeResponse(w, resp)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]string
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["hello"] != "world" {
		t.Errorf("expected hello=world, got %v", result["hello"])
	}
}

func TestWriteResponse_Error(t *testing.T) {
	w := httptest.NewRecorder()
	resp := ErrorResponse("not_found", "spell 999 not found")
	writeResponse(w, resp)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["data"] != nil {
		t.Error("expected no data field for error response")
	}
	if result["error"] == nil {
		t.Fatal("expected error object")
	}
	errObj := result["error"].(map[string]interface{})
	if errObj["code"] != "not_found" {
		t.Errorf("expected code=not_found, got %v", errObj["code"])
	}
	if errObj["message"] != "spell 999 not found" {
		t.Errorf("expected message=spell 999 not found, got %v", errObj["message"])
	}
}

func TestSSESink(t *testing.T) {
	w := httptest.NewRecorder()
	sink := NewSSESink(w)

	ok := sink.Send(map[string]string{"event": "test"})
	if !ok {
		t.Error("expected Send to succeed")
	}

	body := w.Body.String()
	if !strings.Contains(body, "data: {") {
		t.Errorf("expected SSE data format, got: %s", body)
	}
	if !strings.Contains(body, "\n\n") {
		t.Error("expected SSE message separator")
	}

	sink.Close()
	ok = sink.Send(map[string]string{"event": "after_close"})
	if ok {
		t.Error("expected Send to fail after Close")
	}
}
