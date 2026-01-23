package workload

import (
	"sort"
	"time"

	kubeflow "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

type kubeflowWorkload struct{}

func (k *kubeflowWorkload) GetJobStatus(job *unstructured.Unstructured) v1alpha1flow.JobStatus {
	jobStatus := v1alpha1flow.JobStatus{
		Name:           job.GetName(),
		StartTimestamp: job.GetCreationTimestamp(),
		State:          v1alpha1.Pending,
	}

	if conds, found, _ := unstructured.NestedSlice(job.Object, "status", "conditions"); found && len(conds) > 0 {
		sort.Slice(conds, func(i, j int) bool {
			c1 := conds[i].(map[string]interface{})
			c2 := conds[j].(map[string]interface{})
			t1Str, _ := c1["lastTransitionTime"].(string)
			t2Str, _ := c2["lastTransitionTime"].(string)
			t1, _ := time.Parse(time.RFC3339, t1Str)
			t2, _ := time.Parse(time.RFC3339, t2Str)
			return t1.Before(t2)
		})

		lastCond := conds[len(conds)-1].(map[string]interface{})
		t, _ := lastCond["type"].(string)
		s, _ := lastCond["status"].(string)

		if s == "True" {
			switch kubeflow.JobConditionType(t) {
			case kubeflow.JobSucceeded:
				jobStatus.State = v1alpha1.Completed
			case kubeflow.JobFailed:
				jobStatus.State = v1alpha1.Failed
			case kubeflow.JobRunning:
				jobStatus.State = v1alpha1.Running
			case kubeflow.JobCreated, kubeflow.JobRestarting, kubeflow.JobSuspended:
				jobStatus.State = v1alpha1.Pending
			}
		}
	}

	return jobStatus
}

func (k *kubeflowWorkload) GetGVR() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: "kubeflow.org", Version: "v1", Resource: "pytorchjobs"},
		{Group: "kubeflow.org", Version: "v1", Resource: "mpijobs"},
		{Group: "kubeflow.org", Version: "v1", Resource: "paddlejobs"},
	}
}

func (k *kubeflowWorkload) GetPodLabels(job *unstructured.Unstructured) map[string]string {
	return map[string]string{
		"training.kubeflow.org/job-name": job.GetName(),
	}
}

func init() {
	Register(&kubeflowWorkload{})
}
