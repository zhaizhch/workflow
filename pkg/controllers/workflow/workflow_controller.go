/*
Copyright 2022 The Volcano Authors.

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
	upstreamclientset "volcano.sh/apis/pkg/client/clientset/versioned"

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
	vcClient      upstreamclientset.Interface
	flowClient    flowclientset.Interface
	dynamicClient dynamic.Interface

	//informer
	jobFlowInformer     flowinformerV1alpha1.WorkflowInformer
	jobTemplateInformer flowinformerV1alpha1.WorkTemplateInformer

	flowInformerFactory    flowinformer.SharedInformerFactory
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory

	//jobFlowLister
	jobFlowLister flowlister.WorkflowLister
	jobFlowSynced cache.InformerSynced

	//jobTemplateLister
	jobTemplateLister flowlister.WorkTemplateLister
	jobTemplateSynced cache.InformerSynced

	genericJobSynced []cache.InformerSynced

	// Workflow Event recorder
	recorder record.EventRecorder

	queue           workqueue.TypedRateLimitingInterface[apis.FlowRequest]
	enqueueWorkflow func(req apis.FlowRequest)

	syncHandler func(req *apis.FlowRequest) error

	maxRequeueNum int
	Config        *rest.Config
}

func (jf *workflowcontroller) Name() string {
	return "workflow-controller"
}

func (jf *workflowcontroller) Initialize(opt *framework.ControllerOption) error {
	jf.kubeClient = opt.KubeClient
	workload.SetClient(opt.KubeClient)
	jf.Config = opt.Config
	jf.vcClient = opt.VolcanoClient
	jf.flowClient = opt.FlowClient
	jf.dynamicClient = opt.DynamicClient

	if jf.dynamicClient == nil && jf.Config != nil {
		dynamicClient, err := dynamic.NewForConfig(jf.Config)
		if err != nil {
			return err
		}
		jf.dynamicClient = dynamicClient
	}

	jf.flowInformerFactory = opt.FlowSharedInformerFactory

	jf.jobFlowInformer = jf.flowInformerFactory.Flow().V1alpha1().Workflows()
	jf.jobFlowSynced = jf.jobFlowInformer.Informer().HasSynced
	jf.jobFlowLister = jf.jobFlowInformer.Lister()
	jf.jobFlowInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    jf.addWorkflow,
		UpdateFunc: jf.updateWorkflow,
	})

	jf.jobTemplateInformer = jf.flowInformerFactory.Flow().V1alpha1().WorkTemplates()
	jf.jobTemplateSynced = jf.jobTemplateInformer.Informer().HasSynced
	jf.jobTemplateLister = jf.jobTemplateInformer.Lister()

	// Initialize dynamic informer factory
	jf.dynamicInformerFactory = dynamicinformer.NewDynamicSharedInformerFactory(jf.dynamicClient, 0)

	// Register informers for all supported workloads
	for _, wl := range workload.GetAllWorkloads() {
		for _, gvr := range wl.GetGVR() {
			informer := jf.dynamicInformerFactory.ForResource(gvr)
			informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
				UpdateFunc: jf.updateGenericJob,
			})
			jf.genericJobSynced = append(jf.genericJobSynced, informer.Informer().HasSynced)
		}
	}

	jf.maxRequeueNum = opt.MaxRequeueNum
	if jf.maxRequeueNum < 0 {
		jf.maxRequeueNum = -1
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: jf.kubeClient.CoreV1().Events("")})

	jf.recorder = eventBroadcaster.NewRecorder(versionedscheme.Scheme, v1.EventSource{Component: "vc-controller-manager"})
	jf.queue = workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[apis.FlowRequest]())

	jf.enqueueWorkflow = jf.enqueue

	jf.syncHandler = jf.handleWorkflow

	workflowstate.SyncWorkflow = jf.syncWorkflow
	return nil
}

func (jf *workflowcontroller) Run(stopCh <-chan struct{}) {
	defer jf.queue.ShutDown()

	jf.flowInformerFactory.Start(stopCh)
	jf.dynamicInformerFactory.Start(stopCh)

	for informerType, ok := range jf.flowInformerFactory.WaitForCacheSync(stopCh) {
		if !ok {
			klog.Errorf("caches failed to sync: %v", informerType)
			return
		}
	}

	if !cache.WaitForCacheSync(stopCh, jf.genericJobSynced...) {
		klog.Errorf("generic job caches failed to sync")
		return
	}

	for informerType, ok := range jf.dynamicInformerFactory.WaitForCacheSync(stopCh) {
		if !ok {
			klog.Errorf("dynamic dynamic caches failed to sync: %v", informerType)
			return
		}
	}

	go wait.Until(jf.worker, time.Second, stopCh)

	klog.Infof("WorkflowController is running ...... ")

	<-stopCh
}

func (jf *workflowcontroller) worker() {
	for jf.processNextWorkItem() {
	}
}

func (jf *workflowcontroller) processNextWorkItem() bool {
	req, shutdown := jf.queue.Get()
	if shutdown {
		// Stop working
		return false
	}

	// We call Done here so the workqueue knows we have finished
	// processing this item. We also must remember to call Forget if we
	// do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the workqueue and attempted again after a back-off
	// period.
	defer jf.queue.Done(req)

	err := jf.syncHandler(&req)
	jf.handleWorkflowErr(err, req)

	return true
}

func (jf *workflowcontroller) handleWorkflow(req *apis.FlowRequest) error {
	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing workflow %s (%v).", req.WorkflowName, time.Since(startTime))
	}()

	workflow, err := jf.jobFlowLister.Workflows(req.Namespace).Get(req.WorkflowName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("Workflow %s has been deleted.", req.WorkflowName)
			return nil
		}

		return fmt.Errorf("get workflow %s failed for %v", req.WorkflowName, err)
	}

	jobFlowState := workflowstate.NewState(workflow)
	if jobFlowState == nil {
		return fmt.Errorf("workflow %s state %s is invalid", workflow.Name, workflow.Status.State)
	}

	klog.V(4).Infof("Begin execute %s action for workflow %s", req.Action, req.WorkflowName)
	if err := jobFlowState.Execute(req.Action); err != nil {
		return fmt.Errorf("sync workflow %s failed for %v, event is %v, action is %s",
			req.WorkflowName, err, req.Event, req.Action)
	}

	return nil
}

func (jf *workflowcontroller) handleWorkflowErr(err error, req apis.FlowRequest) {
	if err == nil {
		jf.queue.Forget(req)
		return
	}

	if jf.maxRequeueNum == -1 || jf.queue.NumRequeues(req) < jf.maxRequeueNum {
		klog.V(4).Infof("Error syncing jobFlow request %v for %v.", req, err)
		jf.queue.AddRateLimited(req)
		return
	}

	jf.recordEventsForWorkflow(req.Namespace, req.WorkflowName, v1.EventTypeWarning, string(req.Action),
		fmt.Sprintf("%v Workflow failed for %v", req.Action, err))
	klog.V(4).Infof("Dropping Workflow request %v out of the queue for %v.", req, err)
	jf.queue.Forget(req)
}

func (jf *workflowcontroller) recordEventsForWorkflow(namespace, name, eventType, reason, message string) {
	jobFlow, err := jf.jobFlowLister.Workflows(namespace).Get(name)
	if err != nil {
		klog.Errorf("Get Workflow %s failed for %v.", name, err)
		return
	}

	jf.recorder.Event(jobFlow, eventType, reason, message)
}
