package workload

import (
	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

type batchWorkload struct{}

func (v *batchWorkload) GetJobStatus(job *unstructured.Unstructured) v1alpha1flow.JobStatus {
	jobStatus := v1alpha1flow.JobStatus{
		Name:           job.GetName(),
		StartTimestamp: job.GetCreationTimestamp(),
		State:          v1alpha1.Pending,
	}

	phase, found, _ := unstructured.NestedString(job.Object, "status", "state", "phase")
	if found {
		jobStatus.State = v1alpha1.JobPhase(phase)
	} else if succeeded, found, _ := unstructured.NestedInt64(job.Object, "status", "succeeded"); found && succeeded > 0 {
		jobStatus.State = v1alpha1.Completed
	}

	return jobStatus
}

func (v *batchWorkload) GetGVR() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: "batch.volcano.sh", Version: "v1alpha1", Resource: "jobs"},
	}
}

func (v *batchWorkload) GetPodLabels(job *unstructured.Unstructured) map[string]string {
	return map[string]string{
		"volcano.sh/job-name": job.GetName(),
	}
}

func init() {
	Register(&batchWorkload{})
}
