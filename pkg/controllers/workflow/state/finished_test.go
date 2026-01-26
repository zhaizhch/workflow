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

func TestFinishedStates_Execute(t *testing.T) {
	tests := []struct {
		name    string
		state   State
		action  v1alpha1.Action
		wantErr bool
	}{
		{
			name: "Succeed State Sync",
			state: &succeedState{
				jobFlow: &v1alpha1.Workflow{},
			},
			action:  v1alpha1.SyncWorkflowAction,
			wantErr: false,
		},
		{
			name: "Failed State Sync",
			state: &failedState{
				jobFlow: &v1alpha1.Workflow{},
			},
			action:  v1alpha1.SyncWorkflowAction,
			wantErr: false,
		},
		{
			name: "Terminating State Sync",
			state: &terminatingState{
				jobFlow: &v1alpha1.Workflow{},
			},
			action:  v1alpha1.SyncWorkflowAction,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSync := SyncWorkflow
			defer func() { SyncWorkflow = originalSync }()

			called := false
			SyncWorkflow = func(workflow *v1alpha1.Workflow, fn UpdateWorkflowStatusFn) error {
				called = true
				if fn != nil {
					fn(&workflow.Status, 0)
				}
				return nil
			}

			if err := tt.state.Execute(tt.action); (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !called {
				t.Error("expected SyncWorkflow to be called")
			}
		})
	}
}
