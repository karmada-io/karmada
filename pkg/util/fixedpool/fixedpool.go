package fixedpool

import (
	"sync"

	"github.com/karmada-io/karmada/pkg/metrics"
)

// A FixedPool like sync.Pool. But it's limited capacity.
// When pool is full, Put will abandon and call destroyFunc to destroy the object.
type FixedPool struct {
	name     string
	lock     sync.Mutex
	pool     []any
	capacity int

	newFunc     func() (any, error)
	destroyFunc func(any)
}

// New return a FixedPool
func New(name string, newFunc func() (any, error), destroyFunc func(any), capacity int) *FixedPool {
	return &FixedPool{
		name:        name,
		pool:        make([]any, 0, capacity),
		capacity:    capacity,
		newFunc:     newFunc,
		destroyFunc: destroyFunc,
	}
}

// Get selects an arbitrary item from the pool, removes it from the
// pool, and returns it to the caller.
// Get may choose to ignore the pool and treat it as empty.
// Callers should not assume any relation between values passed to Put and
// the values returned by Get.
//
// If pool is empty, Get returns the result of calling newFunc.
func (p *FixedPool) Get() (any, error) {
	o, ok := p.pop()
	if ok {
		metrics.RecordPoolGet(p.name, false)
		return o, nil
	}

	metrics.RecordPoolGet(p.name, true)
	return p.newFunc()
}

// Put adds x to the pool. If pool is full, x will be abandoned,
// and it's destroy function will be called.
func (p *FixedPool) Put(x any) {
	if p.push(x) {
		metrics.RecordPoolPut(p.name, false)
		return
	}
	metrics.RecordPoolPut(p.name, true)
	p.destroyFunc(x)
}

func (p *FixedPool) pop() (any, bool) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if s := len(p.pool); s > 0 {
		o := p.pool[s-1]
		p.pool = p.pool[:s-1]
		return o, true
	}
	return nil, false
}

func (p *FixedPool) push(o any) bool {
	p.lock.Lock()
	defer p.lock.Unlock()
	if s := len(p.pool); s < p.capacity {
		p.pool = append(p.pool, o)
		return true
	}
	return false
}
