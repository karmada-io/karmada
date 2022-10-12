package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

func Test_watchMux_StopBySelf(t *testing.T) {
	m := newWatchMux()

	for i := 0; i < 10; i++ {
		w := newFakeWatcher(wait.NeverStop)
		m.AddSource(w, func(event watch.Event) {})
		go func() {
			ticker := time.NewTimer(time.Second)
			for {
				ticker.Reset(time.Millisecond)
				<-ticker.C
				if _, stopped := w.TryAdd(nil); stopped {
					return
				}
			}
		}()
	}

	m.Start()

	var recvCount int32
	time.AfterFunc(time.Second, m.Stop)
	timeout := time.After(time.Second * 5)
	for {
		select {
		case _, ok := <-m.ResultChan():
			if !ok {
				if recvCount == 0 {
					t.Error("receive no events")
				}
				return
			}
			recvCount++
		case <-timeout:
			t.Error("timeout")
			m.Stop()
			return
		}
	}
}

func Test_watchMux_StopBySource(t *testing.T) {
	m := newWatchMux()
	defer m.Stop()

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()

	for i := 0; i < 10; i++ {
		w := newFakeWatcher(ctx.Done())
		m.AddSource(w, func(event watch.Event) {})
		go func() {
			ticker := time.NewTimer(time.Second)
			for {
				ticker.Reset(time.Millisecond)
				<-ticker.C
				if _, stopped := w.TryAdd(nil); stopped {
					return
				}
			}
		}()
	}

	m.Start()

	var recvCount int32
	timeout := time.After(time.Second * 5)
	for {
		select {
		case _, ok := <-m.ResultChan():
			if !ok {
				if recvCount == 0 {
					t.Error("receive no events")
				}
				return
			}
			recvCount++
		case <-timeout:
			t.Error("timeout")
			return
		}
	}
}

type fakeWatcher struct {
	result  chan watch.Event
	done    chan struct{}
	stopped bool
	sync.Mutex
}

func newFakeWatcher(stopCh <-chan struct{}) *fakeWatcher {
	f := &fakeWatcher{
		result: make(chan watch.Event),
		done:   make(chan struct{}),
	}
	go func() {
		defer f.Stop()
		select {
		case <-stopCh:
			return
		case <-f.done:
			return
		}
	}()
	return f
}

func (f *fakeWatcher) Stop() {
	f.Lock()
	defer f.Unlock()
	if !f.stopped {
		close(f.result)
		close(f.done)
		f.stopped = true
	}
}

func (f *fakeWatcher) ResultChan() <-chan watch.Event {
	return f.result
}

func (f *fakeWatcher) TryAdd(obj runtime.Object) (added bool, stopped bool) {
	f.Lock()
	defer f.Unlock()
	if f.stopped {
		return false, true
	}

	select {
	case f.result <- watch.Event{Type: watch.Added, Object: obj}:
		return true, false
	default:
		return false, false
	}
}
