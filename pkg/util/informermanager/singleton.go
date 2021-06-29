package informermanager

import "sync"

var instance MultiClusterInformerManager
var once sync.Once

// GetInstance returns a shared MultiClusterInformerManager instance.
func GetInstance() MultiClusterInformerManager {
	once.Do(func() {
		instance = NewMultiClusterInformerManager()
	})
	return instance
}
