package pool

import (
	"sync"
	"testing"
)

// testStruct — тестовая структура с методом Reset()
type testStruct struct {
	value int
	data  []string
	name  string
}

func (ts *testStruct) Reset() {
	ts.value = 0
	ts.data = ts.data[:0]
	ts.name = ""
}

func TestPool_GetReturnsCleanObject(t *testing.T) {
	p := New(func() *testStruct { return &testStruct{} })

	obj := p.Get()
	if obj.value != 0 {
		t.Errorf("expected value=0, got %d", obj.value)
	}
	if obj.name != "" {
		t.Errorf("expected name=empty, got %q", obj.name)
	}
	if len(obj.data) != 0 {
		t.Errorf("expected empty data slice, got %d elements", len(obj.data))
	}
}

func TestPool_PutAndGetReturnsResetObject(t *testing.T) {
	p := New(func() *testStruct { return &testStruct{} })

	obj := p.Get()
	obj.value = 42
	obj.name = "dirty"
	obj.data = append(obj.data, "a", "b", "c")

	p.Put(obj)

	// Получаем тот же объект — он должен быть сброшен
	obj2 := p.Get()
	if obj2.value != 0 {
		t.Errorf("expected value=0 after Get, got %d", obj2.value)
	}
	if obj2.name != "" {
		t.Errorf("expected name=empty after Get, got %q", obj2.name)
	}
	if len(obj2.data) != 0 {
		t.Errorf("expected empty data slice after Get, got %d elements", len(obj2.data))
	}
}

func TestPool_ConcurrentAccess(t *testing.T) {
	p := New(func() *testStruct { return &testStruct{} })

	var wg sync.WaitGroup
	const goroutines = 100
	const iterations = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				obj := p.Get()
				obj.value = j
				obj.name = "test"
				obj.data = append(obj.data, "item")

				// Проверяем, что объект «чистый» при получении
				if obj.value != j {
					t.Errorf("concurrent: expected value=%d, got %d", j, obj.value)
				}

				p.Put(obj)
			}
		}()
	}

	wg.Wait()
}

func TestPool_MultipleGetsCreateNewObjects(t *testing.T) {
	p := New(func() *testStruct { return &testStruct{} })

	obj1 := p.Get()
	obj2 := p.Get()

	// Оба объекта должны быть валидными и независимыми
	obj1.value = 1
	obj2.value = 2

	if obj1.value != 1 || obj2.value != 2 {
		t.Errorf("objects should be independent: obj1.value=%d, obj2.value=%d", obj1.value, obj2.value)
	}

	p.Put(obj1)
	p.Put(obj2)
}
