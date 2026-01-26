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

package state

import (
	"strings"

	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

// isWorkflowSuccessful determines if workflow is successful based on SuccessPolicy
func isWorkflowSuccessful(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus, allJobList int) bool {
	policy := getSuccessPolicy(wf)

	switch policy.Type {
	case v1alpha1.SuccessPolicyCritical:
		return allCriticalFlowsSucceeded(policy.CriticalFlows, status)

	case v1alpha1.SuccessPolicyAny:
		return hasAnySuccessfulLeaf(wf, status)

	case v1alpha1.SuccessPolicyAll, "":
		// Default to All strategy
		return len(status.CompletedJobs) == allJobList

	default:
		// Fallback to All strategy for unknown types
		return len(status.CompletedJobs) == allJobList
	}
}

// getSuccessPolicy returns the success policy, defaulting to All if not specified
func getSuccessPolicy(wf *v1alpha1.Workflow) *v1alpha1.SuccessPolicy {
	if wf.Spec.SuccessPolicy == nil {
		return &v1alpha1.SuccessPolicy{
			Type: v1alpha1.SuccessPolicyAll,
		}
	}
	return wf.Spec.SuccessPolicy
}

// allCriticalFlowsSucceeded checks if all critical flows have succeeded
func allCriticalFlowsSucceeded(criticalFlows []string, status *v1alpha1.WorkflowStatus) bool {
	if len(criticalFlows) == 0 {
		return false // Invalid configuration
	}

	for _, criticalFlow := range criticalFlows {
		found := false
		// Check if the critical flow is in completed jobs
		for _, completedJob := range status.CompletedJobs {
			// Job name format: {workflow-name}-{flow-name} or {workflow-name}-{flow-name}-{index}
			if containsFlowName(completedJob, criticalFlow) {
				found = true
				break
			}
		}

		if !found {
			return false // Critical flow not completed
		}
	}

	return true
}

// hasAnySuccessfulLeaf checks if there exists at least one successful leaf node
func hasAnySuccessfulLeaf(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus) bool {
	// Build dependency graph
	graph := buildDependencyGraph(wf)

	// Find leaf nodes (nodes with no dependents)
	leafNodes := findLeafNodes(graph, wf.Spec.Flows)

	// For each leaf node, check if it's completed successfully
	for _, leafNode := range leafNodes {
		if isFlowCompleted(leafNode, status) {
			return true // Found at least one successful leaf node
		}
	}

	return false
}

// buildDependencyGraph builds a map of flow dependencies
// Key: flow name, Value: list of flows it depends on
func buildDependencyGraph(wf *v1alpha1.Workflow) map[string][]string {
	graph := make(map[string][]string)

	for _, flow := range wf.Spec.Flows {
		deps := []string{}

		if flow.DependsOn != nil {
			// Add simple targets
			deps = append(deps, flow.DependsOn.Targets...)

			// Add OR group targets (any one group is sufficient)
			for _, orGroup := range flow.DependsOn.OrGroups {
				deps = append(deps, orGroup.Targets...)
			}
		}

		graph[flow.Name] = deps
	}

	return graph
}

// findLeafNodes finds all nodes that have no dependents (i.e., no other node depends on them)
func findLeafNodes(graph map[string][]string, flows []v1alpha1.Flow) []string {
	// Track which nodes are dependencies of others
	hasDependents := make(map[string]bool)

	for _, deps := range graph {
		for _, dep := range deps {
			hasDependents[dep] = true
		}
	}

	// Leaf nodes are those that don't have dependents
	var leafNodes []string
	for _, flow := range flows {
		if !hasDependents[flow.Name] {
			leafNodes = append(leafNodes, flow.Name)
		}
	}

	return leafNodes
}

// isFlowCompleted checks if a flow is in the completed jobs list
func isFlowCompleted(flowName string, status *v1alpha1.WorkflowStatus) bool {
	for _, completedJob := range status.CompletedJobs {
		if containsFlowName(completedJob, flowName) {
			return true
		}
	}
	return false
}

// containsFlowName checks if a job name contains the flow name
// Job name format: {workflow-name}-{flow-name} or {workflow-name}-{flow-name}-{index}
func containsFlowName(jobName, flowName string) bool {
	return strings.Contains(jobName, flowName)
}
