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

func (jt *worktemplatecontroller) syncWorkTemplate(jobTemplate *v1alpha1flow.WorkTemplate) error {
	// search the jobs created by WorkTemplate
	labelSelector := map[string]string{
		CreatedByWorkTemplate: GetTemplateString(jobTemplate.Namespace, jobTemplate.Name),
	}

	gvr := schema.GroupVersionResource{
		Group:    jobTemplate.Spec.GVR.Group,
		Version:  jobTemplate.Spec.GVR.Version,
		Resource: jobTemplate.Spec.GVR.Resource,
	}

	jobList, err := jt.getAllCRDsByLabel(context.Background(), gvr, jobTemplate.Namespace, labelSelector)
	if err != nil {
		klog.Errorf("Failed to list jobs of WorkTemplate %v/%v: %v",
			jobTemplate.Namespace, jobTemplate.Name, err)
		return err
	}

	if len(jobList) == 0 {
		return nil
	}

	jobListName := make([]string, 0)
	for _, job := range jobList {
		jobListName = append(jobListName, job.GetName())
	}
	jobTemplate.Status.JobDependsOnList = jobListName

	//update jobTemplate status
	_, err = jt.flowClient.FlowV1alpha1().WorkTemplates(jobTemplate.Namespace).UpdateStatus(context.Background(), jobTemplate, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update status of WorkTemplate %v/%v: %v",
			jobTemplate.Namespace, jobTemplate.Name, err)
		return err
	}
	return nil
}

// getAllCRDsByLabel lists resources using dynamic client
func (jt *worktemplatecontroller) getAllCRDsByLabel(
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
		list, err := jt.dynamicClient.Resource(gvr).List(ctx, listOptions)
		if err != nil {
			return nil, err
		}
		resultList = list.Items
	} else {
		// Namespaced resources
		list, err := jt.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, listOptions)
		if err != nil {
			return nil, err
		}
		resultList = list.Items
	}

	return resultList, nil
}
