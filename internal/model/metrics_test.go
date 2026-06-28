package model

import (
	"encoding/json"
	"testing"
)

func TestMetricsConstants(t *testing.T) {
	if Counter != "counter" {
		t.Errorf("Counter = %q, want %q", Counter, "counter")
	}
	if Gauge != "gauge" {
		t.Errorf("Gauge = %q, want %q", Gauge, "gauge")
	}
}

func TestMetricsJSONSerialization(t *testing.T) {
	// Тест для gauge-метрики
	value := 123.45
	gauge := Metrics{
		ID:    "Alloc",
		MType: "gauge",
		Value: &value,
	}

	data, err := json.Marshal(gauge)
	if err != nil {
		t.Fatalf("failed to marshal gauge: %v", err)
	}

	var parsed Metrics
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal gauge: %v", err)
	}

	if parsed.ID != gauge.ID || parsed.MType != gauge.MType {
		t.Errorf("gauge mismatch: got %+v, want %+v", parsed, gauge)
	}
	if parsed.Value == nil || *parsed.Value != *gauge.Value {
		t.Errorf("gauge value mismatch")
	}
	if parsed.Delta != nil {
		t.Error("gauge Delta should be nil")
	}
}

func TestMetricsCounterJSONSerialization(t *testing.T) {
	// Тест для counter-метрики
	delta := int64(42)
	counter := Metrics{
		ID:    "PollCount",
		MType: "counter",
		Delta: &delta,
	}

	data, err := json.Marshal(counter)
	if err != nil {
		t.Fatalf("failed to marshal counter: %v", err)
	}

	var parsed Metrics
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal counter: %v", err)
	}

	if parsed.ID != counter.ID || parsed.MType != counter.MType {
		t.Errorf("counter mismatch: got %+v, want %+v", parsed, counter)
	}
	if parsed.Delta == nil || *parsed.Delta != *counter.Delta {
		t.Errorf("counter delta mismatch")
	}
	if parsed.Value != nil {
		t.Error("counter Value should be nil")
	}
}

func TestMetricsHash(t *testing.T) {
	value := 100.0
	m := Metrics{
		ID:    "Test",
		MType: "gauge",
		Value: &value,
		Hash:  "abc123",
	}

	data, _ := json.Marshal(m)
	var parsed Metrics
	json.Unmarshal(data, &parsed)

	if parsed.Hash != "abc123" {
		t.Errorf("Hash = %q, want %q", parsed.Hash, "abc123")
	}
}
