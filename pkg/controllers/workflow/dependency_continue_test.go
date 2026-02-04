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

package workflow

import (
	"testing"

	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDependency_ContinueOnFail(t *testing.T) {
	f := newFixture(t)

	// Create a WorkTemplate for task "A"
	workTemplateA := &v1alpha1flow.WorkTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "A", Namespace: "default"},
		Spec: v1alpha1flow.WorkTemplateSpec{
			GVR:     metav1.GroupVersionResource{Group: "batch.volcano.sh", Version: "v1alpha1", Resource: "jobs"},
			JobSpec: runtime.RawExtension{Raw: []byte(`{"template":{}}`)}, // simplified
		},
	}
	f.workTemplateLister = append(f.workTemplateLister, workTemplateA)
	f.flowObjects = append(f.flowObjects, workTemplateA)

	workflow := &v1alpha1flow.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "wf", Namespace: "default"},
		Spec: v1alpha1flow.WorkflowSpec{
			Flows: []v1alpha1flow.Flow{
				{Name: "A", ContinueOnFail: true},
				{Name: "B", DependsOn: &v1alpha1flow.DependsOn{Targets: []string{"A"}}},
			},
		},
		Status: v1alpha1flow.WorkflowStatus{
			JobStatusList: []v1alpha1flow.JobStatus{
				{Name: "wf-A", State: "Failed"}, // Job A failed
			},
		},
	}

	t.Run("Dependency satisfied if target failed but ContinueOnFail is true", func(t *testing.T) {
		// Mock the failed job in the cluster
		job := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "batch.volcano.sh/v1alpha1",
				"kind":       "Job",
				"metadata": map[string]interface{}{
					"name":      "wf-A", // Not indexed
					"namespace": "default",
				},
				"status": map[string]interface{}{
					"state": map[string]interface{}{
						"phase": "Failed",
					},
				},
			},
		}
		f.dynamicObjects = append(f.dynamicObjects, job)

		c, _, _ := f.newController()

		// Test judge
		// A is failed. B depends on A.
		// A has ContinueOnFail = true.
		// judge(A failed) -> should return true (satisfied)

		dependsOn := &v1alpha1flow.DependsOn{Targets: []string{"A"}}
		ok, err := c.judge(workflow, dependsOn, "B", -1)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !ok {
			t.Errorf("expected dependency to be satisfied due to ContinueOnFail, but got false")
		}
	})
}
