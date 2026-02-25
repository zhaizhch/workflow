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

package validate

import (
	"errors"
	"fmt"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	whv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	flowv1alpha1 "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"github.com/workflow.sh/work-flow/pkg/webhooks/router"
	"github.com/workflow.sh/work-flow/pkg/webhooks/schema"
	"github.com/workflow.sh/work-flow/pkg/webhooks/util"
)

var (
	VertexNotDefinedError      = errors.New("vertex is not defined")
	FlowNotDAGError            = errors.New("workflow Flow is not DAG")
	OperationNotCreateOrUpdate = errors.New("expect operation to be 'CREATE' or 'UPDATE'")
)

func init() {
	router.RegisterAdmission(service)
}

var service = &router.AdmissionService{
	Path:   "/workflows/validate",
	Func:   AdmitWorkflows,
	Config: config,
	ValidatingConfig: &whv1.ValidatingWebhookConfiguration{
		Webhooks: []whv1.ValidatingWebhook{{
			Name: "validateworkflow.workflow.sh",
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

// AdmitWorkflows is to admit workflows and return response.
func AdmitWorkflows(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	klog.V(3).Infof("admitting workflow -- %s", ar.Request.Operation)

	jobFlow, err := schema.DecodeWorkflow(ar.Request.Object, ar.Request.Resource)
	if err != nil {
		return util.ToAdmissionResponse(err)
	}

	var msg string
	reviewResponse := admissionv1.AdmissionResponse{}
	reviewResponse.Allowed = true

	switch ar.Request.Operation {
	case admissionv1.Create, admissionv1.Update:
		msg = validateWorkflowDAG(jobFlow, &reviewResponse)
	default:
		err := OperationNotCreateOrUpdate
		return util.ToAdmissionResponse(err)
	}

	if !reviewResponse.Allowed {
		reviewResponse.Result = &metav1.Status{Message: strings.TrimSpace(msg)}
	}
	return &reviewResponse
}

func validateWorkflowDAG(workflow *flowv1alpha1.Workflow, reviewResponse *admissionv1.AdmissionResponse) string {
	// Check for empty workflow
	if len(workflow.Spec.Flows) == 0 {
		reviewResponse.Allowed = false
		return "workflow must have at least one flow"
	}

	// Validate SuccessPolicy
	if msg := validateSuccessPolicy(workflow); msg != "" {
		reviewResponse.Allowed = false
		return msg
	}

	graphMap, msg := buildDependencyGraph(workflow.Spec.Flows)
	if msg != "" {
		reviewResponse.Allowed = false
		return msg
	}

	vertices, err := LoadVertexs(graphMap)
	if err != nil {
		reviewResponse.Allowed = false
		return FlowNotDAGError.Error() + ": " + err.Error()
	}

	if !IsDAG(vertices) {
		reviewResponse.Allowed = false
		return FlowNotDAGError.Error()
	}

	return ""
}

// buildDependencyGraph constructs a dependency graph map from the given flows
// and returns an error message if any self-referencing dependencies are detected.
func buildDependencyGraph(flows []flowv1alpha1.Flow) (map[string][]string, string) {
	graphMap := make(map[string][]string, len(flows))

	for _, flow := range flows {
		var allDeps []string

		if flow.DependsOn != nil {
			// 1. Collect AND dependencies (Targets)
			allDeps = append(allDeps, flow.DependsOn.Targets...)

			// 2. Collect OR dependencies (OrGroups)
			// Use conservative strategy: include all possible dependencies
			for _, group := range flow.DependsOn.OrGroups {
				allDeps = append(allDeps, group.Targets...)
			}
		}

		// 3. Check For loop dependencies
		if flow.For != nil && flow.For.DependsOn != nil {
			// Check for self-references in For dependencies
			for _, target := range flow.For.DependsOn.Targets {
				if target == flow.Name {
					return nil, fmt.Sprintf("flow '%s' has self-referencing For dependency", flow.Name)
				}
			}

			// Check for self-references in For OrGroups
			for _, group := range flow.For.DependsOn.OrGroups {
				for _, target := range group.Targets {
					if target == flow.Name {
						return nil, fmt.Sprintf("flow '%s' has self-referencing For dependency in OrGroups", flow.Name)
					}
				}
			}
		}

		graphMap[flow.Name] = allDeps
	}

	return graphMap, ""
}

// validateSuccessPolicy validates the SuccessPolicy configuration
func validateSuccessPolicy(workflow *flowv1alpha1.Workflow) string {
	if workflow.Spec.SuccessPolicy == nil {
		return "" // Default to All, no validation needed
	}

	policy := workflow.Spec.SuccessPolicy

	switch policy.Type {
	case flowv1alpha1.SuccessPolicyCritical:
		// Validate CriticalFlows is not empty
		if len(policy.CriticalFlows) == 0 {
			return "successPolicy.criticalFlows is required when type is Critical"
		}

		// Validate all CriticalFlows exist in Flows
		flowNames := make(map[string]bool)
		for _, flow := range workflow.Spec.Flows {
			flowNames[flow.Name] = true
		}

		for _, criticalFlow := range policy.CriticalFlows {
			if !flowNames[criticalFlow] {
				return fmt.Sprintf("criticalFlow '%s' not found in workflow flows", criticalFlow)
			}
		}

	case flowv1alpha1.SuccessPolicyAll, flowv1alpha1.SuccessPolicyAny:
		if len(policy.CriticalFlows) > 0 {
			return fmt.Sprintf("successPolicy.criticalFlows should be empty when type is %s", policy.Type)
		}

	case "":
		// Empty type defaults to All
		if len(policy.CriticalFlows) > 0 {
			return "successPolicy.criticalFlows should be empty when type is not specified (defaults to All)"
		}

	default:
		return fmt.Sprintf("unknown successPolicy type: %s (must be All, Any, or Critical)", policy.Type)
	}

	return ""
}
