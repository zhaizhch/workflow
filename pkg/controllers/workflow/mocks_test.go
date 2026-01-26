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
	"k8s.io/client-go/discovery"
	batchversioned "volcano.sh/apis/pkg/client/clientset/versioned"
	batchv1alpha1 "volcano.sh/apis/pkg/client/clientset/versioned/typed/batch/v1alpha1"
	busv1alpha1 "volcano.sh/apis/pkg/client/clientset/versioned/typed/bus/v1alpha1"
	flowv1alpha1 "volcano.sh/apis/pkg/client/clientset/versioned/typed/flow/v1alpha1"
	nodeinfov1alpha1 "volcano.sh/apis/pkg/client/clientset/versioned/typed/nodeinfo/v1alpha1"
	schedulingv1beta1 "volcano.sh/apis/pkg/client/clientset/versioned/typed/scheduling/v1beta1"
	topologyv1alpha1 "volcano.sh/apis/pkg/client/clientset/versioned/typed/topology/v1alpha1"
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
