package storage

import (
	"testing"

	"github.com/GAV777/httpmetricalert/internal/model"
)

// === Бенчмарки MemStorage ===

func BenchmarkMemStorage_SetGauge(b *testing.B) {
	s := NewMemStorage()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s.SetGauge("bench_gauge", float64(i))
	}
}

func BenchmarkMemStorage_SetCounter(b *testing.B) {
	s := NewMemStorage()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s.SetCounter("bench_counter", 1)
	}
}

func BenchmarkMemStorage_GetGauge(b *testing.B) {
	s := NewMemStorage()
	s.SetGauge("bench_gauge", 42.0)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = s.GetGauge("bench_gauge")
	}
}

func BenchmarkMemStorage_GetCounter(b *testing.B) {
	s := NewMemStorage()
	s.SetCounter("bench_counter", 42)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = s.GetCounter("bench_counter")
	}
}

func BenchmarkMemStorage_GetAll(b *testing.B) {
	s := NewMemStorage()
	// Заполняем хранилище 100 метриками
	for i := 0; i < 100; i++ {
		s.SetGauge("gauge_"+string(rune('A'+i%26)), float64(i))
		s.SetCounter("counter_"+string(rune('A'+i%26)), int64(i))
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = s.GetAll()
	}
}

func BenchmarkMemStorage_UpdateBatch(b *testing.B) {
	s := NewMemStorage()
	batch := makeMetrics(100)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = s.UpdateBatch(batch)
	}
}

func BenchmarkMemStorage_UpdateBatch_Small(b *testing.B) {
	s := NewMemStorage()
	batch := makeMetrics(10)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = s.UpdateBatch(batch)
	}
}

func BenchmarkMemStorage_UpdateBatch_Large(b *testing.B) {
	s := NewMemStorage()
	batch := makeMetrics(1000)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = s.UpdateBatch(batch)
	}
}

func BenchmarkMemStorage_ConcurrentSetGauge(b *testing.B) {
	s := NewMemStorage()
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.SetGauge("gauge", float64(i))
			i++
		}
	})
}

func BenchmarkMemStorage_ConcurrentSetCounter(b *testing.B) {
	s := NewMemStorage()
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.SetCounter("counter", 1)
			i++
		}
	})
}

// makeMetrics создаёт срез из n метрик (чередует gauge и counter)
func makeMetrics(n int) []model.Metrics {
	metrics := make([]model.Metrics, 0, n)
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			v := float64(i)
			metrics = append(metrics, model.Metrics{
				ID:    "bench_gauge",
				MType: model.Gauge,
				Value: &v,
			})
		} else {
			d := int64(i)
			metrics = append(metrics, model.Metrics{
				ID:    "bench_counter",
				MType: model.Counter,
				Delta: &d,
			})
		}
	}
	return metrics
}
