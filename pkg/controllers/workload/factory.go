package workload

import (
	"sync"

	"k8s.io/client-go/kubernetes"
)

var (
	// groupRegistry 存储 group -> Workload 的映射，支持 O(1) 查找
	groupRegistry = map[string]Workload{}
	// workloadList 保存所有已注册的 Workload，供 GetAllWorkloads() 使用
	workloadList []Workload
	lock         sync.RWMutex
	kubeClient   kubernetes.Interface
)

// SetClient 设置全局 kubernetes 客户端，供 nativeWorkload 等使用
func SetClient(client kubernetes.Interface) {
	kubeClient = client
}

// Register 将 Workload 注册到全局注册表中。
// 同一个 Workload 支持多个 GVR（Group），每个 group 均可独立查找。
func Register(wl Workload) {
	lock.Lock()
	defer lock.Unlock()
	// 将 Workload 支持的每个 group 注册到 map 中
	for _, gvr := range wl.GetGVR() {
		groupRegistry[gvr.Group] = wl
	}
	workloadList = append(workloadList, wl)
}

// GetWorkload 根据 API Group 查找对应的 Workload 处理器，时间复杂度 O(1)。
func GetWorkload(group string) Workload {
	lock.RLock()
	defer lock.RUnlock()
	return groupRegistry[group]
}

// GetAllWorkloads 返回所有已注册的 Workload 列表。
func GetAllWorkloads() []Workload {
	lock.RLock()
	defer lock.RUnlock()
	return workloadList
}
