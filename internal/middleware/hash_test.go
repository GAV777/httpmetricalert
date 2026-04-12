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

func TestHashMiddleware_MissingHash(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	mw := HashMiddleware("secret")
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`{"id":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHashMiddleware_WrongHash(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	mw := HashMiddleware("secret")
	body := `{"id":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HashSHA256Header, "wronghash")
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHashMiddleware_CorrectHash(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	mw := HashMiddleware("secret")
	body := `{"id":"test"}`
	expectedHash := hash.Sign(body, "secret")
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HashSHA256Header, expectedHash)
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestHashMiddleware_ResponseHash(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"result":"ok"}`))
	})

	mw := HashMiddleware("secret")
	body := `{"id":"test"}`
	expectedHash := hash.Sign(body, "secret")
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HashSHA256Header, expectedHash)
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)

	responseHash := rr.Header().Get(HashSHA256Header)
	if responseHash == "" {
		t.Error("expected HashSHA256 header in response")
	}

	expectedResponseHash := hash.Sign(`{"result":"ok"}`, "secret")
	if responseHash != expectedResponseHash {
		t.Errorf("response hash = %s, want %s", responseHash, expectedResponseHash)
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
