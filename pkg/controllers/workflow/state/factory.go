/*
Copyright 2022 The Volcano Authors.

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
	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

type State interface {
	// Execute executes the actions based on current state.
	Execute(action v1alpha1.Action) error
}

// UpdateWorkflowStatusFn updates the jobFlow status.
type UpdateWorkflowStatusFn func(status *v1alpha1.WorkflowStatus, allJobList int)

type WorkflowActionFn func(workflow *v1alpha1.Workflow, fn UpdateWorkflowStatusFn) error

var (
	// SyncWorkflow will sync queue status.
	SyncWorkflow WorkflowActionFn
)

// NewState gets the state from queue status.
func NewState(jobFlow *v1alpha1.Workflow) State {
	switch jobFlow.Status.State.Phase {
	case "", v1alpha1.Pending:
		return &pendingState{jobFlow: jobFlow}
	case v1alpha1.Running:
		return &runningState{jobFlow: jobFlow}
	case v1alpha1.Succeed:
		return &succeedState{jobFlow: jobFlow}
	case v1alpha1.Terminating:
		return &terminatingState{jobFlow: jobFlow}
	case v1alpha1.Failed:
		return &failedState{jobFlow: jobFlow}
	}

	return nil
}
