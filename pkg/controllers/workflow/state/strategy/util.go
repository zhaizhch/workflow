package strategy

import (
	"strings"

	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

// findFlowByJobName 根据 job 名称在 Workflow.Spec.Flows 中查找匹配的 Flow 定义。
// 消除了多处重复的线性查找代码（DRY 原则）。
func findFlowByJobName(wf *v1alpha1.Workflow, jobName string) *v1alpha1.Flow {
	for i := range wf.Spec.Flows {
		if ContainsFlowName(jobName, wf.Spec.Flows[i].Name, wf.Name) {
			return &wf.Spec.Flows[i]
		}
	}
	return nil
}

// findFlowSpec 根据 flowName 精确匹配，返回 Flow 定义的指针。
// 与 findFlowByJobName 的区别：该函数使用精确名称匹配而非前缀匹配。
func findFlowSpec(wf *v1alpha1.Workflow, flowName string) *v1alpha1.Flow {
	for i := range wf.Spec.Flows {
		if wf.Spec.Flows[i].Name == flowName {
			return &wf.Spec.Flows[i]
		}
	}
	return nil
}

// GetPermanentlyFailedJobs 返回已永久失败（重试耗尽且无 ContinueOnFail）的 job 列表。
func GetPermanentlyFailedJobs(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus) []string {
	var failed []string
	for _, failedJobName := range status.FailedJobs {
		matchedFlow := findFlowByJobName(wf, failedJobName)

		if matchedFlow == nil {
			failed = append(failed, failedJobName) // 未知 Job，当作失败处理
			continue
		}

		// 配置了 ContinueOnFail 的 Job 不计入决性失败
		if matchedFlow.ContinueOnFail {
			continue
		}

		// 没有配置 Retry，直接认定失败
		if matchedFlow.Retry == nil {
			failed = append(failed, failedJobName)
			continue
		}

		// 检查重启次数，确认是否还可重试
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

// AreAllLeafPathsBlocked checks if all paths to valid leaf nodes are blocked by failed jobs
func AreAllLeafPathsBlocked(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus, failedJobs []string) bool {
	// Build map of failed flows for easier lookup
	failedFlows := make(map[string]bool)
	for _, job := range failedJobs {
		// Simplified: assuming job name contains flow name.
		for _, flow := range wf.Spec.Flows {
			if ContainsFlowName(job, flow.Name, wf.Name) {
				failedFlows[flow.Name] = true
			}
		}
	}

	graph := buildDependencyGraph(wf)
	leafNodes := findLeafNodes(graph, wf.Spec.Flows)

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

	if failedFlows[flowName] {
		cache[flowName] = true
		return true
	}

	// 使用统一的 findFlowSpec 函数，替代重复的线性查找代码
	flowSpec := findFlowSpec(wf, flowName)
	if flowSpec == nil {
		cache[flowName] = true
		return true
	}

	if flowSpec.DependsOn == nil {
		cache[flowName] = false
		return false
	}

	// AND targets
	for _, target := range flowSpec.DependsOn.Targets {
		if isNodeBlocked(target, wf, graph, failedFlows, cache) {
			cache[flowName] = true
			return true
		}
	}

	// OR groups
	if len(flowSpec.DependsOn.OrGroups) > 0 {
		allGroupsBlocked := true
		for _, group := range flowSpec.DependsOn.OrGroups {
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

	cache[flowName] = false
	return false
}

// AllCriticalFlowsSucceeded checks if all critical flows have succeeded
func AllCriticalFlowsSucceeded(criticalFlows []string, status *v1alpha1.WorkflowStatus, workflowName string) bool {
	if len(criticalFlows) == 0 {
		return false // Invalid configuration
	}

	for _, criticalFlow := range criticalFlows {
		found := false
		for _, completedJob := range status.CompletedJobs {
			if ContainsFlowName(completedJob, criticalFlow, workflowName) {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

// HasAnySuccessfulLeaf checks if there exists at least one successful leaf node
func HasAnySuccessfulLeaf(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus) bool {
	graph := buildDependencyGraph(wf)
	leafNodes := findLeafNodes(graph, wf.Spec.Flows)

	for _, leafNode := range leafNodes {
		if isFlowCompleted(leafNode, status, wf.Name) {
			return true
		}
	}

	return false
}

func buildDependencyGraph(wf *v1alpha1.Workflow) map[string][]string {
	graph := make(map[string][]string)

	for _, flow := range wf.Spec.Flows {
		deps := []string{}
		if flow.DependsOn != nil {
			deps = append(deps, flow.DependsOn.Targets...)
			for _, orGroup := range flow.DependsOn.OrGroups {
				deps = append(deps, orGroup.Targets...)
			}
		}
		graph[flow.Name] = deps
	}

	return graph
}

func findLeafNodes(graph map[string][]string, flows []v1alpha1.Flow) []string {
	hasDependents := make(map[string]bool)

	for _, deps := range graph {
		for _, dep := range deps {
			hasDependents[dep] = true
		}
	}

	var leafNodes []string
	for _, flow := range flows {
		if !hasDependents[flow.Name] {
			leafNodes = append(leafNodes, flow.Name)
		}
	}

	return leafNodes
}

func isFlowCompleted(flowName string, status *v1alpha1.WorkflowStatus, workflowName string) bool {
	for _, completedJob := range status.CompletedJobs {
		if ContainsFlowName(completedJob, flowName, workflowName) {
			return true
		}
	}
	return false
}

// ContainsFlowName checks if a job name contains the flow name
func ContainsFlowName(jobName, flowName, workflowName string) bool {
	prefix := workflowName + "-" + flowName
	return jobName == prefix || strings.HasPrefix(jobName, prefix+"-")
}
