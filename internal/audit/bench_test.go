package audit

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// === Бенчмарки audit ===

func BenchmarkFileObserver_Notify(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "audit.log")
	obs := NewFileObserver(path)

	event := AuditEvent{
		Timestamp: 12345678,
		Metrics:   []string{"Alloc", "Frees", "GC", "Heap"},
		IPAddress: "192.168.0.42",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = obs.Notify(event)
	}
}

func BenchmarkHTTPObserver_Notify(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	obs := NewHTTPObserver(server.URL)

	event := AuditEvent{
		Timestamp: 12345678,
		Metrics:   []string{"Alloc", "Frees"},
		IPAddress: "10.0.0.1",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = obs.Notify(event)
	}
}

func BenchmarkNotifier_Notify_Single(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "audit.log")

	n := NewNotifier()
	n.AddObserver(NewFileObserver(path))

	event := AuditEvent{
		Timestamp: 12345678,
		Metrics:   []string{"Alloc"},
		IPAddress: "10.0.0.1",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n.Notify(event)
	}
}

func BenchmarkNotifier_Notify_Multiple(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "audit.log")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewNotifier()
	n.AddObserver(NewFileObserver(path))
	n.AddObserver(NewHTTPObserver(server.URL))

	event := AuditEvent{
		Timestamp: 12345678,
		Metrics:   []string{"Alloc", "Frees"},
		IPAddress: "10.0.0.1",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n.Notify(event)
	}
}

func BenchmarkAuditEvent_Marshal(b *testing.B) {
	event := AuditEvent{
		Timestamp: 12345678,
		Metrics:   []string{"Alloc", "Frees", "GC", "HeapAlloc", "Sys"},
		IPAddress: "192.168.0.42",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = event.MarshalJSON()
	}
}

func BenchmarkNotifier_Notify_LargeBatch(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "audit.log")
	n := NewNotifier()
	n.AddObserver(NewFileObserver(path))

	// Создаём событие с большим количеством метрик
	metrics := make([]string, 100)
	for i := 0; i < 100; i++ {
		metrics[i] = "metric_" + string(rune('A'+i%26))
	}
	event := AuditEvent{
		Timestamp: 12345678,
		Metrics:   metrics,
		IPAddress: "10.0.0.1",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n.Notify(event)
	}
}

func BenchmarkFileObserver_Notify_Append_Overhead(b *testing.B) {
	// Измеряем накладные расходы на открытие/закрытие файла
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "audit.log")

	// Создаём файл заранее
	f, _ := os.Create(path)
	f.Close()

	obs := NewFileObserver(path)

	event := AuditEvent{
		Timestamp: 12345678,
		Metrics:   []string{"X"},
		IPAddress: "1.2.3.4",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = obs.Notify(event)
	}
}
