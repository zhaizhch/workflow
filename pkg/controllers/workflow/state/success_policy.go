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

// isWorkflowFailed determines if workflow is failed based on SuccessPolicy and ContinueOnFail
func isWorkflowFailed(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus) bool {
	policy := getSuccessPolicy(wf)

	// Get list of effectively failed jobs (retries exhausted AND not ContinueOnFail)
	permanentlyFailedJobs := getPermanentlyFailedJobs(wf, status)

	if len(permanentlyFailedJobs) == 0 {
		return false
	}

	switch policy.Type {
	case v1alpha1.SuccessPolicyCritical:
		// Fail only if a critical flow has failed
		for _, failedJob := range permanentlyFailedJobs {
			for _, criticalFlow := range policy.CriticalFlows {
				if containsFlowName(failedJob, criticalFlow) {
					return true
				}
			}
		}
		return false

	case v1alpha1.SuccessPolicyAny:
		// Fail only if all paths to leaf nodes are blocked
		return areAllLeafPathsBlocked(wf, status, permanentlyFailedJobs)

	case v1alpha1.SuccessPolicyAll, "":
		// Fail if ANY job has failed (and wasn't skipped/continued logic handled in getPermanentlyFailedJobs)
		return len(permanentlyFailedJobs) > 0

	default:
		return len(permanentlyFailedJobs) > 0
	}
}

// getPermanentlyFailedJobs returns jobs that are failed, retries exhausted, and NOT ContinueOnFail
func getPermanentlyFailedJobs(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus) []string {
	var failed []string
	for _, failedJobName := range status.FailedJobs {
		// Find flow spec
		var matchedFlow *v1alpha1.Flow
		for _, flow := range wf.Spec.Flows {
			if containsFlowName(failedJobName, flow.Name) {
				matchedFlow = &flow
				break
			}
		}

		if matchedFlow == nil {
			failed = append(failed, failedJobName) // Unknown job, treat as failed
			continue
		}

		// Check ContinueOnFail
		if matchedFlow.ContinueOnFail {
			continue // Ignored failure
		}

		// Check Retries
		if matchedFlow.Retry == nil {
			failed = append(failed, failedJobName)
			continue
		}

		// Check restart count
		canRetry := false
		for _, js := range status.JobStatusList {
			if js.Name == failedJobName {
				if js.RestartCount < matchedFlow.Retry.MaxRetries {
					canRetry = true
				}
				break
			}
		}

		if !canRetry {
			failed = append(failed, failedJobName)
		}
	}
	return failed
}

// areAllLeafPathsBlocked checks if all paths to valid leaf nodes are blocked by failed jobs
func areAllLeafPathsBlocked(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus, failedJobs []string) bool {
	// Build map of failed flows for easier lookup
	failedFlows := make(map[string]bool)
	for _, job := range failedJobs {
		// Simplified: assuming job name contains flow name.
		// Ideally we map job -> flow exactly.
		// Here we mark the flow as failed if any of its replicas failed.
		for _, flow := range wf.Spec.Flows {
			if containsFlowName(job, flow.Name) {
				failedFlows[flow.Name] = true
			}
		}
	}

	graph := buildDependencyGraph(wf)
	leafNodes := findLeafNodes(graph, wf.Spec.Flows)

	// A node is "blocked" if it is failed, OR if its dependencies are blocking it.
	// We want to see if ANY leaf node is NOT blocked.
	// Memoization for blocked status: map[flowName]bool
	blockedCache := make(map[string]bool)

	for _, leaf := range leafNodes {
		if !isNodeBlocked(leaf, wf, graph, failedFlows, blockedCache) {
			return false // Found a path!
		}
	}

	return true // All paths blocked
}

func isNodeBlocked(flowName string, wf *v1alpha1.Workflow, graph map[string][]string, failedFlows map[string]bool, cache map[string]bool) bool {
	if blocked, ok := cache[flowName]; ok {
		return blocked
	}

	// 1. If flow itself is failed, it's blocked
	if failedFlows[flowName] {
		cache[flowName] = true
		return true
	}

	// 2. If no dependencies, it's not blocked (it's a root that hasn't failed)
	// deps := graph[flowName] // Unused, we use flowSpec directly for details

	// Find the flow spec to check detailed dependency logic (OrGroups)
	var flowSpec *v1alpha1.Flow
	for _, f := range wf.Spec.Flows {
		if f.Name == flowName {
			flowSpec = &f
			break
		}
	}

	if flowSpec == nil {
		// Should not happen, treat as blocked?
		return true
	}

	if flowSpec.DependsOn == nil {
		cache[flowName] = false
		return false
	}

	// 3. Check detailed dependencies
	// AND targets
	for _, target := range flowSpec.DependsOn.Targets {
		if isNodeBlocked(target, wf, graph, failedFlows, cache) {
			cache[flowName] = true
			return true
		}
	}

	// OR groups (Any group must be unblocked)
	if len(flowSpec.DependsOn.OrGroups) > 0 {
		allGroupsBlocked := true
		for _, group := range flowSpec.DependsOn.OrGroups {
			// A group is blocked if ANY of its targets are blocked
			groupBlocked := false
			for _, target := range group.Targets {
				if isNodeBlocked(target, wf, graph, failedFlows, cache) {
					groupBlocked = true
					break
				}
			}
			if !groupBlocked {
				allGroupsBlocked = false
				break
			}
		}

		if allGroupsBlocked {
			cache[flowName] = true
			return true
		}
	}

	// If we passed checks:
	// - Not failed itself
	// - All AND targets are unblocked
	// - At least one OR group (if any) is unblocked
	cache[flowName] = false
	return false
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
// Note: This simple graph just lists all dependencies, doesn't distinguish AND/OR for finding structure
// But isNodeBlocked handles the logic correctly using flowSpec
func buildDependencyGraph(wf *v1alpha1.Workflow) map[string][]string {
	graph := make(map[string][]string)

	for _, flow := range wf.Spec.Flows {
		deps := []string{}

		if flow.DependsOn != nil {
			// Add simple targets
			deps = append(deps, flow.DependsOn.Targets...)

			// Add OR group targets
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
