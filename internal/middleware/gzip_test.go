package middleware

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGzipMiddleware_CompressResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test response"))
	})

	mw := GzipMiddleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("expected Content-Encoding=gzip, got %q", rr.Header().Get("Content-Encoding"))
	}

	// Проверяем, что ответ сжат
	resp := rr.Result()
	defer resp.Body.Close()

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	var buf bytes.Buffer
	buf.ReadFrom(gzReader)
	if buf.String() != "test response" {
		t.Errorf("expected 'test response', got %q", buf.String())
	}
}

func TestGzipMiddleware_DecompressRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		buf.ReadFrom(r.Body)
		if buf.String() != "request body" {
			t.Errorf("expected 'request body', got %q", buf.String())
		}
		w.WriteHeader(http.StatusOK)
	})

	mw := GzipMiddleware(handler)

	// Сжимаем тело запроса
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write([]byte("request body"))
	gz.Close()

	req := httptest.NewRequest("POST", "/", &buf)
	req.Header.Set("Content-Encoding", "gzip")

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestGzipMiddleware_NoGzipSupport(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test response"))
	})

	mw := GzipMiddleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	// Не указываем Accept-Encoding: gzip

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Header().Get("Content-Encoding") == "gzip" {
		t.Error("expected no Content-Encoding when client doesn't support gzip")
	}
}
