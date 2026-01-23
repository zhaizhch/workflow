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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/workflow.sh/work-flow/pkg/controllers/util"
)

var (
	workflowSucceedPhaseCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: util.WorkflowSubSystemName,
			Name:      "workflow_succeed_phase_count",
			Help:      "Number of workflow succeed phase",
		}, []string{"workflow_namespace"},
	)

	workflowFailedPhaseCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: util.WorkflowSubSystemName,
			Name:      "workflow_failed_phase_count",
			Help:      "Number of workflow failed phase",
		}, []string{"workflow_namespace"},
	)
)

func UpdateWorkflowSucceed(namespace string) {
	workflowSucceedPhaseCount.WithLabelValues(namespace).Inc()
}

func UpdateWorkflowFailed(namespace string) {
	workflowFailedPhaseCount.WithLabelValues(namespace).Inc()
}

func DeleteWorkflowMetrics(namespace string) {
	workflowSucceedPhaseCount.DeleteLabelValues(namespace)
	workflowFailedPhaseCount.DeleteLabelValues(namespace)
}
