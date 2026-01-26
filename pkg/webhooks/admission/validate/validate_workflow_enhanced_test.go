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

package validate

import (
	"testing"

	admissionv1 "k8s.io/api/admission/v1"

	flowv1alpha1 "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
)

// Test OR dependencies validation
func TestValidateWorkflowWithOrGroups(t *testing.T) {
	tests := []struct {
		name        string
		workflow    *flowv1alpha1.Workflow
		expectError bool
		errorMsg    string
	}{
		{
			name: "OR dependency with cycle",
			workflow: &flowv1alpha1.Workflow{
				Spec: flowv1alpha1.WorkflowSpec{
					Flows: []flowv1alpha1.Flow{
						{
							Name: "A",
							DependsOn: &flowv1alpha1.DependsOn{
								OrGroups: []flowv1alpha1.DependencyGroup{
									{Targets: []string{"B"}},
								},
							},
						},
						{
							Name: "B",
							DependsOn: &flowv1alpha1.DependsOn{
								OrGroups: []flowv1alpha1.DependencyGroup{
									{Targets: []string{"A"}},
								},
							},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "workflow Flow is not DAG",
		},
		{
			name: "OR dependency no cycle",
			workflow: &flowv1alpha1.Workflow{
				Spec: flowv1alpha1.WorkflowSpec{
					Flows: []flowv1alpha1.Flow{
						{Name: "A"},
						{Name: "B"},
						{
							Name: "C",
							DependsOn: &flowv1alpha1.DependsOn{
								OrGroups: []flowv1alpha1.DependencyGroup{
									{Targets: []string{"A"}},
									{Targets: []string{"B"}},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Mixed AND and OR dependencies with cycle",
			workflow: &flowv1alpha1.Workflow{
				Spec: flowv1alpha1.WorkflowSpec{
					Flows: []flowv1alpha1.Flow{
						{Name: "A"},
						{
							Name: "B",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"A"},
							},
						},
						{
							Name: "C",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"B"},
								OrGroups: []flowv1alpha1.DependencyGroup{
									{Targets: []string{"D"}},
								},
							},
						},
						{
							Name: "D",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"C"}},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "workflow Flow is not DAG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reviewResponse := &admissionv1.AdmissionResponse{Allowed: true}
			msg := validateWorkflowDAG(tt.workflow, reviewResponse)

			if tt.expectError {
				if reviewResponse.Allowed {
					t.Errorf("Expected validation to fail, but it passed")
				}
				if msg == "" {
					t.Errorf("Expected error message, but got empty string")
				}
			} else {
				if !reviewResponse.Allowed {
					t.Errorf("Expected validation to pass, but it failed with: %s", msg)
				}
			}
		})
	}
}

// Test For loop dependencies validation
func TestValidateWorkflowWithForDependencies(t *testing.T) {
	replicas := int32(3)
	tests := []struct {
		name        string
		workflow    *flowv1alpha1.Workflow
		expectError bool
		errorMsg    string
	}{
		{
			name: "For loop with self-reference in Targets",
			workflow: &flowv1alpha1.Workflow{
				Spec: flowv1alpha1.WorkflowSpec{
					Flows: []flowv1alpha1.Flow{
						{
							Name: "parallel-task",
							For: &flowv1alpha1.For{
								Replicas: &replicas,
								DependsOn: &flowv1alpha1.DependsOn{
									Targets: []string{"parallel-task"},
								},
							},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "self-referencing For dependency",
		},
		{
			name: "For loop with self-reference in OrGroups",
			workflow: &flowv1alpha1.Workflow{
				Spec: flowv1alpha1.WorkflowSpec{
					Flows: []flowv1alpha1.Flow{
						{
							Name: "parallel-task",
							For: &flowv1alpha1.For{
								Replicas: &replicas,
								DependsOn: &flowv1alpha1.DependsOn{
									OrGroups: []flowv1alpha1.DependencyGroup{
										{Targets: []string{"parallel-task"}},
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "self-referencing For dependency",
		},
		{
			name: "For loop without self-reference",
			workflow: &flowv1alpha1.Workflow{
				Spec: flowv1alpha1.WorkflowSpec{
					Flows: []flowv1alpha1.Flow{
						{Name: "init"},
						{
							Name: "parallel-task",
							For: &flowv1alpha1.For{
								Replicas: &replicas,
								DependsOn: &flowv1alpha1.DependsOn{
									Targets: []string{"init"},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reviewResponse := &admissionv1.AdmissionResponse{Allowed: true}
			msg := validateWorkflowDAG(tt.workflow, reviewResponse)

			if tt.expectError {
				if reviewResponse.Allowed {
					t.Errorf("Expected validation to fail, but it passed")
				}
				if msg == "" {
					t.Errorf("Expected error message, but got empty string")
				}
			} else {
				if !reviewResponse.Allowed {
					t.Errorf("Expected validation to pass, but it failed with: %s", msg)
				}
			}
		})
	}
}

// Test self-loop detection
func TestValidateWorkflowWithSelfLoop(t *testing.T) {
	tests := []struct {
		name        string
		workflow    *flowv1alpha1.Workflow
		expectError bool
		errorMsg    string
	}{
		{
			name: "Simple self-loop",
			workflow: &flowv1alpha1.Workflow{
				Spec: flowv1alpha1.WorkflowSpec{
					Flows: []flowv1alpha1.Flow{
						{
							Name: "A",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"A"},
							},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "self-dependency",
		},
		{
			name: "No self-loop",
			workflow: &flowv1alpha1.Workflow{
				Spec: flowv1alpha1.WorkflowSpec{
					Flows: []flowv1alpha1.Flow{
						{Name: "A"},
						{
							Name: "B",
							DependsOn: &flowv1alpha1.DependsOn{
								Targets: []string{"A"},
							},
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reviewResponse := &admissionv1.AdmissionResponse{Allowed: true}
			msg := validateWorkflowDAG(tt.workflow, reviewResponse)

			if tt.expectError {
				if reviewResponse.Allowed {
					t.Errorf("Expected validation to fail, but it passed")
				}
				if msg == "" {
					t.Errorf("Expected error message, but got empty string")
				}
			} else {
				if !reviewResponse.Allowed {
					t.Errorf("Expected validation to pass, but it failed with: %s", msg)
				}
			}
		})
	}
}

// Test empty workflow validation
func TestValidateEmptyWorkflow(t *testing.T) {
	workflow := &flowv1alpha1.Workflow{
		Spec: flowv1alpha1.WorkflowSpec{
			Flows: []flowv1alpha1.Flow{},
		},
	}

	reviewResponse := &admissionv1.AdmissionResponse{Allowed: true}
	msg := validateWorkflowDAG(workflow, reviewResponse)

	if reviewResponse.Allowed {
		t.Errorf("Expected empty workflow to be rejected, but it was allowed")
	}
	if msg != "workflow must have at least one flow" {
		t.Errorf("Expected error message 'workflow must have at least one flow', got: %s", msg)
	}
}
