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
	"hash/fnv"
	"strings"

	"k8s.io/klog/v2"

	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"github.com/workflow.sh/work-flow/pkg/controllers/apis"
	batch "volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

func (jt *worktemplatecontroller) enqueue(req apis.FlowRequest) {
	key := req.Namespace + "/" + req.WorkTemplateName
	hash := fnv.New32a()
	hash.Write([]byte(key))
	index := int(hash.Sum32()) % jt.workerNum
	jt.queues[index].Add(req)
}

func (jt *worktemplatecontroller) addWorkTemplate(obj interface{}) {
	jobTemplate, ok := obj.(*v1alpha1.WorkTemplate)
	if !ok {
		klog.Errorf("Failed to convert %v to jobTemplate", obj)
		return
	}

	req := apis.FlowRequest{
		Namespace:        jobTemplate.Namespace,
		WorkTemplateName: jobTemplate.Name,
	}

	jt.enqueueWorkTemplate(req)
}

func (jt *worktemplatecontroller) addJob(obj interface{}) {
	job, ok := obj.(*batch.Job)
	if !ok {
		klog.Errorf("Failed to convert %v to vcjob", obj)
		return
	}

	if job.Labels[CreatedByWorkTemplate] == "" {
		return
	}

	//Filter workloads created by Workflow
	namespaceName := strings.Split(job.Labels[CreatedByWorkTemplate], ".")
	if len(namespaceName) != CreateByWorkTemplateValueNum {
		return
	}
	namespace, name := namespaceName[0], namespaceName[1]

	req := apis.FlowRequest{
		Namespace:        namespace,
		WorkTemplateName: name,
	}
	jt.enqueueWorkTemplate(req)
}
