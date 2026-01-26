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

func TestPendingState_Execute(t *testing.T) {
	tests := []struct {
		name      string
		workflow  *v1alpha1.Workflow
		action    v1alpha1.Action
		wantErr   bool
		wantPhase v1alpha1.Phase
	}{
		{
			name: "SyncWorkflow Action - Transition to Running",
			workflow: &v1alpha1.Workflow{
				Status: v1alpha1.WorkflowStatus{
					State: v1alpha1.State{
						Phase: v1alpha1.Pending,
					},
				},
			},
			action:    v1alpha1.SyncWorkflowAction,
			wantErr:   false,
			wantPhase: v1alpha1.Running,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock SyncWorkflow global function
			originalSync := SyncWorkflow
			defer func() { SyncWorkflow = originalSync }()

			SyncWorkflow = func(workflow *v1alpha1.Workflow, fn UpdateWorkflowStatusFn) error {
				if fn != nil {
					workflow.Status.RunningJobs = []string{"job1"}
					fn(&workflow.Status, 1)
				}
				return nil
			}

			s := &pendingState{jobFlow: tt.workflow}
			if err := s.Execute(tt.action); (err != nil) != tt.wantErr {
				t.Errorf("pendingState.Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.workflow.Status.State.Phase != tt.wantPhase {
				t.Errorf("expected phase %v, got %v", tt.wantPhase, tt.workflow.Status.State.Phase)
			}
		})
	}
}
