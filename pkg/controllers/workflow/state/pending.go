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
	workflowv1alpha1 "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

type pendingState struct {
	jobFlow *workflowv1alpha1.Workflow
}

func (p *pendingState) Execute(action workflowv1alpha1.Action) error {
	switch action {
	case workflowv1alpha1.SyncWorkflowAction:
		return SyncWorkflow(p.jobFlow, func(status *workflowv1alpha1.WorkflowStatus, allJobList int) {
			if (len(status.RunningJobs) > 0 || len(status.CompletedJobs) > 0) && len(status.FailedJobs) <= 0 {
				status.State.Phase = workflowv1alpha1.Running
			} else if len(status.FailedJobs) > 0 || len(status.TerminatedJobs) > 0 { // TODO(dongjiang1989) Modify it when the if condition judgment is implemented
				UpdateWorkflowFailed(p.jobFlow.Namespace)
				status.State.Phase = workflowv1alpha1.Failed
			} else {
				status.State.Phase = workflowv1alpha1.Pending
			}
		})
	}
	return nil
}
