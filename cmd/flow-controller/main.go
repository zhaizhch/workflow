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

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"

	batchclientset "volcano.sh/apis/pkg/client/clientset/versioned"
	batchinformer "volcano.sh/apis/pkg/client/informers/externalversions"

	"github.com/google/uuid"
	flowclientset "github.com/workflow.sh/work-flow/pkg/client/clientset/versioned"
	flowinformer "github.com/workflow.sh/work-flow/pkg/client/informers/externalversions"
	"github.com/workflow.sh/work-flow/pkg/controllers/framework"
	_ "github.com/workflow.sh/work-flow/pkg/controllers/workflow"
	_ "github.com/workflow.sh/work-flow/pkg/controllers/worktemplate"
	_ "github.com/workflow.sh/work-flow/pkg/webhooks/admission/mutate"
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

	// Leader election flags
	var leaderElect bool
	var leaseDuration time.Duration
	var renewDeadline time.Duration
	var retryPeriod time.Duration
	var resourceLock string
	var leaderID string

	flag.BoolVar(&leaderElect, "leader-elect", false, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	flag.DurationVar(&leaseDuration, "leader-elect-lease-duration", 15*time.Second, "The duration that non-leader candidates will wait after observing a leadership renewal until attempting to acquire leadership of a led but unrenewed leader slot. This is effectively the maximum duration that a leader can be stopped before it is replaced by another candidate. This is only applicable if leader election is enabled.")
	flag.DurationVar(&renewDeadline, "leader-elect-renew-deadline", 10*time.Second, "The interval between attempts by the acting master to renew a leadership slot before it stops leading. This must be less than or equal to the lease duration. This is only applicable if leader election is enabled.")
	flag.DurationVar(&retryPeriod, "leader-elect-retry-period", 2*time.Second, "The duration the clients should wait between attempting acquisition and renewal of a leadership. This is only applicable if leader election is enabled.")
	flag.StringVar(&resourceLock, "leader-elect-resource-lock", "leases", "The type of resource object that is used for locking during the leader election. Supported options are 'leases', 'endpointsleases' and 'configmapsleases'.")
	flag.StringVar(&leaderID, "leader-elect-id", "workflow-controller", "The name of the resource lock.")

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

	batchClient, err := batchclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building batch client: %s", err.Error())
	}

	flowClient, err := flowclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building workflow client: %s", err.Error())
	}

	// Create informers
	// Use 30s resync for generic safety, or parse from flags
	resyncPeriod := 30 * time.Second

	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	batchInformerFactory := batchinformer.NewSharedInformerFactory(batchClient, resyncPeriod)
	flowInformerFactory := flowinformer.NewSharedInformerFactory(flowClient, resyncPeriod)

	// Create ControllerOption
	opt := &framework.ControllerOption{
		KubeClient:                 kubeClient,
		BatchClient:                batchClient,
		FlowClient:                 flowClient,
		SharedInformerFactory:      kubeInformerFactory,
		BatchSharedInformerFactory: batchInformerFactory,
		FlowSharedInformerFactory:  flowInformerFactory,
		Config:                     cfg,
		WorkerNum:                  uint32(workers),
		MaxRequeueNum:              -1, // Default value
	}

	// Webhook setup
	webhookConfig := &config.Config{
		EnabledAdmission: enabledAdmission,
	}
	admissionServiceConfig := &router.AdmissionServiceConfig{
		KubeClient:     kubeClient,
		WorkflowClient: flowClient,
		ConfigData:     nil, // Can be populated if needed
	}

	err = router.ForEachAdmission(webhookConfig, func(service *router.AdmissionService) error {
		service.Config = admissionServiceConfig
		http.HandleFunc(service.Path, service.Handler)
		return nil
	})
	if err != nil {
		klog.Fatalf("Failed to setup webhooks: %v", err)
	}

	// Start Informers and Controllers
	run := func(ctx context.Context) {
		stopCh := ctx.Done()

		kubeInformerFactory.Start(stopCh)
		batchInformerFactory.Start(stopCh)
		flowInformerFactory.Start(stopCh)

		if enableController {
			// Initialize and Run Controllers
			framework.ForeachController(func(c framework.Controller) {
				if err := c.Initialize(opt); err != nil {
					klog.Errorf("Failed to initialize controller %s: %v", c.Name(), err)
					return
				}

				go c.Run(stopCh)
				klog.Infof("Started controller %s", c.Name())
			})
		}

		// Start Webhook Server if certs are provided
		if certFile != "" && keyFile != "" {

			go func() {
				klog.Infof("Starting webhook server on port %d", port)

				// Start HTTPS server for webhooks
				if err := http.ListenAndServeTLS(fmt.Sprintf(":%d", port), certFile, keyFile, nil); err != nil {
					klog.Fatalf("Webhook server failed: %v", err)
				}
			}()
		}

		// Wait forever
		<-stopCh
	}

	if !leaderElect {
		run(context.TODO())
	} else {
		id, err := os.Hostname()
		if err != nil {
			klog.Fatalf("Unable to get hostname: %v", err)
		}
		uuidVal, _ := uuid.NewUUID()
		id = id + "_" + uuidVal.String()

		lock, err := resourcelock.New(
			resourceLock,
			"default", // Lock in default namespace or make configurable
			leaderID,
			kubeClient.CoreV1(),
			kubeClient.CoordinationV1(),
			resourcelock.ResourceLockConfig{
				Identity: id,
			},
		)
		if err != nil {
			klog.Fatalf("Unable to create resource lock: %v", err)
		}

		leaderelection.RunOrDie(context.TODO(), leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   leaseDuration,
			RenewDeadline:   renewDeadline,
			RetryPeriod:     retryPeriod,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(ctx context.Context) {
					run(ctx)
				},
				OnStoppedLeading: func() {
					klog.Fatalf("leader election lost")
				},
				OnNewLeader: func(identity string) {
					if identity == id {
						return
					}
					klog.Infof("new leader elected: %s", identity)
				},
			},
		})
	}
}
