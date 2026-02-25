package engine

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

// JobDataProvider defines the interface to fetch job specific data from the Kubernetes cluster
type JobDataProvider interface {
	IsJobTerminalSuccess(workflow *v1alpha1flow.Workflow, jobName string) bool
	GetJobStatusFromCluster(workflow *v1alpha1flow.Workflow, targetName, jobName string) (v1alpha1.JobPhase, error)
	IsContinueOnFail(workflow *v1alpha1flow.Workflow, targetName string) bool
	GetJobIP(workflow *v1alpha1flow.Workflow, targetName, jobName string) (string, error)
	GetJobName(workflowName, taskName string, index int) string
}

// DagEngine evaluates task dependencies and probes
type DagEngine struct {
	provider JobDataProvider
}

// NewDagEngine returns a new DAG engine
func NewDagEngine(provider JobDataProvider) *DagEngine {
	return &DagEngine{
		provider: provider,
	}
}

// Judge evaluates if the current task satisfies all dependencies
func (e *DagEngine) Judge(workflow *v1alpha1flow.Workflow, dependsOn *v1alpha1flow.DependsOn, currentTaskName string, currentIndex int) (bool, error) {
	if dependsOn == nil {
		return true, nil
	}

	// Check the "local" group of targets and probe
	ok, err := e.evaluateTargetsAndProbe(workflow, dependsOn.Targets, dependsOn.Probe, dependsOn.Strategy, currentTaskName, currentIndex)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}

	// Now check OrGroups (OR logic)
	for i := range dependsOn.OrGroups {
		group := dependsOn.OrGroups[i]
		ok, err := e.evaluateTargetsAndProbe(workflow, group.Targets, group.Probe, group.Strategy, currentTaskName, currentIndex)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	return false, nil
}

func (e *DagEngine) evaluateTargetsAndProbe(workflow *v1alpha1flow.Workflow, targets []string, probe *v1alpha1flow.Probe, strategy v1alpha1flow.DependencyStrategy, currentTaskName string, currentIndex int) (bool, error) {
	if len(targets) == 0 && probe == nil {
		return false, nil
	}

	for _, targetName := range targets {
		ok, err := e.checkCondition(workflow, targetName, strategy, currentTaskName, currentIndex, func(jobName string, _ int) (bool, error) {
			if e.provider.IsJobTerminalSuccess(workflow, jobName) {
				return true, nil
			}

			phase, err := e.provider.GetJobStatusFromCluster(workflow, targetName, jobName)
			if err != nil {
				if errors.IsNotFound(err) {
					klog.V(4).Infof("Target job %s not found yet", jobName)
					return false, nil
				}
				return false, err
			}

			if phase == v1alpha1.Completed {
				return true, nil
			}
			// Check if failed but ContinueOnFail
			if phase == v1alpha1.Failed && e.provider.IsContinueOnFail(workflow, targetName) {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	if probe != nil {
		ok, err := e.evaluateProbe(workflow, probe, currentTaskName, currentIndex)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	return true, nil
}

// ForEachReplica iterates over all replicas of a task and executes the provided function.
func (e *DagEngine) ForEachReplica(workflow *v1alpha1flow.Workflow, taskName string, strategy v1alpha1flow.DependencyStrategy, fn func(jobName string, index int) (bool, error)) (bool, error) {
	var targetFlow *v1alpha1flow.Flow
	for _, f := range workflow.Spec.Flows {
		if f.Name == taskName {
			targetFlow = &f
			break
		}
	}
	if targetFlow == nil {
		return false, fmt.Errorf("target flow %s not found in workflow", taskName)
	}

	replicas := 1
	if targetFlow.For != nil && targetFlow.For.Replicas != nil {
		replicas = int(*targetFlow.For.Replicas)
	}

	for i := 0; i < replicas; i++ {
		index := -1
		if targetFlow.For != nil {
			index = i
		}
		jobName := e.provider.GetJobName(workflow.Name, taskName, index)
		ok, err := fn(jobName, index)
		if err != nil {
			return false, err
		}

		if strategy == v1alpha1flow.Any {
			if ok {
				return true, nil
			}
		} else { // Default to All
			if !ok {
				return false, nil
			}
		}
	}

	if strategy == v1alpha1flow.Any {
		return false, nil
	}
	return true, nil
}

func (e *DagEngine) checkCondition(workflow *v1alpha1flow.Workflow, targetName string, strategy v1alpha1flow.DependencyStrategy, currentTaskName string, currentIndex int, fn func(jobName string, index int) (bool, error)) (bool, error) {
	if targetName == currentTaskName && currentIndex >= 0 {
		if currentIndex == 0 {
			return true, nil
		}
		jobName := e.provider.GetJobName(workflow.Name, targetName, currentIndex-1)
		return fn(jobName, currentIndex-1)
	}
	return e.ForEachReplica(workflow, targetName, strategy, fn)
}

func (e *DagEngine) evaluateProbe(workflow *v1alpha1flow.Workflow, probe *v1alpha1flow.Probe, currentTaskName string, currentIndex int) (bool, error) {
	// Status checks
	for _, ts := range probe.TaskStatusList {
		ok, err := e.checkCondition(workflow, ts.TaskName, v1alpha1flow.All, currentTaskName, currentIndex, func(jobName string, _ int) (bool, error) {
			phase, err := e.provider.GetJobStatusFromCluster(workflow, ts.TaskName, jobName)
			if err != nil {
				return false, nil // Not found or error means not satisfied
			}

			satisfied := string(phase) == ts.Phase
			if !satisfied && ts.Phase == string(v1alpha1.Running) && phase == v1alpha1.Completed {
				satisfied = true
			}
			return satisfied, nil
		})
		if !ok || err != nil {
			return ok, err
		}
	}

	// HTTP Probe
	client := &http.Client{Timeout: 2 * time.Second}
	for _, h := range probe.HttpGetList {
		ok, err := e.checkCondition(workflow, h.TaskName, v1alpha1flow.All, currentTaskName, currentIndex, func(jobName string, _ int) (bool, error) {
			ip, err := e.provider.GetJobIP(workflow, h.TaskName, jobName)
			if err != nil {
				return false, nil
			}
			url := fmt.Sprintf("http://%s:%d%s", ip, h.Port, h.Path)
			resp, err := client.Get(url)
			if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 400 {
				return false, nil
			}
			return true, nil
		})
		if !ok || err != nil {
			return ok, err
		}
	}

	// TCP Probe
	for _, t := range probe.TcpSocketList {
		ok, err := e.checkCondition(workflow, t.TaskName, v1alpha1flow.All, currentTaskName, currentIndex, func(jobName string, _ int) (bool, error) {
			ip, err := e.provider.GetJobIP(workflow, t.TaskName, jobName)
			if err != nil {
				return false, nil
			}
			address := net.JoinHostPort(ip, fmt.Sprintf("%d", t.Port))
			conn, err := net.DialTimeout("tcp", address, 2*time.Second)
			if err != nil {
				return false, nil
			}
			conn.Close()
			return true, nil
		})
		if !ok || err != nil {
			return ok, err
		}
	}

	return true, nil
}
