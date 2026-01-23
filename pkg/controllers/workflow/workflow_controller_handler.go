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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	workflowv1alpha1 "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"github.com/workflow.sh/work-flow/pkg/apis/helpers"
	"github.com/workflow.sh/work-flow/pkg/controllers/apis"
	"github.com/workflow.sh/work-flow/pkg/controllers/workload"
)

func (wc *workflowcontroller) enqueue(req apis.FlowRequest) {
	wc.queue.Add(req)
}

func (wc *workflowcontroller) addWorkflow(obj interface{}) {
	workflow, ok := obj.(*workflowv1alpha1.Workflow)
	if !ok {
		klog.Errorf("failed to convert object %+v to Workflow", obj)
		return
	}

	req := apis.FlowRequest{
		Namespace:    workflow.Namespace,
		WorkflowName: workflow.Name,
		Action:       workflowv1alpha1.SyncWorkflowAction,
		Event:        workflowv1alpha1.OutOfSyncEvent,
	}

	wc.enqueueWorkflow(req)
}

func (wc *workflowcontroller) updateWorkflow(oldObj, newObj interface{}) {
	oldWf, ok := oldObj.(*workflowv1alpha1.Workflow)
	if !ok {
		return
	}

	newWf, ok := newObj.(*workflowv1alpha1.Workflow)
	if !ok {
		return
	}

	if newWf.ResourceVersion == oldWf.ResourceVersion {
		return
	}

	// Trigger sync if deletion policy is Delete and workflow succeeded (to cleanup)
	// Or if any other change happened that needs reconciliation
	if newWf.Status.State.Phase != workflowv1alpha1.Succeed || newWf.Spec.JobRetainPolicy != workflowv1alpha1.Delete {
		// Even if not success, we might need to sync for other reasons (e.g. spec change)
		// But for now keeping the original logic which seems to prioritize cleanup on success
	}

	req := apis.FlowRequest{
		Namespace:    newWf.Namespace,
		WorkflowName: newWf.Name,
		Action:       workflowv1alpha1.SyncWorkflowAction,
		Event:        workflowv1alpha1.OutOfSyncEvent,
	}

	wc.enqueueWorkflow(req)
}

func (wc *workflowcontroller) updateGenericJob(oldObj, newObj interface{}) {
	oldUnstructured, ok := oldObj.(*unstructured.Unstructured)
	if !ok {
		return
	}

	newUnstructured, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		return
	}

	// Filter out jobs that are not created from workflow
	if !isControlledBy(newUnstructured, helpers.WorkflowKind) {
		return
	}

	if newUnstructured.GetResourceVersion() == oldUnstructured.GetResourceVersion() {
		return
	}

	wl := workload.GetWorkload(newUnstructured.GroupVersionKind().Group)
	if wl != nil {
		oldStatus := wl.GetJobStatus(oldUnstructured)
		newStatus := wl.GetJobStatus(newUnstructured)
		if oldStatus.State == newStatus.State {
			return
		}
	}

	workflowName := getWorkflowNameByObj(newUnstructured)
	if workflowName == "" {
		return
	}

	req := apis.FlowRequest{
		Namespace:    newUnstructured.GetNamespace(),
		WorkflowName: workflowName,
		Action:       workflowv1alpha1.SyncWorkflowAction,
		Event:        workflowv1alpha1.OutOfSyncEvent,
	}

	wc.enqueueWorkflow(req)
}
