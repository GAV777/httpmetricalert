package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateHandler_Gauge(t *testing.T) {
	req := httptest.NewRequest("POST", "/update/gauge/test_metric/123.45", strings.NewReader("123.45"))
	w := httptest.NewRecorder()

	updateHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if string(body) == "" {
		t.Log("Response body is empty — OK if expected")
	}
}

func TestUpdateHandler_Counter(t *testing.T) {
	req := httptest.NewRequest("POST", "/update/counter/polls/5", strings.NewReader("5"))
	w := httptest.NewRecorder()

	updateHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
