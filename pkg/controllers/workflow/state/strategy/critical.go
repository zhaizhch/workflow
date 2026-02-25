package strategy

import (
	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

type CriticalStrategy struct {
	CriticalFlows []string
}

func (s *CriticalStrategy) IsSuccessful(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus, allJobList int) bool {
	return AllCriticalFlowsSucceeded(s.CriticalFlows, status, wf.Name)
}

func (s *CriticalStrategy) IsFailed(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus) bool {
	permanentlyFailedJobs := GetPermanentlyFailedJobs(wf, status)
	if len(permanentlyFailedJobs) == 0 {
		return false
	}

	// Fail only if a critical flow has failed
	for _, failedJob := range permanentlyFailedJobs {
		for _, criticalFlow := range s.CriticalFlows {
			if ContainsFlowName(failedJob, criticalFlow, wf.Name) {
				return true
			}
		}
	}
	return false
}
