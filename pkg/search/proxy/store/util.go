package store

import (
	"encoding/base64"
	"encoding/json"
	"sort"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
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
	// todo consider to separate this two scenarios:
	// 1. client do not send ResourceVersion
	// 2. client send ResourceVersion with empty cluster.
	if len(m.rvs) == 0 {
		return ""
	}
	buf := marshalRvs(m.rvs)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func marshalRvs(rvs map[string]string) []byte {
	// We must make sure the returned ResourceVersion string is stable, because client might use this string
	// for equality comparison. But hashmap's key iteration order is not determined.
	// So we can't use json encoding library directly.
	// Instead, we convert the map to a slice, sort the slice by `Cluster` field, then manually build a string.
	// The result string resembles json encoding result to keep compatible with older version of proxy.

	if len(rvs) == 0 {
		// need to keep sync with `func (m *multiClusterResourceVersion) String() string`
		return nil
	}

	type onWireRvs struct {
		Cluster         string
		ResourceVersion string
	}

	slice := make([]onWireRvs, 0, len(rvs))

	for clusterName, version := range rvs {
		obj := onWireRvs{clusterName, version}
		slice = append(slice, obj)
	}

	sort.Slice(slice, func(i, j int) bool {
		return slice[i].Cluster < slice[j].Cluster
	})

	// Q: Why preallocate []byte with this capacity?
	// A: Consider this json `{"cluster1":"1","cluster2":"2"}`
	// For begin and end, need "{" and "}", this took 2 bytes.
	// For `"cluster1":"1",`, there's 6 bytes. And we add 14bytes for longer cluster name and resource version.
	var encoded = make([]byte, 0, (len(slice[0].Cluster)+len(slice[0].ResourceVersion)+20)*len(slice)+2)
	encoded = append(encoded, '{')
	for i, n := 0, len(slice); i < n; i++ {
		encoded = append(encoded, '"')
		encoded = append(encoded, slice[i].Cluster...)
		encoded = append(encoded, `":"`...)
		encoded = append(encoded, slice[i].ResourceVersion...)
		encoded = append(encoded, '"')

		if i != n-1 {
			encoded = append(encoded, ',')
		}
	}
	encoded = append(encoded, '}')

	return encoded
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
	lock    sync.RWMutex
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
	for _, source := range w.sources {
		go w.startWatchSource(source.watcher, source.decorator)
	}
}

// ResultChan implements watch.Interface
func (w *watchMux) ResultChan() <-chan watch.Event {
	return w.result
}

// Stop implements watch.Interface
func (w *watchMux) Stop() {
	select {
	case <-w.done:
		return
	default:
	}

	w.lock.Lock()
	defer w.lock.Unlock()

	select {
	case <-w.done:
	default:
		close(w.done)
		close(w.result)
	}
}

func (w *watchMux) startWatchSource(source watch.Interface, decorator func(watch.Event)) {
	defer source.Stop()
	defer w.Stop()
	for {
		var copyEvent watch.Event
		select {
		case sourceEvent, ok := <-source.ResultChan():
			if !ok {
				return
			}
			// sourceEvent object is cacheObject,all watcher use the same point,must deepcopy.
			copyEvent = *sourceEvent.DeepCopy()
			if decorator != nil {
				decorator(copyEvent)
			}
		case <-w.done:
			return
		}

		select {
		case <-w.done:
			return
		default:
		}

		func() {
			w.lock.RLock()
			defer w.lock.RUnlock()
			select {
			case <-w.done:
				return
			default:
				w.result <- copyEvent
			}
		}()
	}
}

func addCacheSourceAnnotation(obj runtime.Object, clusterName string) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		// Object has no meta, do nothing
		return
	}
	annotations := accessor.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[clusterv1alpha1.CacheSourceAnnotationKey] = clusterName
	accessor.SetAnnotations(annotations)
}

// RemoveCacheSourceAnnotation delete CacheSourceAnnotationKey annotation in object. If obj is updated, return true.
func RemoveCacheSourceAnnotation(obj runtime.Object) bool {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		// Object has no meta, do nothing
		return false
	}

	annotations := accessor.GetAnnotations()
	_, exist := annotations[clusterv1alpha1.CacheSourceAnnotationKey]
	if exist {
		delete(annotations, clusterv1alpha1.CacheSourceAnnotationKey)
		accessor.SetAnnotations(annotations)
		return true
	}
	return false
}

// RecoverClusterResourceVersion convert global resource version to single cluster resource version. If obj is updated, return true.
func RecoverClusterResourceVersion(obj runtime.Object, cluster string) bool {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		// Object has no meta, do nothing
		return false
	}

	rv := accessor.GetResourceVersion()
	if rv == "" || rv == "0" {
		return false
	}

	decoded, err := base64.RawURLEncoding.DecodeString(rv)
	if err != nil {
		// it's not global rv, do nothing
		return false
	}

	m := make(map[string]string)
	err = json.Unmarshal(decoded, &m)
	if err != nil {
		// it's not global rv, do nothing
		return false
	}

	crv := m[cluster]
	accessor.SetResourceVersion(crv)
	return true
}

// BuildMultiClusterResourceVersion build multi cluster resource version.
func BuildMultiClusterResourceVersion(clusterResourceMap map[string]string) string {
	m := newMultiClusterResourceVersionWithCapacity(len(clusterResourceMap))
	for cluster, rv := range clusterResourceMap {
		m.set(cluster, rv)
	}
	return m.String()
}
