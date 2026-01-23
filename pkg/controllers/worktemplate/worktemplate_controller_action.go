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

package worktemplate

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

func (wtc *worktemplatecontroller) syncWorkTemplate(workTemplate *v1alpha1flow.WorkTemplate) error {
	// search the jobs created by WorkTemplate
	labelSelector := map[string]string{
		CreatedByWorkTemplate: GetTemplateString(workTemplate.Namespace, workTemplate.Name),
	}

	gvr := schema.GroupVersionResource{
		Group:    workTemplate.Spec.GVR.Group,
		Version:  workTemplate.Spec.GVR.Version,
		Resource: workTemplate.Spec.GVR.Resource,
	}

	jobList, err := wtc.getAllCRDsByLabel(context.Background(), gvr, workTemplate.Namespace, labelSelector)
	if err != nil {
		klog.Errorf("failed to list jobs of WorkTemplate %s/%s: %v",
			workTemplate.Namespace, workTemplate.Name, err)
		return err
	}

	if len(jobList) == 0 {
		return nil
	}

	jobNames := make([]string, 0, len(jobList))
	for _, job := range jobList {
		jobNames = append(jobNames, job.GetName())
	}
	workTemplate.Status.JobDependsOnList = jobNames

	// update WorkTemplate status
	_, err = wtc.flowClient.FlowV1alpha1().WorkTemplates(workTemplate.Namespace).UpdateStatus(context.Background(), workTemplate, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("failed to update status of WorkTemplate %s/%s: %v",
			workTemplate.Namespace, workTemplate.Name, err)
		return err
	}
	return nil
}

// getAllCRDsByLabel lists resources using dynamic client
func (wtc *worktemplatecontroller) getAllCRDsByLabel(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace string,
	labelSelector map[string]string,
) ([]unstructured.Unstructured, error) {

	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector).String(),
	}

	var resultList []unstructured.Unstructured

	if namespace == "" {
		// Cluster-scoped resources
		list, err := wtc.dynamicClient.Resource(gvr).List(ctx, listOptions)
		if err != nil {
			return nil, err
		}
		resultList = list.Items
	} else {
		// Namespaced resources
		list, err := wtc.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, listOptions)
		if err != nil {
			return nil, err
		}
		resultList = list.Items
	}

	return resultList, nil
}
