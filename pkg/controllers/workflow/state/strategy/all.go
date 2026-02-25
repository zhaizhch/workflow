package strategy

import (
	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

type AllStrategy struct{}

func (s *AllStrategy) IsSuccessful(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus, allJobList int) bool {
	return len(status.CompletedJobs) == allJobList
}

func (s *AllStrategy) IsFailed(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus) bool {
	permanentlyFailedJobs := GetPermanentlyFailedJobs(wf, status)
	return len(permanentlyFailedJobs) > 0
}
