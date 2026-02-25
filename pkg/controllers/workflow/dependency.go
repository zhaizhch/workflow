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

package workflow

import (
	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"github.com/workflow.sh/work-flow/pkg/controllers/workflow/engine"
	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

func (wc *workflowcontroller) GetJobName(workflowName, taskName string, index int) string {
	return getJobName(workflowName, taskName, index)
}

func (wc *workflowcontroller) IsJobTerminalSuccess(workflow *v1alpha1flow.Workflow, jobName string) bool {
	return wc.isJobTerminalSuccess(workflow, jobName)
}

func (wc *workflowcontroller) GetJobStatusFromCluster(workflow *v1alpha1flow.Workflow, targetName, jobName string) (v1alpha1.JobPhase, error) {
	return wc.getJobStatusFromCluster(workflow, targetName, jobName)
}

func (wc *workflowcontroller) IsContinueOnFail(workflow *v1alpha1flow.Workflow, targetName string) bool {
	return wc.isContinueOnFail(workflow, targetName)
}

func (wc *workflowcontroller) GetJobIP(workflow *v1alpha1flow.Workflow, targetName, jobName string) (string, error) {
	return wc.getJobIP(workflow, targetName, jobName)
}

func (wc *workflowcontroller) judge(workflow *v1alpha1flow.Workflow, dependsOn *v1alpha1flow.DependsOn, currentTaskName string, currentIndex int) (bool, error) {
	dagEngine := engine.NewDagEngine(wc)
	return dagEngine.Judge(workflow, dependsOn, currentTaskName, currentIndex)
}

func (wc *workflowcontroller) forEachReplica(workflow *v1alpha1flow.Workflow, taskName string, strategy v1alpha1flow.DependencyStrategy, fn func(jobName string, index int) (bool, error)) (bool, error) {
	dagEngine := engine.NewDagEngine(wc)
	return dagEngine.ForEachReplica(workflow, taskName, strategy, fn)
}
