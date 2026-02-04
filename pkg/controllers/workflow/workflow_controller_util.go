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
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

func (wc *workflowcontroller) isContinueOnFail(workflow *v1alpha1flow.Workflow, flowName string) bool {
	for _, flow := range workflow.Spec.Flows {
		if flow.Name == flowName {
			return flow.ContinueOnFail
		}
	}
	return false
}

func getJobName(workflowName string, workTemplateName string, index int) string {
	if index >= 0 {
		return fmt.Sprintf("%s-%s-%d", workflowName, workTemplateName, index)
	}
	return workflowName + "-" + workTemplateName
}

// GenerateObjectString generates the object information string using namespace and name
func GenerateObjectString(namespace, name string) string {
	return namespace + "." + name
}

func isControlledBy(obj metav1.Object, gvk schema.GroupVersionKind) bool {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return false
	}
	if controllerRef.APIVersion == gvk.GroupVersion().String() && controllerRef.Kind == gvk.Kind {
		return true
	}
	return false
}

func getWorkflowNameByObj(obj metav1.Object) string {
	for _, owner := range obj.GetOwnerReferences() {
		if owner.Kind == Workflow && strings.Contains(owner.APIVersion, WorkflowAPIVersion) {
			return owner.Name
		}
	}
	return ""
}
