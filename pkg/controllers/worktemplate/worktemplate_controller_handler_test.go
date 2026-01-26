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

package worktemplate

import (
	"testing"

	"github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	batch "volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

func TestAddWorkTemplate(t *testing.T) {
	f := newFixture(t)
	c, _ := f.newController()

	wt := &v1alpha1.WorkTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-wt",
			Namespace: "default",
		},
	}

	c.addWorkTemplate(wt)

	if c.queues[0].Len() == 0 {
		t.Error("expected worktemplate to be enqueued")
	}
}

func TestAddJob(t *testing.T) {
	f := newFixture(t)
	c, _ := f.newController()

	t.Run("Job without label", func(t *testing.T) {
		job := &batch.Job{
			ObjectMeta: metav1.ObjectMeta{Name: "job1", Namespace: "default"},
		}
		c.addJob(job)
		if c.queues[0].Len() != 0 {
			t.Error("did not expect job without label to be enqueued")
		}
	})

	t.Run("Job with correct label", func(t *testing.T) {
		job := &batch.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "job2",
				Namespace: "default",
				Labels: map[string]string{
					CreatedByWorkTemplate: "default.test-wt",
				},
			},
		}
		c.addJob(job)
		if c.queues[0].Len() == 0 {
			t.Error("expected job with correct label to trigger enqueue")
		}
	})
}
