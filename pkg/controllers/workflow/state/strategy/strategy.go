package strategy

import (
	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

// SuccessStrategy defines the interface for determining workflow success or failure.
type SuccessStrategy interface {
	IsSuccessful(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus, allJobList int) bool
	IsFailed(wf *v1alpha1.Workflow, status *v1alpha1.WorkflowStatus) bool
}
