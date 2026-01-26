/*
Copyright 2026 zhaizhicheng.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package router

import (
	"net/http"
	"sync/atomic"

	"k8s.io/klog/v2"
)

var (
	// serverReady indicates if the server has finished initialization
	serverReady int32
)

// MarkServerReady marks the server as ready to accept traffic
func MarkServerReady() {
	atomic.StoreInt32(&serverReady, 1)
	klog.V(2).Infof("Webhook server marked as ready")
}

// IsServerReady returns true if the server is ready
func IsServerReady() bool {
	return atomic.LoadInt32(&serverReady) == 1
}

// HealthzHandler handles /healthz endpoint for liveness probe
func HealthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// ReadyzHandler handles /readyz endpoint for readiness probe
func ReadyzHandler(w http.ResponseWriter, r *http.Request) {
	if IsServerReady() {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready"))
	}
}
