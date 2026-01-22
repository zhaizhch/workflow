package workload

import (
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	registry = sets.New[Workload]()
	lock     sync.RWMutex
)

func Register(wl Workload) {
	lock.Lock()
	defer lock.Unlock()
	registry.Insert(wl)
}

func GetWorkload(group string) Workload {
	lock.RLock()
	defer lock.RUnlock()

	for _, wl := range registry.UnsortedList() {
		for _, gvr := range wl.GetGVR() {
			if gvr.Group == group {
				return wl
			}
		}
	}

	return nil
}

func GetAllWorkloads() []Workload {
	lock.RLock()
	defer lock.RUnlock()
	return registry.UnsortedList()
}
