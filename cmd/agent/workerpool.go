package main

import (
	"net/http"
	"sync"
)

// workerPool — пул воркеров для отправки метрик
type workerPool struct {
	tasks   chan func()
	client  *http.Client
	baseURL string
	agentIP string
	size    int
	wg      sync.WaitGroup
	stopCh  chan struct{}
}

func newWorkerPool(size int, client *http.Client, baseURL string, agentIP string) *workerPool {
	return &workerPool{
		tasks:   make(chan func(), size*2),
		client:  client,
		baseURL: baseURL,
		agentIP: agentIP,
		size:    size,
		stopCh:  make(chan struct{}),
	}
}

func (wp *workerPool) Start() {
	for i := 0; i < wp.size; i++ {
		wp.wg.Add(1)
		go func() {
			defer wp.wg.Done()
			for {
				select {
				case task := <-wp.tasks:
					task()
				case <-wp.stopCh:
					return
				}
			}
		}()
	}
}

func (wp *workerPool) Submit(task func()) {
	wp.tasks <- task
}

func (wp *workerPool) Stop() {
	close(wp.stopCh)

	// Drain оставшихся задач из канала — выполняем их перед полным выходом
drainLoop:
	for {
		select {
		case task := <-wp.tasks:
			task()
		default:
			break drainLoop
		}
	}

	wp.wg.Wait()
}
