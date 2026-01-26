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
	"strings"

	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

type runningState struct {
	jobFlow *v1alpha1.Workflow
}

func (p *runningState) Execute(action v1alpha1.Action) error {
	switch action {
	case v1alpha1.SyncWorkflowAction:
		return SyncWorkflow(p.jobFlow, func(status *v1alpha1.WorkflowStatus, allJobList int) {
			if len(status.CompletedJobs) == allJobList {
				UpdateWorkflowSucceed(p.jobFlow.Namespace)
				status.State.Phase = v1alpha1.Succeed
			} else if len(status.FailedJobs) > 0 {
				// Only transition to Failed if all failed jobs have exhausted their retries
				allExhausted := true
				for _, failedJobName := range status.FailedJobs {
					canRetry := false
					for _, flow := range p.jobFlow.Spec.Flows {
						// Match job name to flow. We use a simple suffix/contains check
						// since job name is {wf}-{flow} or {wf}-{flow}-{index}
						if strings.Contains(failedJobName, flow.Name) {
							if flow.Retry == nil {
								break // No retry configured, this job is permanently failed
							}

							// Find its current restart count
							for _, js := range status.JobStatusList {
								if js.Name == failedJobName {
									if js.RestartCount < flow.Retry.MaxRetries {
										canRetry = true
									}
									break
								}
							}
							break
						}
					}
					if canRetry {
						allExhausted = false
						break
					}
				}

				if allExhausted {
					status.State.Phase = v1alpha1.Failed
				}
			}
		})
	}
	return nil
}
