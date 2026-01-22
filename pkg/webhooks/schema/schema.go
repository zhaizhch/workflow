/*
Copyright 2019 The Volcano Authors.

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

package schema

import (
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"

	flowv1alpha1 "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

func init() {
	addToScheme(scheme)
}

var scheme = runtime.NewScheme()

// Codecs is for retrieving serializers for the supported wire formats
// and conversion wrappers to define preferred internal and external versions.
var Codecs = serializer.NewCodecFactory(scheme)

func addToScheme(scheme *runtime.Scheme) {
	utilruntime.Must(v1.AddToScheme(scheme))
	utilruntime.Must(admissionv1.AddToScheme(scheme))
	utilruntime.Must(flowv1alpha1.AddToScheme(scheme))
}

// DecodeWorkflow decodes the job using deserializer from the raw object.
func DecodeWorkflow(object runtime.RawExtension, resource metav1.GroupVersionResource) (*flowv1alpha1.Workflow, error) {
	jobFlowResource := metav1.GroupVersionResource{Group: flowv1alpha1.SchemeGroupVersion.Group, Version: flowv1alpha1.SchemeGroupVersion.Version, Resource: "workflows"}
	raw := object.Raw
	jobFlow := flowv1alpha1.Workflow{}

	if resource != jobFlowResource {
		klog.Errorf("expect resource to be %s", jobFlowResource)
		return &jobFlow, fmt.Errorf("expect resource to be %s", jobFlowResource)
	}

	deserializer := Codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(raw, nil, &jobFlow); err != nil {
		return &jobFlow, err
	}
	klog.V(3).Infof("the workflow struct is %+v", jobFlow)

	return &jobFlow, nil
}
