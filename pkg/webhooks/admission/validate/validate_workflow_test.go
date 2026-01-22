/*
Copyright 2025 The Volcano Authors.

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

package validate

import (
	"fmt"
	"strings"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	flowv1alpha1 "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	fakeclient "github.com/workflow.sh/work-flow/pkg/client/clientset/versioned/fake"
	informers "github.com/workflow.sh/work-flow/pkg/client/informers/externalversions"
)

func TestValidateWorkflowCreate(t *testing.T) {
	namespace := "test"
	testCases := []struct {
		Name           string
		Workflow       flowv1alpha1.Workflow
		ExpectErr      bool
		reviewResponse admissionv1.AdmissionResponse
		ret            string
	}{
		{
			Name: "validate valid-workflow",
			Workflow: flowv1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-workflow",
					Namespace: namespace,
				},
				Spec: flowv1alpha1.WorkflowSpec{
					Flows: []flowv1alpha1.Flow{
						{
							Name: "a",
						},
						{
							Name: "b",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"a"},
							},
						},
						{
							Name: "c",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"a", "b"},
							},
						},
						{
							Name: "d",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"b"},
							},
						},
						{
							Name: "e",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"c", "d"},
							},
						},
					},
					JobRetainPolicy: flowv1alpha1.RetainPolicy("retain"),
				},
			},
			reviewResponse: admissionv1.AdmissionResponse{Allowed: true},
			ret:            "",
			ExpectErr:      false,
		},
		// duplicate flow name
		{
			Name: "validate valid-workflow with duplicate flow name",
			Workflow: flowv1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-workflow",
					Namespace: namespace,
				},
				Spec: flowv1alpha1.WorkflowSpec{
					Flows: []flowv1alpha1.Flow{
						{
							Name: "a",
						},
						{
							Name: "a",
						},
						{
							Name: "b",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"a"},
							},
						},
						{
							Name: "c",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"a", "b"},
							},
						},
						{
							Name: "d",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"b"},
							},
						},
						{
							Name: "e",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"c", "d"},
							},
						},
					},
					JobRetainPolicy: flowv1alpha1.RetainPolicy("retain"),
				},
			},
			reviewResponse: admissionv1.AdmissionResponse{Allowed: true},
			ret:            "",
			ExpectErr:      false,
		},
		// 	Miss flow name a
		{
			Name: "validate valid-workflow with miss flow name a",
			Workflow: flowv1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-workflow",
					Namespace: namespace,
				},
				Spec: flowv1alpha1.WorkflowSpec{
					Flows: []flowv1alpha1.Flow{
						{
							Name: "b",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"a"},
							},
						},
						{
							Name: "c",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"a", "b"},
							},
						},
						{
							Name: "d",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"b"},
							},
						},
						{
							Name: "e",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"c", "d"},
							},
						},
					},
					JobRetainPolicy: flowv1alpha1.RetainPolicy("retain"),
				},
			},
			reviewResponse: admissionv1.AdmissionResponse{Allowed: false},
			ret:            "workflow Flow is not DAG: vertex is not defined: a",
			ExpectErr:      true,
		},
		// 	workflow flows not dag
		{
			Name: "validate valid-workflow with flows not dag",
			Workflow: flowv1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-workflow",
					Namespace: namespace,
				},
				Spec: flowv1alpha1.WorkflowSpec{
					Flows: []flowv1alpha1.Flow{
						{
							Name: "a",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"b"},
							},
						},
						{
							Name: "b",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"a"},
							},
						},
						{
							Name: "c",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"a", "b"},
							},
						},
						{
							Name: "d",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"b"},
							},
						},
						{
							Name: "e",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"c", "d"},
							},
						},
					},
					JobRetainPolicy: flowv1alpha1.RetainPolicy("retain"),
				},
			},
			reviewResponse: admissionv1.AdmissionResponse{Allowed: false},
			ret:            "workflow Flow is not DAG",
			ExpectErr:      true,
		},
		// 	workflow flows with muti c
		{
			Name: "validate valid-workflow with lows with muti c",
			Workflow: flowv1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-workflow",
					Namespace: namespace,
				},
				Spec: flowv1alpha1.WorkflowSpec{
					Flows: []flowv1alpha1.Flow{
						{
							Name: "a",
						},
						{
							Name: "b",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"a"},
							},
						},
						{
							Name: "c",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"a"},
							},
						},
						{
							Name: "c",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"b"},
							},
						},
					},
					JobRetainPolicy: flowv1alpha1.RetainPolicy("retain"),
				},
			},
			reviewResponse: admissionv1.AdmissionResponse{Allowed: true},
			ret:            "",
			ExpectErr:      false,
		},
	}

	// create fake volcano clientset
	config.VolcanoClient = fakeclient.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(config.VolcanoClient, 0)

	stopCh := make(chan struct{})
	informerFactory.Start(stopCh)
	for informerType, ok := range informerFactory.WaitForCacheSync(stopCh) {
		if !ok {
			panic(fmt.Errorf("failed to sync cache: %v", informerType))
		}
	}
	defer close(stopCh)

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			ret := validateWorkflowDAG(&testCase.Workflow, &testCase.reviewResponse)
			//fmt.Printf("test-case name:%s, ret:%v  testCase.reviewResponse:%v \n", testCase.Name, ret,testCase.reviewResponse)
			if testCase.ExpectErr == true && ret == "" {
				t.Errorf("Expect error msg :%s, but got nil.", testCase.ret)
			}
			if testCase.ExpectErr == true && testCase.reviewResponse.Allowed != false {
				t.Errorf("Expect Allowed as false but got true.")
			}
			if testCase.ExpectErr == true && !strings.Contains(ret, testCase.ret) {
				t.Errorf("Expect error msg :%s, but got diff error %v", testCase.ret, ret)
			}

			if testCase.ExpectErr == false && ret != "" {
				t.Errorf("Expect no error, but got error %v", ret)
			}
			if testCase.ExpectErr == false && testCase.reviewResponse.Allowed != true {
				t.Errorf("Expect Allowed as true but got false. %v", testCase.reviewResponse)
			}
		})
	}
}
