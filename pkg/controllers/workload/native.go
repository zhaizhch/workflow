package workload

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

type nativeWorkload struct{}

func (n *nativeWorkload) GetJobStatus(job *unstructured.Unstructured) v1alpha1flow.JobStatus {
	jobStatus := v1alpha1flow.JobStatus{
		Name:           job.GetName(),
		StartTimestamp: job.GetCreationTimestamp(),
		State:          v1alpha1.Pending,
	}

	// For Deployment, we check if availableReplicas > 0
	availableReplicas, found, _ := unstructured.NestedInt64(job.Object, "status", "availableReplicas")
	if found && availableReplicas > 0 {
		jobStatus.State = v1alpha1.Running
	}

	if kubeClient != nil {
		specReplicas, found, _ := unstructured.NestedInt64(job.Object, "spec", "replicas")
		if !found {
			specReplicas = 1
		}

		matchLabels, found, _ := unstructured.NestedStringMap(job.Object, "spec", "selector", "matchLabels")
		if found {
			selector := labels.SelectorFromSet(matchLabels)
			pods, err := kubeClient.CoreV1().Pods(job.GetNamespace()).List(context.TODO(), metav1.ListOptions{
				LabelSelector: selector.String(),
			})
			if err != nil {
				klog.Errorf("Failed to list pods for deployment %s/%s: %v", job.GetNamespace(), job.GetName(), err)
				return jobStatus
			}

			completedPods := 0
			for _, pod := range pods.Items {
				if pod.Status.Phase == corev1.PodSucceeded {
					completedPods++
				}
			}

			if int64(completedPods) == specReplicas {
				jobStatus.State = v1alpha1.Completed
			}
		}
	}

	return jobStatus
}

func (n *nativeWorkload) GetGVR() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: "apps", Version: "v1", Resource: "deployments"},
	}
}

func (n *nativeWorkload) GetPodLabels(job *unstructured.Unstructured) map[string]string {
	matchLabels, found, _ := unstructured.NestedStringMap(job.Object, "spec", "selector", "matchLabels")
	if found {
		return matchLabels
	}
	return nil
}

func init() {
	Register(&nativeWorkload{})
}
