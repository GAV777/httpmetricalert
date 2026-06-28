package pool

import "sync"

// Resetter — интерфейс для типов, поддерживающих сброс состояния.
type Resetter interface {
	Reset()
}

// Pool — обёртка над sync.Pool для объектов с методом Reset().
// При получении объекта из пула его состояние сбрасывается через Reset(),
// что гарантирует чистое состояние для каждого пользователя.
type Pool[T Resetter] struct {
	pool    sync.Pool
	factory func() T
}

// New создаёт и возвращает новый Pool для типа T.
func New[T Resetter](factory func() T) *Pool[T] {
	p := &Pool[T]{
		factory: factory,
	}
	p.pool = sync.Pool{
		New: func() interface{} {
			return factory()
		},
	}
	return p
}

// Get возвращает объект из пула.
// Если пул пуст, создаётся новый объект через factory.
// Перед возвратом вызывается Reset() для очистки состояния.
func (p *Pool[T]) Get() T {
	obj := p.pool.Get().(T)
	obj.Reset()
	return obj
}

// Put помещает объект обратно в пул для последующего переиспользования.
func (p *Pool[T]) Put(obj T) {
	p.pool.Put(obj)
}
