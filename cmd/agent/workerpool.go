package main

import (
	"net/http"
	"sync"
)

// workerPool — пул воркеров для отправки метрик
type workerPool struct {
	tasks     chan func()
	client    *http.Client
	baseURL   string
	agentIP   string
	secretKey string
	size      int
	wg        sync.WaitGroup
	stopCh    chan struct{}
}

func newWorkerPool(size int, client *http.Client, baseURL string, agentIP string, secretKey string) *workerPool {
	return &workerPool{
		tasks:     make(chan func(), size*2),
		client:    client,
		baseURL:   baseURL,
		agentIP:   agentIP,
		secretKey: secretKey,
		size:      size,
		stopCh:    make(chan struct{}),
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

	// Ждём завершения всех воркеров
	wp.wg.Wait()

	// Drain оставшихся задач из канала — выполняем их после выхода воркеров
drainLoop:
	for {
		select {
		case task := <-wp.tasks:
			task()
		default:
			break drainLoop
		}
	}
}
