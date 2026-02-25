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

package state

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"github.com/workflow.sh/work-flow/pkg/controllers/workflow/state/strategy"
)

func TestIsWorkflowSuccessful(t *testing.T) {
	tests := []struct {
		name       string
		workflow   *v1alpha1.Workflow
		status     *v1alpha1.WorkflowStatus
		allJobList int
		expected   bool
	}{
		{
			name: "Strategy All: all flows succeeded",
			workflow: &v1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{Name: "wf"},
				Spec: v1alpha1.WorkflowSpec{
					SuccessPolicy: &v1alpha1.SuccessPolicy{Type: v1alpha1.SuccessPolicyAll},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				CompletedJobs: []string{"wf-f1", "wf-f2"},
			},
			allJobList: 2,
			expected:   true,
		},
		{
			name: "Strategy All: some flows failed",
			workflow: &v1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{Name: "wf"},
				Spec: v1alpha1.WorkflowSpec{
					SuccessPolicy: &v1alpha1.SuccessPolicy{Type: v1alpha1.SuccessPolicyAll},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				CompletedJobs: []string{"wf-f1"},
				FailedJobs:    []string{"wf-f2"},
			},
			allJobList: 2,
			expected:   false,
		},
		{
			name: "Strategy Any: at least one leaf succeeded (A->B, A->C, leaf: B, C)",
			workflow: &v1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{Name: "wf"},
				Spec: v1alpha1.WorkflowSpec{
					SuccessPolicy: &v1alpha1.SuccessPolicy{Type: v1alpha1.SuccessPolicyAny},
					Flows: []v1alpha1.Flow{
						{Name: "f1"},
						{Name: "f2", DependsOn: &v1alpha1.DependsOn{Targets: []string{"f1"}}},
						{Name: "f3", DependsOn: &v1alpha1.DependsOn{Targets: []string{"f1"}}},
					},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				CompletedJobs: []string{"wf-f1", "wf-f2"}, // Leaf f2 succeeded
				FailedJobs:    []string{"wf-f3"},          // Leaf f3 failed
			},
			allJobList: 3,
			expected:   true,
		},
		{
			name: "Strategy Any: no leaf succeeded",
			workflow: &v1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{Name: "wf"},
				Spec: v1alpha1.WorkflowSpec{
					SuccessPolicy: &v1alpha1.SuccessPolicy{Type: v1alpha1.SuccessPolicyAny},
					Flows: []v1alpha1.Flow{
						{Name: "f1"},
						{Name: "f2", DependsOn: &v1alpha1.DependsOn{Targets: []string{"f1"}}},
					},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				CompletedJobs: []string{"wf-f1"},
				FailedJobs:    []string{"wf-f2"},
			},
			allJobList: 2,
			expected:   false,
		},
		{
			name: "Strategy Critical: critical flows succeeded",
			workflow: &v1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{Name: "wf"},
				Spec: v1alpha1.WorkflowSpec{
					SuccessPolicy: &v1alpha1.SuccessPolicy{
						Type:          v1alpha1.SuccessPolicyCritical,
						CriticalFlows: []string{"f1", "f3"},
					},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				CompletedJobs: []string{"wf-f1", "wf-f3"},
				FailedJobs:    []string{"wf-f2"},
			},
			allJobList: 3,
			expected:   true,
		},
		{
			name: "Strategy Critical: critical flow failed",
			workflow: &v1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{Name: "wf"},
				Spec: v1alpha1.WorkflowSpec{
					SuccessPolicy: &v1alpha1.SuccessPolicy{
						Type:          v1alpha1.SuccessPolicyCritical,
						CriticalFlows: []string{"f1", "f3"},
					},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				CompletedJobs: []string{"wf-f1"},
				FailedJobs:    []string{"wf-f3"},
			},
			allJobList: 2,
			expected:   false,
		},
		{
			name: "Default strategy: All",
			workflow: &v1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{Name: "wf"},
				Spec:       v1alpha1.WorkflowSpec{},
			},
			status: &v1alpha1.WorkflowStatus{
				CompletedJobs: []string{"wf-f1", "wf-f2"},
			},
			allJobList: 2,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := strategy.GetStrategy(tt.workflow.Spec.SuccessPolicy)
			if got := s.IsSuccessful(tt.workflow, tt.status, tt.allJobList); got != tt.expected {
				t.Errorf("IsSuccessful() = %v, want %v", got, tt.expected)
			}
		})
	}
}
