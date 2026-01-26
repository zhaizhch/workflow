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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"github.com/workflow.sh/work-flow/pkg/apis/helpers"
)

func TestAddWorkflow(t *testing.T) {
	f := newFixture(t)
	c, _, _ := f.newController()

	workflow := &v1alpha1flow.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-wf",
			Namespace: "default",
		},
	}

	c.addWorkflow(workflow)

	// Check if enqueued
	found := false
	for _, q := range c.queues {
		if q.Len() > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected workflow to be enqueued")
	}
}

func TestUpdateWorkflow(t *testing.T) {
	f := newFixture(t)
	c, _, _ := f.newController()

	oldWf := &v1alpha1flow.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-wf",
			Namespace:       "default",
			ResourceVersion: "1",
		},
	}
	newWf := oldWf.DeepCopy()
	newWf.ResourceVersion = "2"

	t.Run("ResourceVersion changed", func(t *testing.T) {
		c.updateWorkflow(oldWf, newWf)
		if c.queues[0].Len() == 0 { // Simple check, might be in other shard but only 1 worker in test
			t.Error("expected workflow to be enqueued")
		}
	})

	t.Run("ResourceVersion same", func(t *testing.T) {
		// Clear queue
		for _, q := range c.queues {
			for q.Len() > 0 {
				q.Get()
			}
		}
		c.updateWorkflow(newWf, newWf)
		for _, q := range c.queues {
			if q.Len() > 0 {
				t.Error("did not expect workflow to be enqueued when ResourceVersion same")
			}
		}
	})
}

func TestUpdateGenericJob(t *testing.T) {
	f := newFixture(t)
	c, _, _ := f.newController()

	job := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "batch.volcano.sh/v1alpha1",
			"kind":       "Job",
			"metadata": map[string]interface{}{
				"name":            "test-job",
				"namespace":       "default",
				"resourceVersion": "1",
				"labels": map[string]interface{}{
					CreatedByWorkflow: GenerateObjectString("default", "test-wf"),
				},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion": v1alpha1flow.SchemeGroupVersion.String(),
						"kind":       helpers.WorkflowKind.Kind,
						"name":       "test-wf",
						"uid":        "test-uid",
						"controller": true,
					},
				},
			},
			"status": map[string]interface{}{
				"state": map[string]interface{}{
					"phase": "Pending",
				},
			},
		},
	}

	newJob := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "batch.volcano.sh/v1alpha1",
			"kind":       "Job",
			"metadata": map[string]interface{}{
				"name":            "test-job",
				"namespace":       "default",
				"resourceVersion": "2",
				"labels": map[string]interface{}{
					CreatedByWorkflow: GenerateObjectString("default", "test-wf"),
				},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion": v1alpha1flow.SchemeGroupVersion.String(),
						"kind":       helpers.WorkflowKind.Kind,
						"name":       "test-wf",
						"uid":        "test-uid",
						"controller": true,
					},
				},
			},
			"status": map[string]interface{}{
				"state": map[string]interface{}{
					"phase": "Running",
				},
			},
		},
	}

	c.updateGenericJob(job, newJob)

	found := false
	for _, q := range c.queues {
		if q.Len() > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected workflow to be enqueued on job update")
	}
}
