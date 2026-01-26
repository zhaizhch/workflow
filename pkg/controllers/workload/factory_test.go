package workload

import (
	"testing"

	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type mockWorkload struct {
	gvrs []schema.GroupVersionResource
}

func (m *mockWorkload) GetGVR() []schema.GroupVersionResource { return m.gvrs }
func (m *mockWorkload) GetJobStatus(obj *unstructured.Unstructured) v1alpha1.JobStatus {
	return v1alpha1.JobStatus{}
}
func (m *mockWorkload) GetPodLabels(job *unstructured.Unstructured) map[string]string {
	return nil
}

func TestRegisterAndGet(t *testing.T) {
	gvr1 := schema.GroupVersionResource{Group: "group1", Version: "v1", Resource: "r1"}
	wl := &mockWorkload{
		gvrs: []schema.GroupVersionResource{gvr1},
	}

	Register(wl)

	got := GetWorkload("group1")
	if got == nil {
		t.Error("expected to find workload for group1")
	}

	got2 := GetWorkload("non-existent")
	if got2 != nil {
		t.Error("expected nil for non-existent group")
	}

	all := GetAllWorkloads()
	found := false
	for _, w := range all {
		if len(w.GetGVR()) > 0 && w.GetGVR()[0].Group == "group1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected mock workload in GetAllWorkloads")
	}
}
