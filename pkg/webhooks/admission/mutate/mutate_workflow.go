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

package mutate

import (
	"encoding/json"

	admissionv1 "k8s.io/api/admission/v1"
	whv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/klog/v2"

	flowv1alpha1 "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"github.com/workflow.sh/work-flow/pkg/webhooks/router"
	"github.com/workflow.sh/work-flow/pkg/webhooks/schema"
	"github.com/workflow.sh/work-flow/pkg/webhooks/util"
)

func init() {
	router.RegisterAdmission(service)
}

var service = &router.AdmissionService{
	Path:   "/workflows/mutate",
	Func:   MutateWorkflows,
	Config: config,
	MutatingConfig: &whv1.MutatingWebhookConfiguration{
		Webhooks: []whv1.MutatingWebhook{{
			Name: "mutateworkflow.workflow.sh",
			Rules: []whv1.RuleWithOperations{
				{
					Operations: []whv1.OperationType{whv1.Create, whv1.Update},
					Rule: whv1.Rule{
						APIGroups:   []string{"flow.workflow.sh"},
						APIVersions: []string{"v1alpha1"},
						Resources:   []string{"workflows"},
					},
				},
			},
		}},
	},
}

var config = &router.AdmissionServiceConfig{}

// MutateWorkflows is to mutate workflows and return response.
func MutateWorkflows(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	klog.V(3).Infof("mutating workflow -- %s", ar.Request.Operation)

	workflow, err := schema.DecodeWorkflow(ar.Request.Object, ar.Request.Resource)
	if err != nil {
		return util.ToAdmissionResponse(err)
	}

	reviewResponse := admissionv1.AdmissionResponse{}
	reviewResponse.Allowed = true

	var patchBytes []byte
	switch ar.Request.Operation {
	case admissionv1.Create, admissionv1.Update:
		patchBytes, err = mutateWorkflow(workflow)
	}

	if err != nil {
		return util.ToAdmissionResponse(err)
	}

	if patchBytes != nil {
		reviewResponse.Patch = patchBytes
		pt := admissionv1.PatchTypeJSONPatch
		reviewResponse.PatchType = &pt
	}

	return &reviewResponse
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func mutateWorkflow(workflow *flowv1alpha1.Workflow) ([]byte, error) {
	var patches []patchOperation

	// Default SuccessPolicy to All if not provided
	if workflow.Spec.SuccessPolicy == nil {
		patches = append(patches, patchOperation{
			Op:   "add",
			Path: "/spec/successPolicy",
			Value: flowv1alpha1.SuccessPolicy{
				Type: flowv1alpha1.SuccessPolicyAll,
			},
		})
	} else if workflow.Spec.SuccessPolicy.Type == "" {
		patches = append(patches, patchOperation{
			Op:    "add",
			Path:  "/spec/successPolicy/type",
			Value: flowv1alpha1.SuccessPolicyAll,
		})
	}

	if len(patches) == 0 {
		return nil, nil
	}

	return json.Marshal(patches)
}
