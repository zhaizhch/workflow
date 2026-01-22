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

package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	kubeflow "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"github.com/workflow.sh/work-flow/pkg/client/clientset/versioned/scheme"
	"github.com/workflow.sh/work-flow/pkg/controllers/workflow/state"
	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

func (jf *workflowcontroller) syncWorkflow(jobFlow *v1alpha1flow.Workflow, updateStateFn state.UpdateWorkflowStatusFn) error {
	klog.V(4).Infof("Begin to sync Workflow %s.", jobFlow.Name)
	defer klog.V(4).Infof("End sync Workflow %s.", jobFlow.Name)

	// JobRetainPolicy Judging whether jobs are necessary to delete
	if jobFlow.Spec.JobRetainPolicy == v1alpha1flow.Delete && jobFlow.Status.State.Phase == v1alpha1flow.Succeed {
		if err := jf.deleteAllJobsCreatedByWorkflow(jobFlow); err != nil {
			klog.Errorf("Failed to delete jobs of Workflow %v/%v: %v",
				jobFlow.Namespace, jobFlow.Name, err)
			return err
		}
		return nil
	}

	// deploy job by dependence order.
	if err := jf.deployJob(jobFlow); err != nil {
		klog.Errorf("Failed to create jobs of Workflow %v/%v: %v",
			jobFlow.Namespace, jobFlow.Name, err)
		return err
	}

	// update jobFlow status
	jobFlowStatus, err := jf.getAllJobStatus(jobFlow)
	if err != nil {
		return err
	}
	jobFlow.Status = *jobFlowStatus
	updateStateFn(&jobFlow.Status, len(jobFlow.Spec.Flows))
	_, err = jf.flowClient.FlowV1alpha1().Workflows(jobFlow.Namespace).UpdateStatus(context.Background(), jobFlow, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update status of Workflow %v/%v: %v",
			jobFlow.Namespace, jobFlow.Name, err)
		return err
	}

	return nil
}

func (jf *workflowcontroller) deployJob(jobFlow *v1alpha1flow.Workflow) error {

	// load jobTemplate by flow and deploy it
	for _, flow := range jobFlow.Spec.Flows {
		jobName := getJobName(jobFlow.Name, flow.Name)

		jobTemplate, err := jf.jobTemplateLister.WorkTemplates(jobFlow.Namespace).Get(flow.Name)
		if err != nil {
			return err
		}
		gvr := schema.GroupVersionResource{
			Group:    jobTemplate.Spec.GVR.Group,
			Version:  jobTemplate.Spec.GVR.Version,
			Resource: jobTemplate.Spec.GVR.Resource,
		}

		// Check existence generic
		if _, err := jf.dynamicClient.Resource(gvr).Namespace(jobFlow.Namespace).Get(context.Background(), jobName, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				// If it is not distributed, judge whether the dependency of the Workload meets the requirements
				if flow.DependsOn == nil || flow.DependsOn.Targets == nil {
					if err := jf.createJob(jobFlow, flow); err != nil {
						return err
					}
				} else {
					// query whether the dependencies of the job have been met
					flag, err := jf.judge(jobFlow, flow)
					if err != nil {
						return err
					}
					if flag {
						if err := jf.createJob(jobFlow, flow); err != nil {
							return err
						}
					}
				}
				continue
			}
			return err
		}
	}
	return nil
}

// judge query whether the dependencies of the job have been met. If it is satisfied, create the job, if not, judge the next job. Create the job if satisfied
func (jf *workflowcontroller) judge(jobFlow *v1alpha1flow.Workflow, flow v1alpha1flow.Flow) (bool, error) {

	for _, targetName := range flow.DependsOn.Targets {
		targetJobName := getJobName(jobFlow.Name, targetName)

		jobTemplate, err := jf.jobTemplateLister.WorkTemplates(jobFlow.Namespace).Get(targetName)
		if err != nil {
			return false, err
		}
		gvr := schema.GroupVersionResource{
			Group:    jobTemplate.Spec.GVR.Group,
			Version:  jobTemplate.Spec.GVR.Version,
			Resource: jobTemplate.Spec.GVR.Resource,
		}

		// Generic status check
		job, err := jf.dynamicClient.Resource(gvr).Namespace(jobFlow.Namespace).Get(context.Background(), targetJobName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				klog.Infof("No %v Job found！", targetJobName)
				return false, nil
			}
			return false, err
		}

		// 判断workload是否completed
		if gvr.Group == "batch.volcano.sh" { // workload 为vcjob时的判断
			phase, found, _ := unstructured.NestedString(job.Object, "status", "state", "phase")
			if !found {
				if succeeded, found, _ := unstructured.NestedInt64(job.Object, "status", "succeeded"); found && succeeded > 0 {
					phase = string(v1alpha1.Completed)
				}
			}
			if v1alpha1.JobPhase(phase) != v1alpha1.Completed {
				return false, nil
			}
		} else { // workload为kubeflow时的判断
			completed := false
			if conditions, found, _ := unstructured.NestedSlice(job.Object, "status", "conditions"); found {
				for _, c := range conditions {
					condition, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					t, _ := condition["type"].(string)
					s, _ := condition["status"].(string)
					// Kubeflow jobs use "Succeeded"
					if (t == string(v1alpha1flow.Succeed) || t == string(kubeflow.JobSucceeded)) && s == "True" {
						completed = true
						break
					}
				}
			}
			if !completed {
				return false, nil
			}
		}
	}
	return true, nil
}

