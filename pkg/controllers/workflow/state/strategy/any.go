package strategy

import (
	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

type AnyStrategy struct{}

func (s *AnyStrategy) IsSuccessful(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus, allJobList int) bool {
	return HasAnySuccessfulLeaf(wf, status)
}

func (s *AnyStrategy) IsFailed(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus) bool {
	permanentlyFailedJobs := GetPermanentlyFailedJobs(wf, status)
	if len(permanentlyFailedJobs) == 0 {
		return false
	}
	// Fail only if all paths to leaf nodes are blocked
	return AreAllLeafPathsBlocked(wf, status, permanentlyFailedJobs)
}
