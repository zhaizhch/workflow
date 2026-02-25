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
		return SyncWorkflow(p.jobFlow, func(status *workflowv1alpha1.WorkflowStatus, _ int) {
			newPhase := determinePhaseFromStatus(status)
			if newPhase == workflowv1alpha1.Failed {
				// 转入失败态前更新 Prometheus 指标
				UpdateWorkflowFailed(p.jobFlow.Namespace)
			}
			status.State.Phase = newPhase
		})
	}
	return nil
}

// determinePhaseFromStatus 根据当前工作流状态内各类 Job 数量判断应转入的阶段。
//
// 判断规则（按优先级高源低）：
//  1. 有 Running 或 Completed 且无 Failed → Running
//  2. 有 Failed 或 Terminated           → Failed
//  3. 其他情况                      → Pending
func determinePhaseFromStatus(status *workflowv1alpha1.WorkflowStatus) workflowv1alpha1.Phase {
	switch {
	case (len(status.RunningJobs) > 0 || len(status.CompletedJobs) > 0) && len(status.FailedJobs) == 0:
		return workflowv1alpha1.Running
	case len(status.FailedJobs) > 0 || len(status.TerminatedJobs) > 0:
		return workflowv1alpha1.Failed
	default:
		return workflowv1alpha1.Pending
	}
}
