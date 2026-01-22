package workload

import (
	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Workload interface {
	GetJobStatus(job *unstructured.Unstructured) v1alpha1flow.JobStatus
	GetGVR() []schema.GroupVersionResource
}
