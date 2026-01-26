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
	"k8s.io/apimachinery/pkg/runtime"

	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

func TestDependency_Judge(t *testing.T) {
	f := newFixture(t)

	// Create a WorkTemplate for task1
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
				Raw: []byte(`{"template":{"spec":{"containers":[{"name":"nginx"}]}}}`),
			},
		},
	}
	f.workTemplateLister = append(f.workTemplateLister, workTemplate)
	f.flowObjects = append(f.flowObjects, workTemplate)

	// Mock discovery for Volcano
	c, _, _ := f.newController()

	workflow := &v1alpha1flow.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-wf",
			Namespace: "default",
		},
		Spec: v1alpha1flow.WorkflowSpec{
			Flows: []v1alpha1flow.Flow{
				{Name: "task1"},
				{Name: "task3"},
			},
		},
		Status: v1alpha1flow.WorkflowStatus{
			JobStatusList: []v1alpha1flow.JobStatus{
				{
					Name:  "test-wf-task1", // Non-indexed job name
					State: "Completed",
				},
			},
		},
	}

	t.Run("DependsOn nil", func(t *testing.T) {
		ok, err := c.judge(workflow, nil, "task2", -1)
		if err != nil || !ok {
			t.Errorf("expected true, nil, got %v, %v", ok, err)
		}
	})

	t.Run("Simple dependency success", func(t *testing.T) {
		dependsOn := &v1alpha1flow.DependsOn{
			Targets: []string{"task1"},
		}
		ok, err := c.judge(workflow, dependsOn, "task2", -1)
		if err != nil || !ok {
			t.Errorf("expected true, nil, got %v, %v", ok, err)
		}
	})

	t.Run("Simple dependency waiting", func(t *testing.T) {
		dependsOn := &v1alpha1flow.DependsOn{
			Targets: []string{"task3"},
		}
		ok, err := c.judge(workflow, dependsOn, "task2", -1)
		if err != nil || ok {
			t.Errorf("expected false, nil, got %v, %v", ok, err)
		}
	})

	t.Run("OR groups success", func(t *testing.T) {
		dependsOn := &v1alpha1flow.DependsOn{
			OrGroups: []v1alpha1flow.DependencyGroup{
				{Targets: []string{"task3"}}, // Waiting
				{Targets: []string{"task1"}}, // Success
			},
		}
		ok, err := c.judge(workflow, dependsOn, "task2", -1)
		if err != nil || !ok {
			t.Errorf("expected true, nil, got %v, %v", ok, err)
		}
	})
}

func TestDependency_Sequential(t *testing.T) {
	f := newFixture(t)

	workTemplate := &v1alpha1flow.WorkTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "task1",
		},
		Spec: v1alpha1flow.WorkTemplateSpec{
			GVR: metav1.GroupVersionResource{Group: "batch.volcano.sh", Version: "v1alpha1", Resource: "jobs"},
		},
	}
	f.workTemplateLister = append(f.workTemplateLister, workTemplate)
	f.flowObjects = append(f.flowObjects, workTemplate)

	c, _, _ := f.newController()

	var replicas int32 = 5
	workflow := &v1alpha1flow.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "wf", Namespace: "default"},
		Spec: v1alpha1flow.WorkflowSpec{
			Flows: []v1alpha1flow.Flow{
				{
					Name: "task1",
					For: &v1alpha1flow.For{
						Replicas: &replicas,
					},
				},
			},
		},
		Status: v1alpha1flow.WorkflowStatus{
			JobStatusList: []v1alpha1flow.JobStatus{
				{Name: "wf-task1-0", State: "Completed"},
			},
		},
	}

	t.Run("Dependent on previous replica", func(t *testing.T) {
		dependsOn := &v1alpha1flow.DependsOn{
			Targets: []string{"task1"},
		}
		// task1[1] depends on task1[0]
		ok, err := c.judge(workflow, dependsOn, "task1", 1)
		if err != nil || !ok {
			t.Errorf("expected true, nil, got %v, %v", ok, err)
		}
	})

	t.Run("Dependent on previous replica - not ready", func(t *testing.T) {
		dependsOn := &v1alpha1flow.DependsOn{
			Targets: []string{"task1"},
		}
		// task1[2] depends on task1[1] (not ready)
		ok, err := c.judge(workflow, dependsOn, "task1", 2)
		if err != nil || ok {
			t.Errorf("expected false, nil, got %v, %v", ok, err)
		}
	})
}
