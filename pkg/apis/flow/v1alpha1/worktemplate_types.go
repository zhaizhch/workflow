/*
Copyright 2021.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// // WorkTemplateSpec defines the desired state of WorkTemplate
// type WorkTemplateSpec struct {
// 	// JobSpec is the specification of the Job
// 	v1alpha1.JobSpec `json:"jobSpec,omitempty" protobuf:"bytes,1,opt,name=jobSpec"`
// }

// WorkTemplateSpec defines the desired state of WorkTemplate with GVK
type WorkTemplateSpec struct {
	// JobSpec is the specification of the Job
	JobSpec runtime.RawExtension `json:"jobSpec,omitempty" protobuf:"bytes,1,opt,name=jobSpec"`
	// GVR is the GroupVersionResource of the Job
	GVR metav1.GroupVersionResource `json:"gvr,omitempty" protobuf:"bytes,2,opt,name=gvr"`
}

// WorkTemplateStatus defines the observed state of WorkTemplate
type WorkTemplateStatus struct {
	// JobDependsOnList is the list of jobs that this job depends on
	JobDependsOnList []string `json:"jobDependsOnList,omitempty" protobuf:"bytes,1,rep,name=jobDependsOnList"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+kubebuilder:object:root=true
// +kubebuilder:resource:path=worktemplates,shortName=jt
//+kubebuilder:subresource:status

// WorkTemplate is the Schema for the worktemplates API
type WorkTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   WorkTemplateSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status WorkTemplateStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+kubebuilder:object:root=true

// WorkTemplateList contains a list of WorkTemplate
type WorkTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []WorkTemplate `json:"items" protobuf:"bytes,2,rep,name=items"`
}
