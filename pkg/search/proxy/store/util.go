package store

import (
	"encoding/base64"
	"encoding/json"
	"sync"

	"k8s.io/apimachinery/pkg/watch"
)

type multiClusterResourceVersion struct {
	rvs    map[string]string
	isZero bool
}

func newMultiClusterResourceVersionWithCapacity(capacity int) *multiClusterResourceVersion {
	return &multiClusterResourceVersion{
		rvs: make(map[string]string, capacity),
	}
}

func newMultiClusterResourceVersionFromString(s string) *multiClusterResourceVersion {
	m := &multiClusterResourceVersion{
		rvs: map[string]string{},
	}
	if s == "" {
		return m
	}
	if s == "0" {
		m.isZero = true
		return m
	}

	decoded, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		// if invalid, ignore the version
		return m
	}
	// if invalid, ignore the version
	_ = json.Unmarshal(decoded, &m.rvs)
	return m
}

func (m *multiClusterResourceVersion) set(cluster, rv string) {
	m.rvs[cluster] = rv
	if rv != "0" {
		m.isZero = false
	}
}

func (m *multiClusterResourceVersion) get(cluster string) string {
	if m.isZero {
		return "0"
	}
	return m.rvs[cluster]
}

func (m *multiClusterResourceVersion) String() string {
	if m.isZero {
		return "0"
	}
	if len(m.rvs) == 0 {
		return ""
	}
	buf, _ := json.Marshal(&m.rvs)
	return base64.RawURLEncoding.EncodeToString(buf)
}

type multiClusterContinue struct {
	Cluster  string `json:"cluster,omitempty"`
	Continue string `json:"continue,omitempty"`
}

func newMultiClusterContinueFromString(s string) multiClusterContinue {
	var m multiClusterContinue
	if s == "" {
		return m
	}

	decoded, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return m
	}
	// if invalid, ignore continue
	_ = json.Unmarshal(decoded, &m)
	return m
}

func (c *multiClusterContinue) String() string {
	if c.Cluster == "" {
		return ""
	}
	buf, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(buf)
}

type decoratedWatcher struct {
	watcher   watch.Interface
	decorator func(watch.Event)
}

type watchMux struct {
	lock    sync.Mutex
	sources []decoratedWatcher
	result  chan watch.Event
	done    chan struct{}
}

func newWatchMux() *watchMux {
	return &watchMux{
		result: make(chan watch.Event),
		done:   make(chan struct{}),
	}
}

// AddSource shall be called before Start
func (w *watchMux) AddSource(watcher watch.Interface, decorator func(watch.Event)) {
	w.sources = append(w.sources, decoratedWatcher{
		watcher:   watcher,
		decorator: decorator,
	})
}

// Start run the watcher
func (w *watchMux) Start() {
	go func() {
		for _, source := range w.sources {
			go w.startWatchSource(source.watcher, source.decorator)
		}
	}()
}

// ResultChan implements watch.Interface
func (w *watchMux) ResultChan() <-chan watch.Event {
	return w.result
}

// Stop implements watch.Interface
func (w *watchMux) Stop() {
	w.lock.Lock()
	defer w.lock.Unlock()

	select {
	case <-w.done:
	default:
		close(w.done)
	}
}

func (w *watchMux) startWatchSource(source watch.Interface, decorator func(watch.Event)) {
	defer source.Stop()
	defer w.Stop()
	for {
		var event watch.Event
		var ok bool
		select {
		case event, ok = <-source.ResultChan():
			if !ok {
				return
			}
			decorator(event)
		case <-w.done:
			return
		}

		select {
		case <-w.done:
			return
		case w.result <- event:
		}
	}
}
