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
	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

type runningState struct {
	jobFlow *v1alpha1.Workflow
}

func (p *runningState) Execute(action v1alpha1.Action) error {
	switch action {
	case v1alpha1.SyncWorkflowAction:
		return SyncWorkflow(p.jobFlow, func(status *v1alpha1.WorkflowStatus, allJobList int) {
			// Check workflow success based on SuccessPolicy
			if isWorkflowSuccessful(p.jobFlow, status, allJobList) {
				UpdateWorkflowSucceed(p.jobFlow.Namespace)
				status.State.Phase = v1alpha1.Succeed
			} else {
				// Check workflow failure based on SuccessPolicy and ContinueOnFail
				if isWorkflowFailed(p.jobFlow, status) {
					status.State.Phase = v1alpha1.Failed
				}
			}
		})
	}
	return nil
}
