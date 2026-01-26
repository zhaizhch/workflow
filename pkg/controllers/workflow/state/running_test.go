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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRunningState_Execute(t *testing.T) {
	retryLimit := int32(3)

	tests := []struct {
		name          string
		workflow      *v1alpha1.Workflow
		action        v1alpha1.Action
		completedJobs []string
		failedJobs    []string
		jobStatusList []v1alpha1.JobStatus
		allJobList    int
		wantPhase     v1alpha1.Phase
	}{
		{
			name: "All jobs completed -> Succeed",
			workflow: &v1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{{Name: "flow1"}},
				},
				Status: v1alpha1.WorkflowStatus{
					State: v1alpha1.State{
						Phase: v1alpha1.Running,
					},
				},
			},
			action:        v1alpha1.SyncWorkflowAction,
			completedJobs: []string{"Default/flow1"},
			failedJobs:    []string{},
			allJobList:    1,
			wantPhase:     v1alpha1.Succeed,
		},
		{
			name: "Job failed, no retry -> Failed",
			workflow: &v1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{{Name: "flow1"}}, // No Retry
				},
				Status: v1alpha1.WorkflowStatus{
					State: v1alpha1.State{
						Phase: v1alpha1.Running,
					},
				},
			},
			action:        v1alpha1.SyncWorkflowAction,
			completedJobs: []string{},
			failedJobs:    []string{"flow1"},
			allJobList:    1,
			wantPhase:     v1alpha1.Failed,
		},
		{
			name: "Job failed, retry available -> Running",
			workflow: &v1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{{
						Name: "flow1",
						Retry: &v1alpha1.Retry{
							MaxRetries: retryLimit,
						},
					}},
				},
				Status: v1alpha1.WorkflowStatus{
					State: v1alpha1.State{
						Phase: v1alpha1.Running,
					},
				},
			},
			action:        v1alpha1.SyncWorkflowAction,
			completedJobs: []string{},
			failedJobs:    []string{"flow1"},
			jobStatusList: []v1alpha1.JobStatus{
				{Name: "flow1", RestartCount: 1}, // < Limit 3
			},
			allJobList: 1,
			wantPhase:  v1alpha1.Running,
		},
		{
			name: "Job failed, retry exhausted -> Failed",
			workflow: &v1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: v1alpha1.WorkflowSpec{
					Flows: []v1alpha1.Flow{{
						Name: "flow1",
						Retry: &v1alpha1.Retry{
							MaxRetries: retryLimit,
						},
					}},
				},
				Status: v1alpha1.WorkflowStatus{
					State: v1alpha1.State{
						Phase: v1alpha1.Running,
					},
				},
			},
			action:        v1alpha1.SyncWorkflowAction,
			completedJobs: []string{},
			failedJobs:    []string{"flow1"},
			jobStatusList: []v1alpha1.JobStatus{
				{Name: "flow1", RestartCount: 3}, // >= Limit 3
			},
			allJobList: 1,
			wantPhase:  v1alpha1.Failed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSync := SyncWorkflow
			defer func() { SyncWorkflow = originalSync }()

			SyncWorkflow = func(workflow *v1alpha1.Workflow, fn UpdateWorkflowStatusFn) error {
				workflow.Status.CompletedJobs = tt.completedJobs
				workflow.Status.FailedJobs = tt.failedJobs
				workflow.Status.JobStatusList = tt.jobStatusList

				// Invoke the callback that running state uses to update status
				fn(&workflow.Status, tt.allJobList)
				return nil
			}

			s := &runningState{jobFlow: tt.workflow}
			if err := s.Execute(tt.action); err != nil {
				t.Errorf("runningState.Execute() error = %v", err)
			}

			if tt.workflow.Status.State.Phase != tt.wantPhase {
				t.Errorf("expected phase %v, got %v", tt.wantPhase, tt.workflow.Status.State.Phase)
			}
		})
	}
}