// createJob
func (jf *workflowcontroller) createJob(jobFlow *v1alpha1flow.Workflow, flow v1alpha1flow.Flow) error {

	jobStr, gvr, err := jf.constructJobFromTemplate(jobFlow, flow.Name, getJobName(jobFlow.Name, flow.Name))
	if err != nil {
		return err
	}

	if _, err := jf.dynamicClient.Resource(*gvr).Namespace(jobFlow.Namespace).Create(context.Background(), jobStr, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	jf.recorder.Eventf(jobFlow, corev1.EventTypeNormal, "Created", fmt.Sprintf("create a job named %v!", jobStr.GetName()))
	return nil
}

// getAllJobStatus Get the information of all created jobs
func (jf *workflowcontroller) getAllJobStatus(workFlow *v1alpha1flow.Workflow) (*v1alpha1flow.WorkflowStatus, error) {
	jobList, err := jf.getAllJobsCreatedByWorkflow(workFlow)
	if err != nil {
		klog.Error(err, "get jobList error")
		return nil, err
	}

	statusListJobMap := map[v1alpha1.JobPhase][]string{
		v1alpha1.Pending:     make([]string, 0),
		v1alpha1.Running:     make([]string, 0),
		v1alpha1.Completing:  make([]string, 0),
		v1alpha1.Completed:   make([]string, 0),
		v1alpha1.Terminating: make([]string, 0),
		v1alpha1.Terminated:  make([]string, 0),
		v1alpha1.Failed:      make([]string, 0),
	}

	UnKnowJobs := make([]string, 0)
	conditions := make(map[string]v1alpha1flow.Condition)
	jobStatusList := make([]v1alpha1flow.JobStatus, 0)
	if workFlow.Status.JobStatusList != nil {
		jobStatusList = workFlow.Status.JobStatusList
	}

	for _, job := range jobList {
		// Determine normalized job phase
		jobPhase := v1alpha1.Pending // default

		// 1. Try Volcano style
		phase, found, _ := unstructured.NestedString(job.Object, "status", "state", "phase")
		if found {
			jobPhase = v1alpha1.JobPhase(phase)
		} else if succeeded, found, _ := unstructured.NestedInt64(job.Object, "status", "succeeded"); found && succeeded > 0 {
			// Volcano fallback
			jobPhase = v1alpha1.Completed
		} else {
			// 2. Try Kubeflow style (conditions)
			if conds, found, _ := unstructured.NestedSlice(job.Object, "status", "conditions"); found && len(conds) > 0 {
				sort.Slice(conds, func(i, j int) bool {
					c1 := conds[i].(map[string]interface{})
					c2 := conds[j].(map[string]interface{})
					t1Str, _ := c1["lastTransitionTime"].(string)
					t2Str, _ := c2["lastTransitionTime"].(string)
					t1, _ := time.Parse(time.RFC3339, t1Str)
					t2, _ := time.Parse(time.RFC3339, t2Str)
					return t1.Before(t2)
				})

				lastCond := conds[len(conds)-1].(map[string]interface{})
				t, _ := lastCond["type"].(string)
				s, _ := lastCond["status"].(string)

				if s == "True" {
					switch kubeflow.JobConditionType(t) {
					case kubeflow.JobSucceeded:
						jobPhase = v1alpha1.Completed
					case kubeflow.JobFailed:
						jobPhase = v1alpha1.Failed
					case kubeflow.JobRunning:
						jobPhase = v1alpha1.Running
					case kubeflow.JobCreated, kubeflow.JobRestarting, kubeflow.JobSuspended:
						jobPhase = v1alpha1.Pending
					}
				}
			}
		}

		// Update maps and lists
		if _, ok := statusListJobMap[jobPhase]; ok {
			statusListJobMap[jobPhase] = append(statusListJobMap[jobPhase], job.GetName())
		} else {
			UnKnowJobs = append(UnKnowJobs, job.GetName())
		}

		creationTimestamp := job.GetCreationTimestamp()
		conditions[job.GetName()] = v1alpha1flow.Condition{
			Phase:           jobPhase,
			CreateTimestamp: creationTimestamp,
		}

		// Update JobStatusList
		newJobStatus := v1alpha1flow.JobStatus{
			Name:           job.GetName(),
			State:          jobPhase,
			StartTimestamp: creationTimestamp,
		}

		foundExisting := false
		for i := range jobStatusList {
			if jobStatusList[i].Name == newJobStatus.Name {
				foundExisting = true
				// Preserve histories
				newJobStatus.RunningHistories = jobStatusList[i].RunningHistories
				jobStatusList[i] = newJobStatus
				break
			}
		}
		if !foundExisting {
			jobStatusList = append(jobStatusList, newJobStatus)
		}
	}

	jobFlowStatus := v1alpha1flow.WorkflowStatus{
		PendingJobs:    statusListJobMap[v1alpha1.Pending],
		RunningJobs:    statusListJobMap[v1alpha1.Running],
		FailedJobs:     statusListJobMap[v1alpha1.Failed],
		CompletedJobs:  statusListJobMap[v1alpha1.Completed],
		TerminatedJobs: statusListJobMap[v1alpha1.Terminated],
		UnKnowJobs:     UnKnowJobs,
		JobStatusList:  jobStatusList,
		Conditions:     conditions,
		State:          workFlow.Status.State,
	}
	return &jobFlowStatus, nil
}

func getRunningHistories(jobStatusList []v1alpha1flow.JobStatus, job *v1alpha1.Job) []v1alpha1flow.JobRunningHistory {
	// This helper is hard to adapt without strong typing on the job.
	// It might be unused if we inline the logic or skip it.
	// Leaving it for now but it's not called by the updated code above.
	return nil
}

func (jf *workflowcontroller) constructJobFromTemplate(jobFlow *v1alpha1flow.Workflow, flowName string, jobName string) (*unstructured.Unstructured, *schema.GroupVersionResource, error) {
	// load jobTemplate
	jobTemplate, err := jf.jobTemplateLister.WorkTemplates(jobFlow.Namespace).Get(flowName)
	if err != nil {
		return nil, nil, err
	}

	gvr := &schema.GroupVersionResource{
		Group:    jobTemplate.Spec.GVR.Group,
		Version:  jobTemplate.Spec.GVR.Version,
		Resource: jobTemplate.Spec.GVR.Resource,
	}

	objMeta := metav1.ObjectMeta{
		Name:      jobName,
		Namespace: jobFlow.Namespace,
		Labels: map[string]string{
			CreatedByWorkTemplate: GenerateObjectString(jobFlow.Namespace, flowName),
			CreatedByWorkflow:     GenerateObjectString(jobFlow.Namespace, jobFlow.Name),
		},
		Annotations: map[string]string{
			CreatedByWorkTemplate: GenerateObjectString(jobFlow.Namespace, flowName),
			CreatedByWorkflow:     GenerateObjectString(jobFlow.Namespace, jobFlow.Name),
		},
	}

	var unstructuredJob *unstructured.Unstructured

	// Generic handling for all resource types
	gvk, err := jf.kubeClient.Discovery().ServerResourcesForGroupVersion(gvr.GroupVersion().String())
	if err != nil {
		return nil, nil, err
	}

	var kind string
	for _, resource := range gvk.APIResources {
		if resource.Name == gvr.Resource {
			kind = resource.Kind
			break
		}
	}
	if kind == "" {
		return nil, nil, fmt.Errorf("failed to find kind for resource %v", gvr.Resource)
	}

	// Prepare the unstructured object structure
	unstructuredContent := map[string]interface{}{
		"apiVersion": gvr.GroupVersion().String(),
		"kind":       kind,
		"metadata":   map[string]interface{}{},
		"spec":       map[string]interface{}{},
	}

	// Unmarshal the raw job spec into the "spec" field
	var specContent map[string]interface{}
	if err := json.Unmarshal(jobTemplate.Spec.JobSpec.Raw, &specContent); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal job spec: %v", err)
	}
	unstructuredContent["spec"] = specContent

	unstructuredJob = &unstructured.Unstructured{Object: unstructuredContent}

	// Set metadata
	unstructuredJob.SetName(objMeta.Name)
	unstructuredJob.SetNamespace(objMeta.Namespace)
	unstructuredJob.SetLabels(labels.Merge(unstructuredJob.GetLabels(), objMeta.Labels))
	unstructuredJob.SetAnnotations(labels.Merge(unstructuredJob.GetAnnotations(), objMeta.Annotations))

	if err := controllerutil.SetControllerReference(jobFlow, unstructuredJob, scheme.Scheme); err != nil {
		return nil, nil, err
	}

	return unstructuredJob, gvr, nil
}

