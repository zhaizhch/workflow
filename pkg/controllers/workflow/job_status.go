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
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"github.com/workflow.sh/work-flow/pkg/controllers/workload"
	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

func (wc *workflowcontroller) getAllJobStatus(workflow *v1alpha1flow.Workflow) (*v1alpha1flow.WorkflowStatus, error) {
	jobList, err := wc.getAllJobsCreatedByWorkflow(workflow)
	if err != nil {
		klog.Errorf("failed to get jobList for workflow %s: %v", workflow.Name, err)
		return nil, err
	}

	statusListJobMap := map[v1alpha1.JobPhase][]string{
		v1alpha1.Pending:     {},
		v1alpha1.Running:     {},
		v1alpha1.Completing:  {},
		v1alpha1.Completed:   {},
		v1alpha1.Terminating: {},
		v1alpha1.Terminated:  {},
		v1alpha1.Failed:      {},
	}

	unknownJobs := make([]string, 0)
	conditions := make(map[string]v1alpha1flow.Condition)
	jobStatusList := make([]v1alpha1flow.JobStatus, 0)
	if workflow.Status.JobStatusList != nil {
		jobStatusList = workflow.Status.JobStatusList
	}

	for _, job := range jobList {
		wl := workload.GetWorkload(job.GroupVersionKind().Group)
		if wl == nil {
			klog.Warningf("No workload handler found for group %s, skipping job %s", job.GroupVersionKind().Group, job.GetName())
			continue
		}
		jobStatus := wl.GetJobStatus(&job)
		jobPhase := jobStatus.State

		if _, ok := statusListJobMap[jobPhase]; ok {
			statusListJobMap[jobPhase] = append(statusListJobMap[jobPhase], job.GetName())
		} else {
			unknownJobs = append(unknownJobs, job.GetName())
		}

		creationTimestamp := job.GetCreationTimestamp()
		conditions[job.GetName()] = v1alpha1flow.Condition{
			Phase:           jobPhase,
			CreateTimestamp: creationTimestamp,
		}

		newJobStatus := v1alpha1flow.JobStatus{
			Name:           job.GetName(),
			State:          jobPhase,
			StartTimestamp: creationTimestamp,
		}

		foundExisting := false
		for i := range jobStatusList {
			if jobStatusList[i].Name == newJobStatus.Name {
				foundExisting = true
				newJobStatus.RestartCount = jobStatusList[i].RestartCount
				if jobPhase == v1alpha1.Running && jobStatusList[i].State == v1alpha1.Failed {
					newJobStatus.RestartCount++
				}
				newJobStatus.RunningHistories = jobStatusList[i].RunningHistories
				jobStatusList[i] = newJobStatus
				break
			}
		}
		if !foundExisting {
			jobStatusList = append(jobStatusList, newJobStatus)
		}
	}

	workflowStatus := v1alpha1flow.WorkflowStatus{
		PendingJobs:    statusListJobMap[v1alpha1.Pending],
		RunningJobs:    statusListJobMap[v1alpha1.Running],
		FailedJobs:     statusListJobMap[v1alpha1.Failed],
		CompletedJobs:  statusListJobMap[v1alpha1.Completed],
		TerminatedJobs: statusListJobMap[v1alpha1.Terminated],
		UnKnowJobs:     unknownJobs,
		JobStatusList:  jobStatusList,
		Conditions:     conditions,
		State:          workflow.Status.State,
	}

	return &workflowStatus, nil
}

func (wc *workflowcontroller) getJobStatusFromCluster(workflow *v1alpha1flow.Workflow, taskName, jobName string) (v1alpha1.JobPhase, error) {
	for _, js := range workflow.Status.JobStatusList {
		if js.Name == jobName && js.State == v1alpha1.JobPhase(v1alpha1flow.JobSkipped) {
			return js.State, nil
		}
	}

	jobTemplate, err := wc.workTemplateLister.WorkTemplates(workflow.Namespace).Get(taskName)
	if err != nil {
		return "", err
	}
	gvr := schema.GroupVersionResource{
		Group:    jobTemplate.Spec.GVR.Group,
		Version:  jobTemplate.Spec.GVR.Version,
		Resource: jobTemplate.Spec.GVR.Resource,
	}

	job, err := wc.dynamicClient.Resource(gvr).Namespace(workflow.Namespace).Get(context.Background(), jobName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	wl := workload.GetWorkload(gvr.Group)
	if wl == nil {
		return "", fmt.Errorf("no workload handler for group %s", gvr.Group)
	}
	return wl.GetJobStatus(job).State, nil
}

func (wc *workflowcontroller) isJobTerminalSuccess(workflow *v1alpha1flow.Workflow, jobName string) bool {
	for _, js := range workflow.Status.JobStatusList {
		if js.Name == jobName {
			return js.State == v1alpha1.Completed || js.State == v1alpha1flow.JobSkipped
		}
	}
	return false
}

func (wc *workflowcontroller) getTotalJobCount(workflow *v1alpha1flow.Workflow) int {
	total := 0
	for _, flow := range workflow.Spec.Flows {
		if flow.For != nil && flow.For.Replicas != nil {
			total += int(*flow.For.Replicas)
		} else {
			total++
		}
	}
	return total
}

func (wc *workflowcontroller) getJobIP(workflow *v1alpha1flow.Workflow, taskName, jobName string) (string, error) {
	jobTemplate, err := wc.workTemplateLister.WorkTemplates(workflow.Namespace).Get(taskName)
	if err != nil {
		return "", err
	}
	gvr := schema.GroupVersionResource{
		Group:    jobTemplate.Spec.GVR.Group,
		Version:  jobTemplate.Spec.GVR.Version,
		Resource: jobTemplate.Spec.GVR.Resource,
	}

	job, err := wc.dynamicClient.Resource(gvr).Namespace(workflow.Namespace).Get(context.Background(), jobName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	wl := workload.GetWorkload(gvr.Group)
	if wl == nil {
		return "", fmt.Errorf("no workload handler for group %s", gvr.Group)
	}

	ls := wl.GetPodLabels(job)
	if len(ls) == 0 {
		return "", fmt.Errorf("no pod labels found for job %s", jobName)
	}

	podList, err := wc.kubeClient.CoreV1().Pods(workflow.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(ls).String(),
	})
	if err != nil {
		return "", err
	}

	if len(podList.Items) == 0 {
		return "", fmt.Errorf("no pods found for job %s", jobName)
	}

	// Priority 1: Running pod with IP that is a "Master"
	for _, pod := range podList.Items {
		if pod.Status.Phase == v1.PodRunning && pod.Status.PodIP != "" {
			if pod.Labels["training.kubeflow.org/replica-type"] == "Master" || pod.Labels["workflow.sh/task-name"] == "master" {
				return pod.Status.PodIP, nil
			}
		}
	}

	// Priority 2: Any running pod with IP
	for _, pod := range podList.Items {
		if pod.Status.Phase == v1.PodRunning && pod.Status.PodIP != "" {
			return pod.Status.PodIP, nil
		}
	}

	return "", fmt.Errorf("no running pods with IP found for job %s", jobName)
}
