package agent

import (
	"sync"
	"testing"
)

func TestStore_SetGauge(t *testing.T) {
	store := NewStore()
	store.SetGauge("test_gauge", 42.0)

	gauges, _ := store.Snapshot()
	found := false
	for _, m := range gauges {
		if m.ID == "test_gauge" && m.Value != nil && *m.Value == 42.0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected test_gauge with value 42.0 in snapshot")
	}
}

func TestStore_IncrCounter(t *testing.T) {
	store := NewStore()
	store.IncrCounter("test_counter", 5)
	store.IncrCounter("test_counter", 3)

	_, counters := store.Snapshot()
	found := false
	for _, m := range counters {
		if m.ID == "test_counter" && m.Delta != nil && *m.Delta == 8 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected test_counter with delta 8 in snapshot")
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store := NewStore()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			store.SetGauge("gauge", float64(i))
		}(i)
		go func(i int) {
			defer wg.Done()
			store.IncrCounter("counter", 1)
		}(i)
	}

	wg.Wait()
	_, counters := store.Snapshot()
	for _, m := range counters {
		if m.ID == "counter" && m.Delta != nil && *m.Delta == 100 {
			return
		}
	}
	t.Error("Expected counter value 100")
}
