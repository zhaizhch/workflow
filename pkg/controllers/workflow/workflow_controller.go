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
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	flowclientset "github.com/workflow.sh/work-flow/pkg/client/clientset/versioned"
	versionedscheme "github.com/workflow.sh/work-flow/pkg/client/clientset/versioned/scheme"
	flowinformer "github.com/workflow.sh/work-flow/pkg/client/informers/externalversions"
	flowinformerV1alpha1 "github.com/workflow.sh/work-flow/pkg/client/informers/externalversions/flow/v1alpha1"
	flowlister "github.com/workflow.sh/work-flow/pkg/client/listers/flow/v1alpha1"
	batchclientset "volcano.sh/apis/pkg/client/clientset/versioned"

	"github.com/workflow.sh/work-flow/pkg/controllers/apis"
	"github.com/workflow.sh/work-flow/pkg/controllers/framework"
	workflowstate "github.com/workflow.sh/work-flow/pkg/controllers/workflow/state"
	"github.com/workflow.sh/work-flow/pkg/controllers/workload"
)

func init() {
	framework.RegisterController(&workflowcontroller{})
}

// workflowcontroller the Workflow workflowcontroller type.
type workflowcontroller struct {
	kubeClient    kubernetes.Interface
	batchClient   batchclientset.Interface
	flowClient    flowclientset.Interface
	dynamicClient dynamic.Interface

	//informer
	workflowInformer     flowinformerV1alpha1.WorkflowInformer
	workTemplateInformer flowinformerV1alpha1.WorkTemplateInformer

	flowInformerFactory    flowinformer.SharedInformerFactory
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory

	//lister
	workflowLister flowlister.WorkflowLister
	workflowSynced cache.InformerSynced

	//workTemplateLister
	workTemplateLister flowlister.WorkTemplateLister
	workTemplateSynced cache.InformerSynced

	genericJobSynced []cache.InformerSynced

	// record events
	recorder record.EventRecorder

	queue           workqueue.TypedRateLimitingInterface[apis.FlowRequest]
	enqueueWorkflow func(req apis.FlowRequest)

	syncHandler func(req *apis.FlowRequest) error

	maxRequeueNum int
	Config        *rest.Config
}

func (wc *workflowcontroller) Name() string {
	return "workflow-controller"
}

func (wc *workflowcontroller) Initialize(opt *framework.ControllerOption) error {
	wc.kubeClient = opt.KubeClient
	workload.SetClient(opt.KubeClient)
	wc.Config = opt.Config
	wc.batchClient = opt.BatchClient
	wc.flowClient = opt.FlowClient
	wc.dynamicClient = opt.DynamicClient

	if wc.dynamicClient == nil && wc.Config != nil {
		dynamicClient, err := dynamic.NewForConfig(wc.Config)
		if err != nil {
			return err
		}
		wc.dynamicClient = dynamicClient
	}

	wc.flowInformerFactory = opt.FlowSharedInformerFactory

	wc.workflowInformer = wc.flowInformerFactory.Flow().V1alpha1().Workflows()
	wc.workflowSynced = wc.workflowInformer.Informer().HasSynced
	wc.workflowLister = wc.workflowInformer.Lister()
	wc.workflowInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    wc.addWorkflow,
		UpdateFunc: wc.updateWorkflow,
	})

	wc.workTemplateInformer = wc.flowInformerFactory.Flow().V1alpha1().WorkTemplates()
	wc.workTemplateSynced = wc.workTemplateInformer.Informer().HasSynced
	wc.workTemplateLister = wc.workTemplateInformer.Lister()

	// Initialize dynamic informer factory
	wc.dynamicInformerFactory = dynamicinformer.NewDynamicSharedInformerFactory(wc.dynamicClient, 0)

	// Register informers for all supported workloads
	for _, wl := range workload.GetAllWorkloads() {
		for _, gvr := range wl.GetGVR() {
			informer := wc.dynamicInformerFactory.ForResource(gvr)
			informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
				UpdateFunc: wc.updateGenericJob,
			})
			wc.genericJobSynced = append(wc.genericJobSynced, informer.Informer().HasSynced)
		}
	}

	wc.maxRequeueNum = opt.MaxRequeueNum
	if wc.maxRequeueNum < 0 {
		wc.maxRequeueNum = -1
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: wc.kubeClient.CoreV1().Events("")})

	wc.recorder = eventBroadcaster.NewRecorder(versionedscheme.Scheme, v1.EventSource{Component: "workflow-controller"})
	wc.queue = workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[apis.FlowRequest]())

	wc.enqueueWorkflow = wc.enqueue

	wc.syncHandler = wc.handleWorkflow

	workflowstate.SyncWorkflow = wc.syncWorkflow
	return nil
}

func (wc *workflowcontroller) Run(stopCh <-chan struct{}) {
	defer wc.queue.ShutDown()

	wc.flowInformerFactory.Start(stopCh)
	wc.dynamicInformerFactory.Start(stopCh)

	for informerType, ok := range wc.flowInformerFactory.WaitForCacheSync(stopCh) {
		if !ok {
			klog.Errorf("caches failed to sync: %v", informerType)
			return
		}
	}

	if !cache.WaitForCacheSync(stopCh, wc.genericJobSynced...) {
		klog.Errorf("generic job caches failed to sync")
		return
	}

	for informerType, ok := range wc.dynamicInformerFactory.WaitForCacheSync(stopCh) {
		if !ok {
			klog.Errorf("dynamic caches failed to sync: %v", informerType)
			return
		}
	}

	go wait.Until(wc.worker, time.Second, stopCh)

	klog.Infof("WorkflowController is running ...... ")

	<-stopCh
}

func (wc *workflowcontroller) worker() {
	for wc.processNextWorkItem() {
	}
}

func (wc *workflowcontroller) processNextWorkItem() bool {
	req, shutdown := wc.queue.Get()
	if shutdown {
		return false
	}

	defer wc.queue.Done(req)

	err := wc.syncHandler(&req)
	wc.handleWorkflowErr(err, req)

	return true
}

func (wc *workflowcontroller) handleWorkflow(req *apis.FlowRequest) error {
	workflow, err := wc.workflowLister.Workflows(req.Namespace).Get(req.WorkflowName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get workflow %s: %v", req.WorkflowName, err)
	}

	wfState := workflowstate.NewState(workflow)
	if wfState == nil {
		return fmt.Errorf("workflow %s state %v is invalid", workflow.Name, workflow.Status.State)
	}

	if err := wfState.Execute(req.Action); err != nil {
		return fmt.Errorf("failed to execute %s for workflow %s: %v", req.Action, req.WorkflowName, err)
	}

	return nil
}

func (wc *workflowcontroller) handleWorkflowErr(err error, req apis.FlowRequest) {
	if err == nil {
		wc.queue.Forget(req)
		return
	}

	if wc.maxRequeueNum == -1 || wc.queue.NumRequeues(req) < wc.maxRequeueNum {
		wc.queue.AddRateLimited(req)
		return
	}

	wc.recordEventsForWorkflow(req.Namespace, req.WorkflowName, v1.EventTypeWarning, string(req.Action),
		fmt.Sprintf("%v failed for %v", req.Action, err))
	wc.queue.Forget(req)
}

func (wc *workflowcontroller) recordEventsForWorkflow(namespace, name, eventType, reason, message string) {
	workflow, err := wc.workflowLister.Workflows(namespace).Get(name)
	if err != nil {
		return
	}
	wc.recorder.Event(workflow, eventType, reason, message)
}
