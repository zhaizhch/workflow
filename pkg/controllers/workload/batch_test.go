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

package workload

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

func TestBatchWorkload_GetJobStatus(t *testing.T) {
	bw := &batchWorkload{}

	t.Run("Status from phase", func(t *testing.T) {
		job := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": map[string]interface{}{
					"state": map[string]interface{}{
						"phase": "Running",
					},
				},
			},
		}
		status := bw.GetJobStatus(job)
		if status.State != v1alpha1.Running {
			t.Errorf("expected Running state, got %v", status.State)
		}
	})

	t.Run("Status from succeeded count", func(t *testing.T) {
		job := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": map[string]interface{}{
					"succeeded": int64(1),
				},
			},
		}
		status := bw.GetJobStatus(job)
		if status.State != v1alpha1.Completed {
			t.Errorf("expected Completed state, got %v", status.State)
		}
	})
}
