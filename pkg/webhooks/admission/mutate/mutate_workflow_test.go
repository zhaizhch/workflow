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

package mutate

import (
	"encoding/json"
	"testing"

	flowv1alpha1 "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMutateWorkflow(t *testing.T) {
	testCases := []struct {
		name          string
		workflow      flowv1alpha1.Workflow
		expectedPatch []patchOperation
		expectError   bool
	}{
		{
			name: "SuccessPolicy is nil",
			workflow: flowv1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{Name: "test-1"},
				Spec:       flowv1alpha1.WorkflowSpec{},
			},
			expectedPatch: []patchOperation{
				{
					Op:   "add",
					Path: "/spec/successPolicy",
					Value: map[string]interface{}{
						"type": "All",
					},
				},
			},
		},
		{
			name: "SuccessPolicy exists but Type is empty",
			workflow: flowv1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{Name: "test-2"},
				Spec: flowv1alpha1.WorkflowSpec{
					SuccessPolicy: &flowv1alpha1.SuccessPolicy{},
				},
			},
			expectedPatch: []patchOperation{
				{
					Op:    "add",
					Path:  "/spec/successPolicy/type",
					Value: "All",
				},
			},
		},
		{
			name: "SuccessPolicy is valid",
			workflow: flowv1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{Name: "test-3"},
				Spec: flowv1alpha1.WorkflowSpec{
					SuccessPolicy: &flowv1alpha1.SuccessPolicy{
						Type: flowv1alpha1.SuccessPolicyAny,
					},
				},
			},
			expectedPatch: nil, // No patch expected
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			patchBytes, err := mutateWorkflow(&tc.workflow)
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tc.expectedPatch == nil {
				if patchBytes != nil {
					t.Errorf("Expected nil patch but got: %s", string(patchBytes))
				}
				return
			}

			var actualPatches []patchOperation
			if err := json.Unmarshal(patchBytes, &actualPatches); err != nil {
				t.Errorf("Failed to unmarshal patch: %v", err)
				return
			}

			// Verify patch content
			// Since we manually construct, order should be stable.
			if len(actualPatches) != len(tc.expectedPatch) {
				t.Errorf("Expected %d patches, got %d", len(tc.expectedPatch), len(actualPatches))
				return
			}

			for i, p := range actualPatches {
				expected := tc.expectedPatch[i]
				if p.Op != expected.Op || p.Path != expected.Path {
					t.Errorf("Patch[%d] mismatch. Expected %v, got %v", i, expected, p)
				}

				// Value comparison might be tricky due to map interface{} vs struct
				// For simple types strings it's fine. For struct, we might need more care.
				// In first case, Value is map[string]interface{}

				// Let's rely on JSON equality for Value
				expectedValBytes, _ := json.Marshal(expected.Value)
				actualValBytes, _ := json.Marshal(p.Value)

				if string(expectedValBytes) != string(actualValBytes) {
					t.Errorf("Patch[%d] value listmatch. Expected %s, got %s", i, string(expectedValBytes), string(actualValBytes))
				}
			}
		})
	}
}
