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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	workflowv1alpha1 "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"github.com/workflow.sh/work-flow/pkg/apis/helpers"
	"github.com/workflow.sh/work-flow/pkg/controllers/apis"
)

func (jf *workflowcontroller) enqueue(req apis.FlowRequest) {
	jf.queue.Add(req)
}

func (jf *workflowcontroller) addWorkflow(obj interface{}) {
	jobFlow, ok := obj.(*workflowv1alpha1.Workflow)
	if !ok {
		klog.Errorf("Failed to convert %v to jobFlow", obj)
		return
	}

	// use struct instead of pointer
	req := apis.FlowRequest{
		Namespace:    jobFlow.Namespace,
		WorkflowName: jobFlow.Name,

		Action: workflowv1alpha1.SyncWorkflowAction,
		Event:  workflowv1alpha1.OutOfSyncEvent,
	}

	jf.enqueueWorkflow(req)
}

func (jf *workflowcontroller) updateWorkflow(oldObj, newObj interface{}) {
	oldWorkflow, ok := oldObj.(*workflowv1alpha1.Workflow)
	if !ok {
		klog.Errorf("Failed to convert %v to workflow", oldWorkflow)
		return
	}

	newWorkflow, ok := newObj.(*workflowv1alpha1.Workflow)
	if !ok {
		klog.Errorf("Failed to convert %v to workflow", newWorkflow)
		return
	}

	if newWorkflow.ResourceVersion == oldWorkflow.ResourceVersion {
		return
	}

	//Todo The update operation of Workflow is reserved for possible future use. The current update operation on Workflow will not affect the Workflow process
	if newWorkflow.Status.State.Phase != workflowv1alpha1.Succeed || newWorkflow.Spec.JobRetainPolicy != workflowv1alpha1.Delete {
		return
	}

	req := apis.FlowRequest{
		Namespace:    newWorkflow.Namespace,
		WorkflowName: newWorkflow.Name,

		Action: workflowv1alpha1.SyncWorkflowAction,
		Event:  workflowv1alpha1.OutOfSyncEvent,
	}

	jf.enqueueWorkflow(req)
}

func (jf *workflowcontroller) updateGenericJob(oldObj, newObj interface{}) {
	oldUnstructured, ok := oldObj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("Failed to convert %v to unstructured", oldObj)
		return
	}

	newUnstructured, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("Failed to convert %v to unstructured", newObj)
		return
	}

	// Filter out jobs that are not created from volcano workflow
	if !isControlledBy(newUnstructured, helpers.WorkflowKind) {
		return
	}

	if newUnstructured.GetResourceVersion() == oldUnstructured.GetResourceVersion() {
		return
	}

	jobFlowName := getWorkflowNameByObj(newUnstructured)
	if jobFlowName == "" {
		return
	}

	req := apis.FlowRequest{
		Namespace:    newUnstructured.GetNamespace(),
		WorkflowName: jobFlowName,
		Action:       workflowv1alpha1.SyncWorkflowAction,
		Event:        workflowv1alpha1.OutOfSyncEvent,
	}

	jf.enqueueWorkflow(req)
}
