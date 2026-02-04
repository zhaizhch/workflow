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

	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

func TestIsWorkflowFailed(t *testing.T) {
	tests := []struct {
		name       string
		workflow   *v1alpha1.Workflow
		status     *v1alpha1.WorkflowStatus
		wantFailed bool
	}{
		// --- Policy: All (Default) ---
		{
			name: "All: No failures -> Not Failed",
			workflow: &v1alpha1.Workflow{
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{{Name: "A"}, {Name: "B"}},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				FailedJobs: []string{},
			},
			wantFailed: false,
		},
		{
			name: "All: One failed, no retry -> Failed",
			workflow: &v1alpha1.Workflow{
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{{Name: "A"}},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				FailedJobs: []string{"wf-A"},
			},
			wantFailed: true,
		},
		{
			name: "All: One failed, ContinueOnFail -> Not Failed",
			workflow: &v1alpha1.Workflow{
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{{Name: "A", ContinueOnFail: true}},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				FailedJobs: []string{"wf-A"},
			},
			wantFailed: false,
		},
		{
			name: "All: One failed, retry available -> Not Failed",
			workflow: &v1alpha1.Workflow{
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{{Name: "A", Retry: &v1alpha1.Retry{MaxRetries: 3}}},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				FailedJobs:    []string{"wf-A"},
				JobStatusList: []v1alpha1.JobStatus{{Name: "wf-A", RestartCount: 1}},
			},
			wantFailed: false,
		},
		{
			name: "All: One failed, retry exhausted -> Failed",
			workflow: &v1alpha1.Workflow{
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{{Name: "A", Retry: &v1alpha1.Retry{MaxRetries: 3}}},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				FailedJobs:    []string{"wf-A"},
				JobStatusList: []v1alpha1.JobStatus{{Name: "wf-A", RestartCount: 3}},
			},
			wantFailed: true,
		},

		// --- Policy: Critical ---
		{
			name: "Critical: Critical flow failed -> Failed",
			workflow: &v1alpha1.Workflow{
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{{Name: "A"}, {Name: "B"}},
					SuccessPolicy: &v1alpha1.SuccessPolicy{
						Type:          v1alpha1.SuccessPolicyCritical,
						CriticalFlows: []string{"A"},
					},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				FailedJobs: []string{"wf-A"},
			},
			wantFailed: true,
		},
		{
			name: "Critical: Non-critical flow failed -> Not Failed",
			workflow: &v1alpha1.Workflow{
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{{Name: "A"}, {Name: "B"}},
					SuccessPolicy: &v1alpha1.SuccessPolicy{
						Type:          v1alpha1.SuccessPolicyCritical,
						CriticalFlows: []string{"A"},
					},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				FailedJobs: []string{"wf-B"},
			},
			wantFailed: false,
		},

		// --- Policy: Any ---
		{
			name: "Any: Path blocked (A failed, B depends on A) -> Failed",
			// A (failed) -> B. Leaf is B. Path blocked.
			workflow: &v1alpha1.Workflow{
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{
						{Name: "A"},
						{Name: "B", DependsOn: &v1alpha1.DependsOn{Targets: []string{"A"}}},
					},
					SuccessPolicy: &v1alpha1.SuccessPolicy{Type: v1alpha1.SuccessPolicyAny},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				FailedJobs: []string{"wf-A"},
			},
			wantFailed: true,
		},
		{
			name: "Any: One path open (A failed, B independent) -> Not Failed",
			// A (failed), B (ok). Leafs: A, B. A is blocked, B is not.
			workflow: &v1alpha1.Workflow{
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{
						{Name: "A"},
						{Name: "B"},
					},
					SuccessPolicy: &v1alpha1.SuccessPolicy{Type: v1alpha1.SuccessPolicyAny},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				FailedJobs: []string{"wf-A"},
			},
			wantFailed: false,
		},
		{
			name: "Any: Path recovered with ContinueOnFail -> Not Failed",
			// A (failed+continue) -> B. Leaf B. Path NOT blocked.
			workflow: &v1alpha1.Workflow{
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{
						{Name: "A", ContinueOnFail: true},
						{Name: "B", DependsOn: &v1alpha1.DependsOn{Targets: []string{"A"}}},
					},
					SuccessPolicy: &v1alpha1.SuccessPolicy{Type: v1alpha1.SuccessPolicyAny},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				FailedJobs: []string{"wf-A"},
			},
			wantFailed: false,
		},
		{
			name: "Any: Complex OR dependency (A failed, B ok, C depends on A or B) -> Not Failed",
			// C depends on (A || B). A failed, B running/ok. C not blocked.
			workflow: &v1alpha1.Workflow{
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{
						{Name: "A"},
						{Name: "B"},
						{Name: "C", DependsOn: &v1alpha1.DependsOn{
							OrGroups: []v1alpha1.DependencyGroup{
								{Targets: []string{"A"}},
								{Targets: []string{"B"}},
							},
						}},
					},
					SuccessPolicy: &v1alpha1.SuccessPolicy{Type: v1alpha1.SuccessPolicyAny},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				FailedJobs: []string{"wf-A"},
			},
			wantFailed: false,
		},
		{
			name: "Any: Complex OR dependency Blocked (A failed, B failed, C depends on A or B) -> Failed",
			// C depends on (A || B). A failed, B failed. C blocked.
			workflow: &v1alpha1.Workflow{
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{
						{Name: "A"},
						{Name: "B"},
						{Name: "C", DependsOn: &v1alpha1.DependsOn{
							OrGroups: []v1alpha1.DependencyGroup{
								{Targets: []string{"A"}},
								{Targets: []string{"B"}},
							},
						}},
					},
					SuccessPolicy: &v1alpha1.SuccessPolicy{Type: v1alpha1.SuccessPolicyAny},
				},
			},
			status: &v1alpha1.WorkflowStatus{
				FailedJobs: []string{"wf-A", "wf-B"},
			},
			wantFailed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isWorkflowFailed(tt.workflow, tt.status)
			if got != tt.wantFailed {
				t.Errorf("isWorkflowFailed() = %v, want %v", got, tt.wantFailed)
			}
		})
	}
}
