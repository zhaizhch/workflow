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
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

func TestNativeWorkload_GetJobStatus(t *testing.T) {
	nw := &nativeWorkload{}

	t.Run("Pending state", func(t *testing.T) {
		job := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test-job",
				},
			},
		}
		status := nw.GetJobStatus(job)
		if status.State != v1alpha1.Pending {
			t.Errorf("expected Pending state, got %v", status.State)
		}
	})

	t.Run("Running state (availableReplicas > 0)", func(t *testing.T) {
		job := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test-job",
				},
				"status": map[string]interface{}{
					"availableReplicas": int64(1),
				},
			},
		}
		status := nw.GetJobStatus(job)
		if status.State != v1alpha1.Running {
			t.Errorf("expected Running state, got %v", status.State)
		}
	})

	t.Run("Completed state (all pods succeeded)", func(t *testing.T) {
		client := kubefake.NewSimpleClientset()
		SetClient(client)
		defer SetClient(nil)

		job := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "test-job",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"replicas": int64(1),
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "test",
						},
					},
				},
			},
		}

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
				Labels: map[string]string{
					"app": "test",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodSucceeded,
			},
		}
		_, _ = client.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})

		status := nw.GetJobStatus(job)
		if status.State != v1alpha1.Completed {
			t.Errorf("expected Completed state, got %v", status.State)
		}
	})
}

func TestNativeWorkload_GetPodLabels(t *testing.T) {
	nw := &nativeWorkload{}
	job := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "test",
					},
				},
			},
		},
	}
	labels := nw.GetPodLabels(job)
	if labels["app"] != "test" {
		t.Errorf("expected label app=test, got %v", labels)
	}
}

func TestNativeWorkload_GetGVR(t *testing.T) {
	nw := &nativeWorkload{}
	gvrs := nw.GetGVR()
	if len(gvrs) != 1 || gvrs[0].Resource != "deployments" {
		t.Errorf("unexpected GVR: %v", gvrs)
	}
}
