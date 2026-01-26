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

package workflow

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
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
	batchinformer "volcano.sh/apis/pkg/client/informers/externalversions"
)

var (
	alwaysReady = func() bool { return true }
)

type fixture struct {
	t *testing.T

	kubeClient    *kubefake.Clientset
	batchClient   batchversioned.Interface
	flowClient    *flowfake.Clientset
	dynamicClient *dynamicfake.FakeDynamicClient

	// Objects to put in the store.
	workflowLister     []*v1alpha1flow.Workflow
	workTemplateLister []*v1alpha1flow.WorkTemplate

	// Objects from here preloaded into NewSimpleFake.
	kubeObjects    []runtime.Object
	batchObjects   []runtime.Object
	flowObjects    []runtime.Object
	dynamicObjects []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.kubeObjects = []runtime.Object{}
	f.batchObjects = []runtime.Object{}
	f.flowObjects = []runtime.Object{}
	f.dynamicObjects = []runtime.Object{}
	return f
}

func (f *fixture) newController() (*workflowcontroller, flowinformer.SharedInformerFactory, kubeinformers.SharedInformerFactory) {
	f.kubeClient = kubefake.NewSimpleClientset(f.kubeObjects...)
	f.batchClient = &mockBatchClientset{}
	f.flowClient = flowfake.NewSimpleClientset(f.flowObjects...)

	scheme := runtime.NewScheme()
	f.dynamicClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "batch.volcano.sh", Version: "v1alpha1", Resource: "jobs"}: "JobList",
	}, f.dynamicObjects...)

	// Mock Discovery Resources
	if fakeDiscovery, ok := f.kubeClient.Discovery().(*discoveryfake.FakeDiscovery); ok {
		fakeDiscovery.Resources = []*metav1.APIResourceList{
			{
				GroupVersion: "batch.volcano.sh/v1alpha1",
				APIResources: []metav1.APIResource{
					{
						Name:       "jobs",
						Kind:       "Job",
						Namespaced: true,
					},
				},
			},
		}
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(f.kubeClient, 0)
	flowInformerFactory := flowinformer.NewSharedInformerFactory(f.flowClient, 0)
	batchInformerFactory := batchinformer.NewSharedInformerFactory(f.batchClient, 0)

	c := &workflowcontroller{
		kubeClient:         f.kubeClient,
		batchClient:        f.batchClient,
		flowClient:         f.flowClient,
		dynamicClient:      f.dynamicClient,
		recorder:           &record.FakeRecorder{},
		workflowLister:     flowInformerFactory.Flow().V1alpha1().Workflows().Lister(),
		workTemplateLister: flowInformerFactory.Flow().V1alpha1().WorkTemplates().Lister(),
		workflowSynced:     alwaysReady,
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

	for _, w := range f.workflowLister {
		flowInformerFactory.Flow().V1alpha1().Workflows().Informer().GetIndexer().Add(w)
	}
	for _, w := range f.workTemplateLister {
		flowInformerFactory.Flow().V1alpha1().WorkTemplates().Informer().GetIndexer().Add(w)
	}

	return c, flowInformerFactory, kubeInformerFactory
}

func (f *fixture) run(req apis.FlowRequest) {
	c, _, _ := f.newController()
	err := c.handleWorkflow(&req)

	if err != nil {
		f.t.Logf("handleWorkflow error: %v", err)
	}
}

func TestInitialize(t *testing.T) {
	f := newFixture(t)
	c, _, _ := f.newController()
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
		Namespace:    "default",
		WorkflowName: "test-workflow",
		Action:       v1alpha1flow.SyncWorkflowAction,
	})
}

func TestSyncHandler_BasicFlow(t *testing.T) {
	f := newFixture(t)

	workflow := &v1alpha1flow.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workflow",
			Namespace: "default",
		},
		Spec: v1alpha1flow.WorkflowSpec{
			Flows: []v1alpha1flow.Flow{
				{
					Name: "task1",
				},
			},
		},
		Status: v1alpha1flow.WorkflowStatus{
			State: v1alpha1flow.State{
				Phase: v1alpha1flow.Running,
			},
		},
	}

	workTemplate := &v1alpha1flow.WorkTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "task1",
			Namespace: "default",
		},
		Spec: v1alpha1flow.WorkTemplateSpec{
			GVR: metav1.GroupVersionResource{
				Group:    "batch.volcano.sh",
				Version:  "v1alpha1",
				Resource: "jobs",
			},
			JobSpec: runtime.RawExtension{
				Raw: []byte(`{"template":{"spec":{"containers":[{"name":"nginx"}]}}}`),
			},
		},
	}

	f.workflowLister = append(f.workflowLister, workflow)
	f.workTemplateLister = append(f.workTemplateLister, workTemplate)
	f.flowObjects = append(f.flowObjects, workflow, workTemplate)

	c, _, _ := f.newController()

	req := apis.FlowRequest{
		Namespace:    "default",
		WorkflowName: "test-workflow",
		Action:       v1alpha1flow.SyncWorkflowAction,
	}

	err := c.syncHandler(&req)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	actions := f.dynamicClient.Actions()
	_ = actions
}

