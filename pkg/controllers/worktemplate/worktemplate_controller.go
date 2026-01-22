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

package worktemplate

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	upstreamclientset "volcano.sh/apis/pkg/client/clientset/versioned"
	upstreaminformer "volcano.sh/apis/pkg/client/informers/externalversions"
	batchinformer "volcano.sh/apis/pkg/client/informers/externalversions/batch/v1alpha1"

	flowclientset "github.com/workflow.sh/work-flow/pkg/client/clientset/versioned"
	versionedscheme "github.com/workflow.sh/work-flow/pkg/client/clientset/versioned/scheme"
	flowinformer "github.com/workflow.sh/work-flow/pkg/client/informers/externalversions"
	flowinformerV1alpha1 "github.com/workflow.sh/work-flow/pkg/client/informers/externalversions/flow/v1alpha1"

	flowlister "github.com/workflow.sh/work-flow/pkg/client/listers/flow/v1alpha1"
	batchlister "volcano.sh/apis/pkg/client/listers/batch/v1alpha1"

	"github.com/workflow.sh/work-flow/pkg/controllers/apis"
	"github.com/workflow.sh/work-flow/pkg/controllers/framework"
)

func init() {
	framework.RegisterController(&worktemplatecontroller{})
}

// worktemplatecontroller the WorkTemplate worktemplatecontroller type.
type worktemplatecontroller struct {
	kubeClient kubernetes.Interface
	vcClient   upstreamclientset.Interface
	flowClient flowclientset.Interface

	//informer
	jobTemplateInformer flowinformerV1alpha1.WorkTemplateInformer
	jobInformer         batchinformer.JobInformer

	dynamicClient dynamic.Interface

	//InformerFactory
	vcInformerFactory   upstreaminformer.SharedInformerFactory
	flowInformerFactory flowinformer.SharedInformerFactory

	//jobTemplateLister
	jobTemplateLister flowlister.WorkTemplateLister
	jobTemplateSynced cache.InformerSynced

	//jobLister
	jobLister batchlister.JobLister
	jobSynced cache.InformerSynced

	// WorkTemplate Event recorder
	recorder record.EventRecorder

	queue               workqueue.TypedRateLimitingInterface[apis.FlowRequest]
	enqueueWorkTemplate func(req apis.FlowRequest)

	syncHandler func(req *apis.FlowRequest) error

	maxRequeueNum int

	Config *rest.Config
}

func (jt *worktemplatecontroller) Name() string {
	return "worktemplate-controller"
}

func (jt *worktemplatecontroller) Initialize(opt *framework.ControllerOption) error {
	jt.kubeClient = opt.KubeClient
	jt.vcClient = opt.VolcanoClient
	jt.flowClient = opt.FlowClient
	jt.Config = opt.Config
	jt.dynamicClient = opt.DynamicClient

	if jt.dynamicClient == nil && jt.Config != nil {
		dynamicClient, err := dynamic.NewForConfig(jt.Config)
		if err != nil {
			return err
		}
		jt.dynamicClient = dynamicClient
	}

	jt.vcInformerFactory = opt.VCSharedInformerFactory
	jt.flowInformerFactory = opt.FlowSharedInformerFactory

	jt.jobTemplateInformer = jt.flowInformerFactory.Flow().V1alpha1().WorkTemplates()
	jt.jobTemplateSynced = jt.jobTemplateInformer.Informer().HasSynced
	jt.jobTemplateLister = jt.jobTemplateInformer.Lister()
	jt.jobTemplateInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: jt.addWorkTemplate,
	})

	jt.jobInformer = jt.vcInformerFactory.Batch().V1alpha1().Jobs()
	jt.jobSynced = jt.jobInformer.Informer().HasSynced
	jt.jobLister = jt.jobInformer.Lister()
	jt.jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: jt.addJob,
	})

	jt.maxRequeueNum = opt.MaxRequeueNum
	if jt.maxRequeueNum < 0 {
		jt.maxRequeueNum = -1
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: jt.kubeClient.CoreV1().Events("")})

	jt.recorder = eventBroadcaster.NewRecorder(versionedscheme.Scheme, v1.EventSource{Component: "vc-controller-manager"})
	jt.queue = workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[apis.FlowRequest]())

	jt.enqueueWorkTemplate = jt.enqueue

	jt.syncHandler = jt.handleWorkTemplate

	return nil
}

func (jt *worktemplatecontroller) Run(stopCh <-chan struct{}) {
	defer jt.queue.ShutDown()

	jt.vcInformerFactory.Start(stopCh)
	jt.flowInformerFactory.Start(stopCh)

	for informerType, ok := range jt.vcInformerFactory.WaitForCacheSync(stopCh) {
		if !ok {
			klog.Errorf("caches failed to sync: %v", informerType)
			return
		}
	}

	for informerType, ok := range jt.flowInformerFactory.WaitForCacheSync(stopCh) {
		if !ok {
			klog.Errorf("caches failed to sync: %v", informerType)
			return
		}
	}

	go wait.Until(jt.worker, time.Second, stopCh)

	klog.Infof("WorkTemplateController is running ...... ")

	<-stopCh
}

func (jt *worktemplatecontroller) worker() {
	for jt.processNextWorkItem() {
	}
}

func (jt *worktemplatecontroller) processNextWorkItem() bool {
	req, shutdown := jt.queue.Get()
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
	defer jt.queue.Done(req)

	err := jt.syncHandler(&req)
	jt.handleWorkTemplateErr(err, req)

	return true
}

func (jt *worktemplatecontroller) handleWorkTemplate(req *apis.FlowRequest) error {
	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing jobTemplate %s (%v).", req.WorkTemplateName, time.Since(startTime))
	}()

	jobTemplate, err := jt.jobTemplateLister.WorkTemplates(req.Namespace).Get(req.WorkTemplateName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("WorkTemplate %s has been deleted.", req.WorkTemplateName)
			return nil
		}

		return fmt.Errorf("get jobTemplate %s failed for %v", req.WorkflowName, err)
	}

	klog.V(4).Infof("Begin syncWorkTemplate for jobTemplate %s", req.WorkflowName)
	if err := jt.syncWorkTemplate(jobTemplate); err != nil {
		return fmt.Errorf("sync jobTemplate %s failed for %v, event is %v, action is %s",
			req.WorkflowName, err, req.Event, req.Action)
	}

	return nil
}

func (jt *worktemplatecontroller) handleWorkTemplateErr(err error, req apis.FlowRequest) {
	if err == nil {
		jt.queue.Forget(req)
		return
	}

	if jt.maxRequeueNum == -1 || jt.queue.NumRequeues(req) < jt.maxRequeueNum {
		klog.V(4).Infof("Error syncing jobTemplate request %v for %v.", req, err)
		jt.queue.AddRateLimited(req)
		return
	}

	jt.recordEventsForWorkTemplate(req.Namespace, req.WorkTemplateName, v1.EventTypeWarning, string(req.Action),
		fmt.Sprintf("%v WorkTemplate failed for %v", req.Action, err))
	klog.V(2).Infof("Dropping WorkTemplate request %v out of the queue for %v.", req, err)
	jt.queue.Forget(req)
}

func (jt *worktemplatecontroller) recordEventsForWorkTemplate(namespace, name, eventType, reason, message string) {
	jobTemplate, err := jt.jobTemplateLister.WorkTemplates(namespace).Get(name)
	if err != nil {
		klog.Errorf("Get WorkTemplate %s failed for %v.", name, err)
		return
	}

	jt.recorder.Event(jobTemplate, eventType, reason, message)
}
