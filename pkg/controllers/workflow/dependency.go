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
	"net"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

func (wc *workflowcontroller) judge(workflow *v1alpha1flow.Workflow, dependsOn *v1alpha1flow.DependsOn, currentTaskName string, currentIndex int) (bool, error) {
	if dependsOn == nil {
		return true, nil
	}

	// Check the "local" group of targets and probe
	ok, err := wc.evaluateTargetsAndProbe(workflow, dependsOn.Targets, dependsOn.Probe, dependsOn.Strategy, currentTaskName, currentIndex)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}

	// Now check OrGroups (OR logic)
	for i := range dependsOn.OrGroups {
		group := dependsOn.OrGroups[i]
		ok, err := wc.evaluateTargetsAndProbe(workflow, group.Targets, group.Probe, group.Strategy, currentTaskName, currentIndex)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	return false, nil
}

func (wc *workflowcontroller) evaluateTargetsAndProbe(workflow *v1alpha1flow.Workflow, targets []string, probe *v1alpha1flow.Probe, strategy v1alpha1flow.DependencyStrategy, currentTaskName string, currentIndex int) (bool, error) {
	if len(targets) == 0 && probe == nil {
		return false, nil
	}

	for _, targetName := range targets {
		// Handle sequential dependency: if target is same as current task and we have an index
		if targetName == currentTaskName && currentIndex >= 0 {
			if currentIndex == 0 {
				continue // First replica has no previous one
			}
			// Check only the previous replica
			jobName := getJobName(workflow.Name, targetName, currentIndex-1)
			if wc.isJobTerminalSuccess(workflow, jobName) {
				continue
			}
			phase, err := wc.getJobStatusFromCluster(workflow, targetName, jobName)
			if err != nil {
				if errors.IsNotFound(err) {
					return false, nil
				}
				return false, err
			}
			if phase != v1alpha1.Completed {
				return false, nil
			}
			continue
		}

		// Normal dependency: check replicas based on strategy
		ok, err := wc.forEachReplica(workflow, targetName, strategy, func(jobName string, _ int) (bool, error) {
			if wc.isJobTerminalSuccess(workflow, jobName) {
				return true, nil
			}

			phase, err := wc.getJobStatusFromCluster(workflow, targetName, jobName)
			if err != nil {
				if errors.IsNotFound(err) {
					klog.Infof("Target job %s not found yet", jobName)
					return false, nil
				}
				return false, err
			}

			if phase != v1alpha1.Completed {
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	if probe != nil {
		ok, err := wc.evaluateProbe(workflow, probe, currentTaskName, currentIndex)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	return true, nil
}

// forEachReplica iterates over all replicas of a task and executes the provided function.
// If strategy is All, it returns true only if the function returns true for ALL replicas.
// If strategy is Any, it returns true if the function returns true for AT LEAST ONE replica.
func (wc *workflowcontroller) forEachReplica(workflow *v1alpha1flow.Workflow, taskName string, strategy v1alpha1flow.DependencyStrategy, fn func(jobName string, index int) (bool, error)) (bool, error) {
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
		jobName := getJobName(workflow.Name, taskName, index)
		ok, err := fn(jobName, index)
		if err != nil {
			return false, err
		}

		if strategy == v1alpha1flow.Any {
			if ok { // 如果是 Any 策略，只要有一个副本满足条件 (ok == true)，就立即返回 true
				return true, nil
			}
			// If not ok, continue to check next replica
		} else { // Default to All
			if !ok { // 如果是 All 策略，只要有一个副本不满足条件 (ok == false)，就立即返回 false
				return false, nil
			}
		}
	}

	if strategy == v1alpha1flow.Any { // 如果是 Any 策略且循环结束仍未返回，说明没有任何副本满足条件，返回 false
		return false, nil
	}
	return true, nil // 如果是 All 策略且循环顺利结束没有提前返回，说明所有副本都满足了条件，返回 true
}

func (wc *workflowcontroller) evaluateProbe(workflow *v1alpha1flow.Workflow, probe *v1alpha1flow.Probe, currentTaskName string, currentIndex int) (bool, error) {
	// Status checks
	for _, ts := range probe.TaskStatusList {
		// Handle sequential dependency in probe
		if ts.TaskName == currentTaskName && currentIndex >= 0 {
			if currentIndex == 0 {
				continue
			}
			jobName := getJobName(workflow.Name, ts.TaskName, currentIndex-1)
			phase, err := wc.getJobStatusFromCluster(workflow, ts.TaskName, jobName)
			if err != nil {
				return false, nil
			}

			satisfied := string(phase) == ts.Phase
			if !satisfied && ts.Phase == string(v1alpha1.Running) && phase == v1alpha1.Completed {
				satisfied = true
			}
			if !satisfied {
				return false, nil
			}
			continue
		}

		ok, err := wc.forEachReplica(workflow, ts.TaskName, v1alpha1flow.All, func(jobName string, _ int) (bool, error) {
			phase, err := wc.getJobStatusFromCluster(workflow, ts.TaskName, jobName)
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
		// Handle sequential dependency in HTTP probe
		if h.TaskName == currentTaskName && currentIndex >= 0 {
			if currentIndex == 0 {
				continue
			}
			jobName := getJobName(workflow.Name, h.TaskName, currentIndex-1)
			ip, err := wc.getJobIP(workflow, h.TaskName, jobName)
			if err != nil {
				return false, nil
			}
			url := fmt.Sprintf("http://%s:%d%s", ip, h.Port, h.Path)
			resp, err := client.Get(url)
			if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 400 {
				return false, nil
			}
			continue
		}

		ok, err := wc.forEachReplica(workflow, h.TaskName, v1alpha1flow.All, func(jobName string, _ int) (bool, error) {
			ip, err := wc.getJobIP(workflow, h.TaskName, jobName)
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
		// Handle sequential dependency in TCP probe
		if t.TaskName == currentTaskName && currentIndex >= 0 {
			if currentIndex == 0 {
				continue
			}
			jobName := getJobName(workflow.Name, t.TaskName, currentIndex-1)
			ip, err := wc.getJobIP(workflow, t.TaskName, jobName)
			if err != nil {
				return false, nil
			}
			address := net.JoinHostPort(ip, fmt.Sprintf("%d", t.Port))
			conn, err := net.DialTimeout("tcp", address, 2*time.Second)
			if err != nil {
				return false, nil
			}
			conn.Close()
			continue
		}

		ok, err := wc.forEachReplica(workflow, t.TaskName, v1alpha1flow.All, func(jobName string, _ int) (bool, error) {
			ip, err := wc.getJobIP(workflow, t.TaskName, jobName)
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