func TestProcessNextWorkItem(t *testing.T) {
	f := newFixture(t)
	c, _, _ := f.newController()

	req := apis.FlowRequest{
		Namespace:    "default",
		WorkflowName: "test",
		Action:       v1alpha1flow.SyncWorkflowAction,
	}

	c.queues[0].Add(req)

	res := c.processNextWorkItem(0)
	if !res {
		t.Error("expected processNextWorkItem to return true")
	}
}

func TestHandleWorkflowErr(t *testing.T) {
	f := newFixture(t)
	c, _, _ := f.newController()
	queue := c.queues[0]

	req := apis.FlowRequest{Namespace: "default", WorkflowName: "test-wf"}

	t.Run("No error", func(t *testing.T) {
		c.handleWorkflowErr(nil, req, queue)
		if queue.NumRequeues(req) != 0 {
			t.Errorf("expected 0 requeues, got %d", queue.NumRequeues(req))
		}
	})

	t.Run("Error with requeue", func(t *testing.T) {
		c.maxRequeueNum = 5
		c.handleWorkflowErr(fmt.Errorf("test error"), req, queue)
		// RateLimited queue add is async/internal
	})

	t.Run("Max requeue exceeded", func(t *testing.T) {
		c.maxRequeueNum = 1
		workflow := &v1alpha1flow.Workflow{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wf", Namespace: "default"},
		}
		f.flowObjects = append(f.flowObjects, workflow)
		c, _, _ = f.newController()
		queue = c.queues[0]

		queue.AddRateLimited(req)
		c.handleWorkflowErr(fmt.Errorf("terminal error"), req, queue)
	})
}

func TestHandleRetry(t *testing.T) {
	f := newFixture(t)

	workflow := &v1alpha1flow.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "wf", Namespace: "default"},
		Status: v1alpha1flow.WorkflowStatus{
			JobStatusList: []v1alpha1flow.JobStatus{
				{Name: "job1", State: "Failed", RestartCount: 0},
			},
		},
	}
	f.flowObjects = append(f.flowObjects, workflow)

	flow := v1alpha1flow.Flow{
		Name: "task1",
		Retry: &v1alpha1flow.Retry{
			MaxRetries: 3,
		},
	}

	job := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "batch.volcano.sh/v1alpha1",
			"kind":       "Job",
			"metadata": map[string]interface{}{
				"name":      "job1",
				"namespace": "default",
			},
			"status": map[string]interface{}{
				"state": map[string]interface{}{
					"phase": "Failed",
				},
			},
		},
	}
	f.dynamicObjects = append(f.dynamicObjects, job)

	c, _, _ := f.newController()

	gvr := schema.GroupVersionResource{Group: "batch.volcano.sh", Version: "v1alpha1", Resource: "jobs"}

	t.Run("Should retry", func(t *testing.T) {
		c.handleRetry(workflow, flow, job, gvr, "job1")
		// Check for delete action
		foundDelete := false
		for _, action := range f.dynamicClient.Actions() {
			if action.GetVerb() == "delete" && action.GetResource().Resource == "jobs" {
				foundDelete = true
				break
			}
		}
		if !foundDelete {
			t.Error("expected job to be deleted for retry")
		}
	})

	t.Run("Max retries exceeded", func(t *testing.T) {
		f.dynamicClient.ClearActions()
		workflow.Status.JobStatusList[0].RestartCount = 3
		c.handleRetry(workflow, flow, job, gvr, "job1")
		if len(f.dynamicClient.Actions()) > 0 {
			t.Error("did not expect any actions when nested retries exceeded")
		}
	})
}

func TestRecordEventsForWorkflow(t *testing.T) {
	f := newFixture(t)
	workflow := &v1alpha1flow.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "test-wf", Namespace: "default"},
	}
	f.flowObjects = append(f.flowObjects, workflow)
	c, _, _ := f.newController()

	c.recordEventsForWorkflow("default", "test-wf", "Normal", "TestReason", "TestMessage")
}
