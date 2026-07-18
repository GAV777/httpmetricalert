package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GAV777/httpmetricalert/pkg/hash"
)

func TestHashMiddleware_NoKey(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	mw := HashMiddleware("")
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`{"id":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get(HashSHA256Header) != "" {
		t.Error("expected no HashSHA256 header when key is empty")
	}
}

func TestHashMiddleware_WithKey(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"result":"ok"}`))
	})

	mw := HashMiddleware("secret")
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`{"id":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	responseHash := rr.Header().Get(HashSHA256Header)
	if responseHash == "" {
		t.Error("expected HashSHA256 header in response")
	}

	expectedHash := hash.Sign(`{"result":"ok"}`, "secret")
	if responseHash != expectedHash {
		t.Errorf("response hash = %s, want %s", responseHash, expectedHash)
	}
}

func TestHashMiddleware_GetRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	})

	mw := HashMiddleware("secret")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for GET, got %d", rr.Code)
	}
}

func TestHashMiddleware_NoBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mw := HashMiddleware("secret")
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}
	// Нет тела — нет хеша
	if rr.Header().Get(HashSHA256Header) != "" {
		t.Error("expected no HashSHA256 header when response body is empty")
	}
}
