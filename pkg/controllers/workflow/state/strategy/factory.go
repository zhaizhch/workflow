package strategy

import (
	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

// Factory returns the appropriate SuccessStrategy based on the provided policy.
func GetStrategy(policy *v1alpha1.SuccessPolicy) SuccessStrategy {
	if policy == nil {
		return &AllStrategy{}
	}

	switch policy.Type {
	case v1alpha1.SuccessPolicyCritical:
		return &CriticalStrategy{CriticalFlows: policy.CriticalFlows}
	case v1alpha1.SuccessPolicyAny:
		return &AnyStrategy{}
	case v1alpha1.SuccessPolicyAll, "":
		return &AllStrategy{}
	default:
		return &AllStrategy{}
	}
}
