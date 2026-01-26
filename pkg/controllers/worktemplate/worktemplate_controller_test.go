/*
Copyright 2026 zhaizhicheng.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package worktemplate

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	discovery "k8s.io/client-go/discovery"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	flowfake "github.com/workflow.sh/work-flow/pkg/client/clientset/versioned/fake"
	flowinformer "github.com/workflow.sh/work-flow/pkg/client/informers/externalversions"
	"github.com/workflow.sh/work-flow/pkg/controllers/apis"
	"github.com/workflow.sh/work-flow/pkg/controllers/framework"
	batchversioned "volcano.sh/apis/pkg/client/clientset/versioned"
	batchv1alpha1 "volcano.sh/apis/pkg/client/clientset/versioned/typed/batch/v1alpha1"
	busv1alpha1 "volcano.sh/apis/pkg/client/clientset/versioned/typed/bus/v1alpha1"
	flowv1alpha1 "volcano.sh/apis/pkg/client/clientset/versioned/typed/flow/v1alpha1"
	nodeinfov1alpha1 "volcano.sh/apis/pkg/client/clientset/versioned/typed/nodeinfo/v1alpha1"
	schedulingv1beta1 "volcano.sh/apis/pkg/client/clientset/versioned/typed/scheduling/v1beta1"
	topologyv1alpha1 "volcano.sh/apis/pkg/client/clientset/versioned/typed/topology/v1alpha1"
	batchinformer "volcano.sh/apis/pkg/client/informers/externalversions"
)

var (
	alwaysReady = func() bool { return true }
)

type mockBatchClientset struct {
	batchversioned.Interface
}

func (m *mockBatchClientset) Discovery() discovery.DiscoveryInterface {
	return nil
}

func (m *mockBatchClientset) BatchV1alpha1() batchv1alpha1.BatchV1alpha1Interface {
	return nil
}

func (m *mockBatchClientset) BusV1alpha1() busv1alpha1.BusV1alpha1Interface {
	return nil
}

func (m *mockBatchClientset) FlowV1alpha1() flowv1alpha1.FlowV1alpha1Interface {
	return nil
}

func (m *mockBatchClientset) NodeinfoV1alpha1() nodeinfov1alpha1.NodeinfoV1alpha1Interface {
	return nil
}

func (m *mockBatchClientset) SchedulingV1beta1() schedulingv1beta1.SchedulingV1beta1Interface {
	return nil
}

func (m *mockBatchClientset) TopologyV1alpha1() topologyv1alpha1.TopologyV1alpha1Interface {
	return nil
}

type fixture struct {
	t *testing.T

	kubeClient    *kubefake.Clientset
	batchClient   batchversioned.Interface
	flowClient    *flowfake.Clientset
	dynamicClient *dynamicfake.FakeDynamicClient

	// Objects to put in the store.
	workTemplateLister []*v1alpha1flow.WorkTemplate

	// Objects from here preloaded into NewSimpleFake.
	kubeObjects    []runtime.Object
	flowObjects    []runtime.Object
	dynamicObjects []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.kubeObjects = []runtime.Object{}
	f.flowObjects = []runtime.Object{}
	f.dynamicObjects = []runtime.Object{}
	return f
}

func (f *fixture) newController() (*worktemplatecontroller, flowinformer.SharedInformerFactory) {
	f.kubeClient = kubefake.NewSimpleClientset(f.kubeObjects...)
	f.batchClient = &mockBatchClientset{}
	f.flowClient = flowfake.NewSimpleClientset(f.flowObjects...)

	scheme := runtime.NewScheme()
	f.dynamicClient = dynamicfake.NewSimpleDynamicClient(scheme, f.dynamicObjects...)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(f.kubeClient, 0)
	flowInformerFactory := flowinformer.NewSharedInformerFactory(f.flowClient, 0)
	batchInformerFactory := batchinformer.NewSharedInformerFactory(f.batchClient, 0)

	c := &worktemplatecontroller{
		kubeClient:         f.kubeClient,
		batchClient:        f.batchClient,
		flowClient:         f.flowClient,
		dynamicClient:      f.dynamicClient,
		recorder:           record.NewFakeRecorder(100),
		workTemplateLister: flowInformerFactory.Flow().V1alpha1().WorkTemplates().Lister(),
		workTemplateSynced: alwaysReady,
	}

	opt := &framework.ControllerOption{
		KubeClient:                 f.kubeClient,
		BatchClient:                f.batchClient,
		FlowClient:                 f.flowClient,
		SharedInformerFactory:      kubeInformerFactory,
		FlowSharedInformerFactory:  flowInformerFactory,
		BatchSharedInformerFactory: batchInformerFactory,
		DynamicClient:              f.dynamicClient,
		WorkerNum:                  1,
		MaxRequeueNum:              3,
	}
	c.Initialize(opt)

	for _, w := range f.workTemplateLister {
		flowInformerFactory.Flow().V1alpha1().WorkTemplates().Informer().GetIndexer().Add(w)
	}

	return c, flowInformerFactory
}

func (f *fixture) run(req apis.FlowRequest) {
	c, _ := f.newController()
	err := c.handleWorkTemplate(&req)
	if err != nil {
		f.t.Logf("handleWorkTemplate error: %v", err)
	}
}

func TestInitialize(t *testing.T) {
	f := newFixture(t)
	c, _ := f.newController()
	if c == nil {
		t.Fatal("expected controller to be not nil")
	}
	if c.workerNum != 1 {
		t.Errorf("expected workerNum 1, got %d", c.workerNum)
	}
	if len(c.queues) != 1 {
		t.Errorf("expected 1 queue, got %d", len(c.queues))
	}
}

func TestSyncHandler_NotFound(t *testing.T) {
	f := newFixture(t)
	f.run(apis.FlowRequest{
		Namespace:        "default",
		WorkTemplateName: "test-template",
		Action:           v1alpha1flow.SyncWorkTemplateAction,
	})
}

func TestSyncHandler_BasicFlow(t *testing.T) {
	f := newFixture(t)

	wt := &v1alpha1flow.WorkTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-wt",
			Namespace: "default",
		},
		Spec: v1alpha1flow.WorkTemplateSpec{
			GVR: metav1.GroupVersionResource{
				Group:    "batch.volcano.sh",
				Version:  "v1alpha1",
				Resource: "jobs",
			},
		},
	}
	f.workTemplateLister = append(f.workTemplateLister, wt)
	f.flowObjects = append(f.flowObjects, wt)

	// Add a job that should be picked up
	job := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "batch.volcano.sh/v1alpha1",
			"kind":       "Job",
			"metadata": map[string]interface{}{
				"name":      "job-1",
				"namespace": "default",
				"labels": map[string]interface{}{
					CreatedByWorkTemplate: GetTemplateString("default", "test-wt"),
				},
			},
		},
	}
	f.dynamicObjects = append(f.dynamicObjects, job)

	f.run(apis.FlowRequest{
		Namespace:        "default",
		WorkTemplateName: "test-wt",
		Action:           v1alpha1flow.SyncWorkTemplateAction,
	})

	// Verify status update
	updatedWT, _ := f.flowClient.FlowV1alpha1().WorkTemplates("default").Get(context.Background(), "test-wt", metav1.GetOptions{})
	if updatedWT == nil || len(updatedWT.Status.JobDependsOnList) != 1 {
		t.Errorf("expected 1 job in status, got %+v", updatedWT.Status.JobDependsOnList)
	}
}
