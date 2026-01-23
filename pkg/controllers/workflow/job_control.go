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
	"encoding/json"
	"fmt"

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
)

func (wc *workflowcontroller) createJob(workflow *v1alpha1flow.Workflow, flow v1alpha1flow.Flow, index int) error {
	job, gvr, err := wc.constructJobFromTemplate(workflow, flow, getJobName(workflow.Name, flow.Name, index))
	if err != nil {
		klog.Errorf("failed to construct job for flow %s in workflow %s: %v", flow.Name, workflow.Name, err)
		return err
	}

	if _, err := wc.dynamicClient.Resource(*gvr).Namespace(workflow.Namespace).Create(context.Background(), job, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	wc.recorder.Eventf(workflow, corev1.EventTypeNormal, "Created", "Successfully created job %s", job.GetName())
	return nil
}

func (wc *workflowcontroller) constructJobFromTemplate(workflow *v1alpha1flow.Workflow, flow v1alpha1flow.Flow, jobName string) (*unstructured.Unstructured, *schema.GroupVersionResource, error) {
	jobTemplate, err := wc.workTemplateLister.WorkTemplates(workflow.Namespace).Get(flow.Name)
	if err != nil {
		return nil, nil, err
	}

	gvr := &schema.GroupVersionResource{
		Group:    jobTemplate.Spec.GVR.Group,
		Version:  jobTemplate.Spec.GVR.Version,
		Resource: jobTemplate.Spec.GVR.Resource,
	}

	// Prepare the unstructured object structure
	unstructuredContent := map[string]interface{}{
		"apiVersion": gvr.GroupVersion().String(),
		"kind":       "", // Will be set below
		"metadata": map[string]interface{}{
			"name":      jobName,
			"namespace": workflow.Namespace,
			"labels": map[string]string{
				CreatedByWorkTemplate: GenerateObjectString(workflow.Namespace, flow.Name),
				CreatedByWorkflow:     GenerateObjectString(workflow.Namespace, workflow.Name),
			},
			"annotations": map[string]string{
				CreatedByWorkTemplate: GenerateObjectString(workflow.Namespace, flow.Name),
				CreatedByWorkflow:     GenerateObjectString(workflow.Namespace, workflow.Name),
			},
		},
	}

	// Discovery to find Kind
	resources, err := wc.kubeClient.Discovery().ServerResourcesForGroupVersion(gvr.GroupVersion().String())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find server resources for %v: %v", gvr.GroupVersion(), err)
	}

	for _, resource := range resources.APIResources {
		if resource.Name == gvr.Resource {
			unstructuredContent["kind"] = resource.Kind
			break
		}
	}
	if unstructuredContent["kind"] == "" {
		return nil, nil, fmt.Errorf("failed to find kind for resource %s", gvr.Resource)
	}

	// Unmarshal JobSpec
	var specContent map[string]interface{}
	if err := json.Unmarshal(jobTemplate.Spec.JobSpec.Raw, &specContent); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal job spec: %v", err)
	}
	unstructuredContent["spec"] = specContent

	job := &unstructured.Unstructured{Object: unstructuredContent}

	// Apply Patches
	if flow.Patch != nil && len(flow.Patch.Raw) > 0 {
		var patchMap map[string]interface{}
		if err := json.Unmarshal(flow.Patch.Raw, &patchMap); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal patch: %v", err)
		}
		wc.applyPatch(job, patchMap)
	}

	// Inject Global Settings
	if workflow.Spec.SchedulerName != "" {
		unstructured.SetNestedField(job.Object, workflow.Spec.SchedulerName, "spec", "schedulerName")
	}
	if len(workflow.Spec.Plugins) > 0 {
		// Convert plugins map[string][]string to map[string]interface{} for unstructured.
		// We must convert []string to []interface{} because DeepCopyJSONValue does not support []string.
		pluginsMap := make(map[string]interface{})
		for k, v := range workflow.Spec.Plugins {
			var interfaceSlice []interface{}
			for _, s := range v {
				interfaceSlice = append(interfaceSlice, s)
			}
			pluginsMap[k] = interfaceSlice
		}
		unstructured.SetNestedMap(job.Object, pluginsMap, "spec", "plugins")
	}

	if err := controllerutil.SetControllerReference(workflow, job, scheme.Scheme); err != nil {
		return nil, nil, fmt.Errorf("failed to set controller reference: %v", err)
	}

	return job, gvr, nil
}

