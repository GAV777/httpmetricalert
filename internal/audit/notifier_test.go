package audit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditEvent_MarshalJSON(t *testing.T) {
	t.Parallel()

	event := AuditEvent{
		Timestamp: 12345678,
		Metrics:   []string{"Alloc", "Frees"},
		IPAddress: "192.168.0.42",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded AuditEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Timestamp != event.Timestamp {
		t.Errorf("Timestamp: got %d, want %d", decoded.Timestamp, event.Timestamp)
	}
	if decoded.IPAddress != event.IPAddress {
		t.Errorf("IPAddress: got %q, want %q", decoded.IPAddress, event.IPAddress)
	}
	if len(decoded.Metrics) != len(event.Metrics) {
		t.Errorf("Metrics count: got %d, want %d", len(decoded.Metrics), len(event.Metrics))
	}
}

func TestNotifier_NoObservers(t *testing.T) {
	t.Parallel()

	n := NewNotifier()
	if n.HasObservers() {
		t.Error("Expected HasObservers() to be false for empty notifier")
	}
}

func TestNotifier_AddObserver(t *testing.T) {
	t.Parallel()

	n := NewNotifier()
	n.AddObserver(&testObserver{})
	if !n.HasObservers() {
		t.Error("Expected HasObservers() to be true after adding observer")
	}
}

func TestFileObserver_Notify(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filepath := filepath.Join(tmpDir, "audit.log")

	obs := NewFileObserver(filepath)

	event := AuditEvent{
		Timestamp: 12345678,
		Metrics:   []string{"Alloc"},
		IPAddress: "10.0.0.1",
	}

	if err := obs.Notify(event); err != nil {
		t.Fatalf("Notify failed: %v", err)
	}

	// Проверяем что файл создан и содержит JSON
	data, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var decoded AuditEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to parse JSON from file: %v", err)
	}

	if decoded.Timestamp != event.Timestamp {
		t.Errorf("Timestamp mismatch: got %d, want %d", decoded.Timestamp, event.Timestamp)
	}
}

func TestFileObserver_Append(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filepath := filepath.Join(tmpDir, "audit.log")

	obs := NewFileObserver(filepath)

	// Записываем два события
	obs.Notify(AuditEvent{Timestamp: 1, Metrics: []string{"A"}, IPAddress: "1.1.1.1"})
	obs.Notify(AuditEvent{Timestamp: 2, Metrics: []string{"B"}, IPAddress: "2.2.2.2"})

	data, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	lines := splitLines(string(data))
	if len(lines) != 2 {
		t.Fatalf("Expected 2 lines, got %d", len(lines))
	}
}

func TestNotifier_NotifyAll(t *testing.T) {
	t.Parallel()

	n := NewNotifier()
	obs1 := &countingObserver{}
	obs2 := &countingObserver{}
	n.AddObserver(obs1)
	n.AddObserver(obs2)

	n.Notify(AuditEvent{Timestamp: 1, Metrics: []string{"X"}, IPAddress: "1.2.3.4"})

	if obs1.count != 1 {
		t.Errorf("obs1: got %d notifications, want 1", obs1.count)
	}
	if obs2.count != 1 {
		t.Errorf("obs2: got %d notifications, want 1", obs2.count)
	}
}

func TestNotifier_Close(t *testing.T) {
	t.Parallel()

	n := NewNotifier()
	obs := &countingObserver{}
	n.AddObserver(obs)

	n.Close()

	if !obs.closed {
		t.Error("Expected observer to be closed")
	}
}

func TestHTTPObserver_Notify(t *testing.T) {
	t.Parallel()

	// Создаём тестовый HTTP-сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", ct)
		}

		var event AuditEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}
		if event.Timestamp != 999 {
			t.Errorf("Timestamp: got %d, want 999", event.Timestamp)
		}
	}))
	defer server.Close()

	obs := NewHTTPObserver(server.URL)
	err := obs.Notify(AuditEvent{Timestamp: 999, Metrics: []string{"Test"}, IPAddress: "5.5.5.5"})
	if err != nil {
		t.Fatalf("Notify failed: %v", err)
	}
}

// --- Вспомогательные типы ---

type testObserver struct{}

func (t *testObserver) Notify(event AuditEvent) error { return nil }
func (t *testObserver) Close() error                  { return nil }

type countingObserver struct {
	count  int
	closed bool
}

func (c *countingObserver) Notify(event AuditEvent) error {
	c.count++
	return nil
}
func (c *countingObserver) Close() error {
	c.closed = true
	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
