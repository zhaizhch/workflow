/*
Copyright 2024 The Volcano Authors.

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

package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	upstreamclientset "volcano.sh/apis/pkg/client/clientset/versioned"
	upstreaminformer "volcano.sh/apis/pkg/client/informers/externalversions"

	flowclientset "github.com/workflow.sh/work-flow/pkg/client/clientset/versioned"
	flowinformer "github.com/workflow.sh/work-flow/pkg/client/informers/externalversions"
	"github.com/workflow.sh/work-flow/pkg/controllers/framework"
	_ "github.com/workflow.sh/work-flow/pkg/controllers/workflow"
	_ "github.com/workflow.sh/work-flow/pkg/controllers/worktemplate"
	_ "github.com/workflow.sh/work-flow/pkg/webhooks/admission/validate"
	"github.com/workflow.sh/work-flow/pkg/webhooks/config"
	"github.com/workflow.sh/work-flow/pkg/webhooks/router"
)

var (
	masterURL        string
	kubeconfig       string
	workers          int
	port             int
	certFile         string
	keyFile          string
	enabledAdmission string
	enableController bool
)

func main() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.IntVar(&workers, "workers", 1, "The number of workers")
	flag.IntVar(&port, "port", 8443, "The port for webhooks")
	flag.StringVar(&certFile, "tls-cert-file", "", "The certificate file for webhooks")
	flag.StringVar(&keyFile, "tls-private-key-file", "", "The private key file for webhooks")
	flag.StringVar(&enabledAdmission, "enabled-admission", "workflow", "The webhooks to enable")
	flag.BoolVar(&enableController, "enable-controller", true, "whether to enable controllers")

	klog.InitFlags(nil)
	flag.Parse()

	// Build config
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	// Create clients
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes client: %s", err.Error())
	}

	upstreamClient, err := upstreamclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building volcano upstream client: %s", err.Error())
	}

	flowClient, err := flowclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building volcano flow client: %s", err.Error())
	}

	// Create informers
	// Use 30s resync for generic safety, or parse from flags
	resyncPeriod := 30 * time.Second

	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	upstreamInformerFactory := upstreaminformer.NewSharedInformerFactory(upstreamClient, resyncPeriod)
	flowInformerFactory := flowinformer.NewSharedInformerFactory(flowClient, resyncPeriod)

	// Create ControllerOption
	opt := &framework.ControllerOption{
		KubeClient:                kubeClient,
		VolcanoClient:             upstreamClient,
		FlowClient:                flowClient,
		SharedInformerFactory:     kubeInformerFactory,
		VCSharedInformerFactory:   upstreamInformerFactory,
		FlowSharedInformerFactory: flowInformerFactory,
		Config:                    cfg,
		WorkerNum:                 uint32(workers),
		MaxRequeueNum:             10, // Default value
	}

	// Webhook setup
	webhookConfig := &config.Config{
		EnabledAdmission: enabledAdmission,
	}
	admissionServiceConfig := &router.AdmissionServiceConfig{
		KubeClient:    kubeClient,
		VolcanoClient: flowClient,
		ConfigData:    nil, // Can be populated if needed
	}

	err = router.ForEachAdmission(webhookConfig, func(service *router.AdmissionService) error {
		service.Config = admissionServiceConfig
		http.HandleFunc(service.Path, service.Handler)
		return nil
	})
	if err != nil {
		klog.Fatalf("Failed to setup webhooks: %v", err)
	}

	if enableController {
		// Initialize and Run Controllers
		framework.ForeachController(func(c framework.Controller) {
			if err := c.Initialize(opt); err != nil {
				klog.Errorf("Failed to initialize controller %s: %v", c.Name(), err)
				return
			}

			go c.Run(make(chan struct{}))
			klog.Infof("Started controller %s", c.Name())
		})
	}

	// Start Informers
	stopCh := make(chan struct{})
	defer close(stopCh)

	kubeInformerFactory.Start(stopCh)
	upstreamInformerFactory.Start(stopCh)
	flowInformerFactory.Start(stopCh)

	// Start Webhook Server if certs are provided
	if certFile != "" && keyFile != "" {
		go func() {
			klog.Infof("Starting webhook server on port %d", port)
			if err := http.ListenAndServeTLS(fmt.Sprintf(":%d", port), certFile, keyFile, nil); err != nil {
				klog.Fatalf("Webhook server failed: %v", err)
			}
		}()
	}

	// Wait forever
	select {}
}
