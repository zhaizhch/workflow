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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"github.com/workflow.sh/work-flow/pkg/controllers/workflow/state"
	"github.com/workflow.sh/work-flow/pkg/controllers/workload"
	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

func (wc *workflowcontroller) syncWorkflow(workflow *v1alpha1flow.Workflow, updateStateFn state.UpdateWorkflowStatusFn) error {
	klog.V(4).Infof("Begin to sync Workflow %s.", workflow.Name)
	defer klog.V(4).Infof("End sync Workflow %s.", workflow.Name)

	// JobRetainPolicy Judging whether jobs are necessary to delete
	shouldDelete := false
	switch workflow.Spec.JobRetainPolicy {
	case v1alpha1flow.Delete:
		phase := workflow.Status.State.Phase
		shouldDelete = phase == v1alpha1flow.Succeed || phase == v1alpha1flow.Failed || phase == v1alpha1flow.Terminating
	case v1alpha1flow.DeleteOnSuccess:
		shouldDelete = workflow.Status.State.Phase == v1alpha1flow.Succeed
	}

	if shouldDelete {
		if err := wc.deleteAllJobsCreatedByWorkflow(workflow); err != nil {
			klog.Errorf("Failed to delete jobs of Workflow %v/%v: %v", workflow.Namespace, workflow.Name, err)
			return err
		}
		return nil
	}

	// update workflow status first to ensure deployJob uses latest data
	workflowStatus, err := wc.getAllJobStatus(workflow)
	if err != nil {
		return err
	}
	workflow.Status = *workflowStatus
	totalJobs := wc.getTotalJobCount(workflow)
	updateStateFn(&workflow.Status, totalJobs)
	_, err = wc.flowClient.FlowV1alpha1().Workflows(workflow.Namespace).UpdateStatus(context.Background(), workflow, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update status of Workflow %v/%v: %v", workflow.Namespace, workflow.Name, err)
		return err
	}

	// deploy job by dependence order.
	if err := wc.deployJob(workflow); err != nil {
		klog.Errorf("Failed to create jobs of Workflow %v/%v: %v", workflow.Namespace, workflow.Name, err)
		return err
	}

	return nil
}

func (wc *workflowcontroller) deployJob(workflow *v1alpha1flow.Workflow) error {
	for _, flow := range workflow.Spec.Flows {
		_, err := wc.forEachReplica(workflow, flow.Name, v1alpha1flow.All, func(jobName string, index int) (bool, error) {
			return wc.processJobReplica(workflow, flow, jobName, index)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (wc *workflowcontroller) processJobReplica(workflow *v1alpha1flow.Workflow, flow v1alpha1flow.Flow, jobName string, index int) (bool, error) {
	if wc.isJobTerminalSuccess(workflow, jobName) {
		return true, nil
	}

	jobTemplate, err := wc.workTemplateInformer.Lister().WorkTemplates(workflow.Namespace).Get(flow.Name)
	if err != nil {
		return false, err
	}

	gvr := schema.GroupVersionResource{
		Group:    jobTemplate.Spec.GVR.Group,
		Version:  jobTemplate.Spec.GVR.Version,
		Resource: jobTemplate.Spec.GVR.Resource,
	}

	existingJob, err := wc.dynamicClient.Resource(gvr).Namespace(workflow.Namespace).Get(context.Background(), jobName, metav1.GetOptions{})

	if err != nil {
		if !errors.IsNotFound(err) {
			return false, err
		}

		satisfied, err := wc.checkDependencies(workflow, flow, index)
		if err != nil {
			return false, err
		}

		if satisfied {
			if err := wc.createJob(workflow, flow, index); err != nil {
				return false, err
			}
		}
		return true, nil
	}

	wc.handleRetry(workflow, flow, existingJob, gvr, jobName)
	return true, nil
}

func (wc *workflowcontroller) checkDependencies(workflow *v1alpha1flow.Workflow, flow v1alpha1flow.Flow, index int) (bool, error) {
	// Global dependencies
	if flow.DependsOn != nil && (len(flow.DependsOn.Targets) > 0 || flow.DependsOn.Probe != nil || len(flow.DependsOn.OrGroups) > 0) {
		satisfied, err := wc.judge(workflow, flow.DependsOn, flow.Name, -1)
		if err != nil || !satisfied {
			return false, err
		}
	}

	// Replica-level dependencies
	if flow.For != nil && flow.For.DependsOn != nil {
		satisfied, err := wc.judge(workflow, flow.For.DependsOn, flow.Name, index)
		if err != nil || !satisfied {
			return false, err
		}
	}

	return true, nil
}

func (wc *workflowcontroller) handleRetry(workflow *v1alpha1flow.Workflow, flow v1alpha1flow.Flow, job interface{}, gvr schema.GroupVersionResource, jobName string) {
	if flow.Retry == nil {
		return
	}

	wl := workload.GetWorkload(gvr.Group)
	if wl == nil {
		return
	}

	status := wl.GetJobStatus(job.(*unstructured.Unstructured))
	if status.State != v1alpha1.Failed {
		return
	}

	var restartCount int32 = 0
	for _, js := range workflow.Status.JobStatusList {
		if js.Name == jobName {
			restartCount = js.RestartCount
			break
		}
	}

	if restartCount < flow.Retry.MaxRetries {
		klog.Infof("Retrying job %s, current retry: %d", jobName, restartCount)
		if err := wc.dynamicClient.Resource(gvr).Namespace(workflow.Namespace).Delete(context.Background(), jobName, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("Failed to delete failed job %s for retry: %v", jobName, err)
		}
	}
}
