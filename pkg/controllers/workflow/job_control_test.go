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
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	discoveryfake "k8s.io/client-go/discovery/fake"

	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

func TestConstructJobFromTemplate(t *testing.T) {
	// Setup fixture
	f := newFixture(t)

	// Create a WorkTemplate
	jobSpec := map[string]interface{}{
		"template": map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name": "nginx",
					},
				},
			},
		},
	}
	jobSpecRaw, _ := json.Marshal(jobSpec)

	workTemplate := &v1alpha1flow.WorkTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "task1",
			Namespace: "default",
		},
		Spec: v1alpha1flow.WorkTemplateSpec{
			GVR: metav1.GroupVersionResource{
				Group:    "batch.volcano.sh",
				Version:  "v1alpha1",
				Resource: "jobs",
			},
			JobSpec: runtime.RawExtension{
				Raw: jobSpecRaw,
			},
		},
	}
	f.workTemplateLister = append(f.workTemplateLister, workTemplate)
	f.flowObjects = append(f.flowObjects, workTemplate)

	c, _, _ := f.newController()

	// Mock Discovery Resources
	fakeDiscovery, ok := c.kubeClient.Discovery().(*discoveryfake.FakeDiscovery)
	if ok {
		fakeDiscovery.Resources = []*metav1.APIResourceList{
			{
				GroupVersion: "batch.volcano.sh/v1alpha1",
				APIResources: []metav1.APIResource{
					{
						Name:       "jobs",
						Kind:       "Job",
						Namespaced: true,
					},
				},
			},
		}
	}

	workflow := &v1alpha1flow.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wf1",
			Namespace: "default",
		},
	}

	flow := v1alpha1flow.Flow{
		Name: "task1",
	}

	// Test
	job, gvr, err := c.constructJobFromTemplate(workflow, flow, "job-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gvr.Group != "batch.volcano.sh" || gvr.Resource != "jobs" {
		t.Errorf("unexpected gvr: %v", gvr)
	}

	if job.GetName() != "job-1" {
		t.Errorf("expected job name job-1, got %s", job.GetName())
	}

	// Check content
	spec, found, _ := statsNestedMap(job.Object, "spec")
	if !found {
		t.Error("spec not found in constructed job")
	}
	_ = spec
}

func statsNestedMap(obj map[string]interface{}, fields ...string) (map[string]interface{}, bool, error) {
	var val interface{} = obj
	for _, field := range fields {
		if m, ok := val.(map[string]interface{}); ok {
			val, ok = m[field]
			if !ok {
				return nil, false, nil
			}
		} else {
			return nil, false, nil
		}
	}
	m, ok := val.(map[string]interface{})
	return m, ok, nil
}
