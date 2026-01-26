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
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

func TestKubeflowWorkload_GetJobStatus(t *testing.T) {
	kw := &kubeflowWorkload{}

	t.Run("Status from conditions", func(t *testing.T) {
		job := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":               "Created",
							"status":             "True",
							"lastTransitionTime": time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
						},
						map[string]interface{}{
							"type":               "Succeeded",
							"status":             "True",
							"lastTransitionTime": time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
						},
					},
				},
			},
		}
		status := kw.GetJobStatus(job)
		if status.State != v1alpha1.Completed {
			t.Errorf("expected Completed state, got %v", status.State)
		}
	})

	t.Run("Status from Failed condition", func(t *testing.T) {
		job := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":               "Failed",
							"status":             "True",
							"lastTransitionTime": time.Now().Format(time.RFC3339),
						},
					},
				},
			},
		}
		status := kw.GetJobStatus(job)
		if status.State != v1alpha1.Failed {
			t.Errorf("expected Failed state, got %v", status.State)
		}
	})
}