func (jf *workflowcontroller) deleteAllJobsCreatedByWorkflow(jobFlow *v1alpha1flow.Workflow) error {

	for _, flow := range jobFlow.Spec.Flows {
		// load jobTemplate to get GVR
		jobTemplate, err := jf.jobTemplateLister.WorkTemplates(jobFlow.Namespace).Get(flow.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				klog.Warningf("WorkTemplate %s not found for flow in Workflow %s/%s", flow.Name, jobFlow.Namespace, jobFlow.Name)
				continue
			}
			return err
		}

		gvr := schema.GroupVersionResource{
			Group:    jobTemplate.Spec.GVR.Group,
			Version:  jobTemplate.Spec.GVR.Version,
			Resource: jobTemplate.Spec.GVR.Resource,
		}

		jobName := getJobName(jobFlow.Name, flow.Name)
		// Delete the resource dynamically
		err = jf.dynamicClient.Resource(gvr).Namespace(jobFlow.Namespace).Delete(context.Background(), jobName, metav1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			klog.Errorf("Failed to delete resource %s %s/%s of Workflow %v/%v: %v",
				gvr.Resource, jobFlow.Namespace, jobName, jobFlow.Namespace, jobFlow.Name, err)
			return err
		}
	}
	return nil
}

func (jf *workflowcontroller) getAllJobsCreatedByWorkflow(jobFlow *v1alpha1flow.Workflow) ([]unstructured.Unstructured, error) {
	// 1. Collect unique GVRs
	gvrs := make(map[schema.GroupVersionResource]struct{})
	for _, flow := range jobFlow.Spec.Flows {
		jobTemplate, err := jf.jobTemplateLister.WorkTemplates(jobFlow.Namespace).Get(flow.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		gvr := schema.GroupVersionResource{
			Group:    jobTemplate.Spec.GVR.Group,
			Version:  jobTemplate.Spec.GVR.Version,
			Resource: jobTemplate.Spec.GVR.Resource,
		}
		gvrs[gvr] = struct{}{}
	}

	var allJobs []unstructured.Unstructured
	// 2. Query resources for each GVR
	for gvr := range gvrs {
		list, err := jf.getAllResourcesCreatedByWorkflow(context.Background(), jobFlow, gvr)
		if err != nil {
			klog.Errorf("Failed to list resources for GVR %v: %v", gvr, err)
			return nil, err
		}
		allJobs = append(allJobs, list...)
	}

	return allJobs, nil
}

// 可以查询任意CRD
func (jf *workflowcontroller) getAllCRDsByLabel(
	ctx context.Context,
	gvr schema.GroupVersionResource, // 例如: schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	namespace string,
	labelSelector map[string]string,
) ([]unstructured.Unstructured, error) {

	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector).String(),
	}

	var resultList []unstructured.Unstructured

	if namespace == "" {
		// 集群级别资源
		list, err := jf.dynamicClient.Resource(gvr).List(ctx, listOptions)
		if err != nil {
			return nil, err
		}
		resultList = list.Items
	} else {
		// 命名空间级别资源
		list, err := jf.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, listOptions)
		if err != nil {
			return nil, err
		}
		resultList = list.Items
	}

	return resultList, nil
}

func (jf *workflowcontroller) getAllResourcesCreatedByWorkflow(
	ctx context.Context,
	jobFlow *v1alpha1flow.Workflow,
	gvr schema.GroupVersionResource,
) ([]unstructured.Unstructured, error) {
	labelSelector := map[string]string{
		CreatedByWorkflow: GenerateObjectString(jobFlow.Namespace, jobFlow.Name),
	}
	return jf.getAllCRDsByLabel(ctx, gvr, jobFlow.Namespace, labelSelector)
}