func (wc *workflowcontroller) applyPatch(job *unstructured.Unstructured, patchMap map[string]interface{}) {
	for k, v := range patchMap {
		if v == nil {
			continue
		}
		switch k {
		case "metadata":
			wc.mergeMetadata(job, v)
		case "spec":
			wc.mergeSpec(job, v)
		default:
			unstructured.SetNestedField(job.Object, v, "spec", k)
		}
	}
}

func (wc *workflowcontroller) mergeMetadata(job *unstructured.Unstructured, v interface{}) {
	patchMeta, ok := v.(map[string]interface{})
	if !ok {
		return
	}
	if labelsMap, ok := patchMeta["labels"].(map[string]interface{}); ok {
		existing, _, _ := unstructured.NestedStringMap(job.Object, "metadata", "labels")
		if existing == nil {
			existing = make(map[string]string)
		}
		for lk, lv := range labelsMap {
			if s, ok := lv.(string); ok {
				existing[lk] = s
			}
		}
		unstructured.SetNestedStringMap(job.Object, existing, "metadata", "labels")
	}
	if annosMap, ok := patchMeta["annotations"].(map[string]interface{}); ok {
		existing, _, _ := unstructured.NestedStringMap(job.Object, "metadata", "annotations")
		if existing == nil {
			existing = make(map[string]string)
		}
		for ak, av := range annosMap {
			if s, ok := av.(string); ok {
				existing[ak] = s
			}
		}
		unstructured.SetNestedStringMap(job.Object, existing, "metadata", "annotations")
	}
}

func (wc *workflowcontroller) mergeSpec(job *unstructured.Unstructured, v interface{}) {
	specMap, ok := v.(map[string]interface{})
	if !ok {
		return
	}
	for sk, sv := range specMap {
		unstructured.SetNestedField(job.Object, sv, "spec", sk)
	}
}

func (wc *workflowcontroller) deleteAllJobsCreatedByWorkflow(workflow *v1alpha1flow.Workflow) error {
	for _, flow := range workflow.Spec.Flows {
		jobTemplate, err := wc.workTemplateLister.WorkTemplates(workflow.Namespace).Get(flow.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return err
		}

		gvr := schema.GroupVersionResource{
			Group:    jobTemplate.Spec.GVR.Group,
			Version:  jobTemplate.Spec.GVR.Version,
			Resource: jobTemplate.Spec.GVR.Resource,
		}

		replicas := 1
		if flow.For != nil && flow.For.Replicas != nil {
			replicas = int(*flow.For.Replicas)
		}

		for i := 0; i < replicas; i++ {
			index := -1
			if flow.For != nil {
				index = i
			}
			jobName := getJobName(workflow.Name, flow.Name, index)
			if err := wc.dynamicClient.Resource(gvr).Namespace(workflow.Namespace).Delete(context.Background(), jobName, metav1.DeleteOptions{}); err != nil {
				if errors.IsNotFound(err) {
					continue
				}
				klog.Errorf("Failed to delete resource %s %s/%s for workflow %s: %v", gvr.Resource, workflow.Namespace, jobName, workflow.Name, err)
				return err
			}
		}
	}
	return nil
}

func (wc *workflowcontroller) getAllJobsCreatedByWorkflow(workflow *v1alpha1flow.Workflow) ([]unstructured.Unstructured, error) {
	gvrs := make(map[schema.GroupVersionResource]struct{})
	for _, flow := range workflow.Spec.Flows {
		jobTemplate, err := wc.workTemplateLister.WorkTemplates(workflow.Namespace).Get(flow.Name)
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
	for gvr := range gvrs {
		list, err := wc.getAllResourcesCreatedByWorkflow(context.Background(), workflow, gvr)
		if err != nil {
			return nil, err
		}
		allJobs = append(allJobs, list...)
	}

	return allJobs, nil
}

func (wc *workflowcontroller) getAllResourcesCreatedByWorkflow(ctx context.Context, workflow *v1alpha1flow.Workflow, gvr schema.GroupVersionResource) ([]unstructured.Unstructured, error) {
	labelSelector := map[string]string{
		CreatedByWorkflow: GenerateObjectString(workflow.Namespace, workflow.Name),
	}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector).String(),
	}

	list, err := wc.dynamicClient.Resource(gvr).Namespace(workflow.Namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}
