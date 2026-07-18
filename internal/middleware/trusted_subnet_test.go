package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustedSubnetMiddleware_AllowsTrustedIP(t *testing.T) {
	t.Parallel()

	var handlerCalled bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	mw := TrustedSubnetMiddleware("192.168.1.0/24")
	req := httptest.NewRequest("POST", "/update", nil)
	req.Header.Set("X-Real-IP", "192.168.1.100")
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("expected handler to be called for trusted IP")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestTrustedSubnetMiddleware_RejectsUntrustedIP(t *testing.T) {
	t.Parallel()

	var handlerCalled bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	mw := TrustedSubnetMiddleware("192.168.1.0/24")
	req := httptest.NewRequest("POST", "/update", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)

	if handlerCalled {
		t.Error("expected handler NOT to be called for untrusted IP")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rr.Code)
	}
}

func TestTrustedSubnetMiddleware_EmptySubnet(t *testing.T) {
	t.Parallel()

	var handlerCalled bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	mw := TrustedSubnetMiddleware("")
	req := httptest.NewRequest("POST", "/update", nil)
	// No X-Real-IP header
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("expected handler to be called when subnet is empty")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestTrustedSubnetMiddleware_MissingHeader(t *testing.T) {
	t.Parallel()

	var handlerCalled bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	mw := TrustedSubnetMiddleware("10.0.0.0/8")
	req := httptest.NewRequest("POST", "/update", nil)
	// No X-Real-IP header
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)

	if handlerCalled {
		t.Error("expected handler NOT to be called when X-Real-IP is missing")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rr.Code)
	}
}

func TestTrustedSubnetMiddleware_InvalidIP(t *testing.T) {
	t.Parallel()

	var handlerCalled bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	mw := TrustedSubnetMiddleware("10.0.0.0/8")
	req := httptest.NewRequest("POST", "/update", nil)
	req.Header.Set("X-Real-IP", "not-an-ip")
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)

	if handlerCalled {
		t.Error("expected handler NOT to be called for invalid IP")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rr.Code)
	}
}
