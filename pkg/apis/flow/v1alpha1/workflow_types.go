/*
Copyright 2021.

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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	batchv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

// WorkflowSpec defines the desired state of Workflow
type WorkflowSpec struct {
	// +optional
	Flows []Flow `json:"flows,omitempty" protobuf:"bytes,1,rep,name=flows"`
	// +optional
	JobRetainPolicy RetainPolicy `json:"jobRetainPolicy,omitempty" protobuf:"bytes,2,opt,name=jobRetainPolicy"`
	// +optional
	SchedulerName string `json:"schedulerName,omitempty" protobuf:"bytes,3,opt,name=schedulerName"`
	// +optional
	Plugins map[string][]string `json:"plugins,omitempty" protobuf:"bytes,4,rep,name=plugins"`
}

// +k8s:deepcopy-gen=true
// Flow defines the dependent of jobs
type Flow struct {
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// +optional
	DependsOn *DependsOn `json:"dependsOn,omitempty" protobuf:"bytes,2,opt,name=dependsOn"`
	// +optional
	For *For `json:"for,omitempty" protobuf:"bytes,7,opt,name=for"`
	// +optional
	Retry *Retry `json:"retry,omitempty" protobuf:"bytes,8,opt,name=retry"`
	// +optional
	Patch *Patch `json:"patch,omitempty" protobuf:"bytes,9,opt,name=patch"`
}

// +k8s:deepcopy-gen=true
type For struct {
	// +optional
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`
	// +optional
	DependsOn *DependsOn `json:"dependsOn,omitempty" protobuf:"bytes,2,opt,name=dependsOn"`
}

// +k8s:deepcopy-gen=true
type Retry struct {
	// +optional
	MaxRetries int32 `json:"maxRetries,omitempty" protobuf:"varint,1,opt,name=maxRetries"`
	// +optional
	Interval string `json:"interval,omitempty" protobuf:"bytes,2,opt,name=interval"`
}

type DependsOn struct {
	// Simple AND: All targets must meet condition
	// +optional
	Targets []string `json:"targets,omitempty" protobuf:"bytes,1,rep,name=targets"`
	// OR logic: One of these groups must meet condition.
	// Each group is itself an AND of its targets/probe.
	// +optional
	OrGroups []DependencyGroup `json:"orGroups,omitempty" protobuf:"bytes,4,rep,name=orGroups"`
	// +optional
	Probe *Probe `json:"probe,omitempty" protobuf:"bytes,2,opt,name=probe"`
	// +optional
	Strategy DependencyStrategy `json:"strategy,omitempty" protobuf:"bytes,3,opt,name=strategy"`
}

// +k8s:deepcopy-gen=true
type DependencyGroup struct {
	// +optional
	Targets []string `json:"targets,omitempty" protobuf:"bytes,1,rep,name=targets"`
	// +optional
	Probe *Probe `json:"probe,omitempty" protobuf:"bytes,2,opt,name=probe"`
	// +optional
	Strategy DependencyStrategy `json:"strategy,omitempty" protobuf:"bytes,3,opt,name=strategy"`
}

type DependencyStrategy string

const (
	All DependencyStrategy = "All"
	Any DependencyStrategy = "Any"
)

// +k8s:deepcopy-gen=true
// +kubebuilder:pruning:PreserveUnknownFields
// +kubebuilder:validation:Schemaless
type Patch struct {
	// +optional
	runtime.RawExtension `json:",inline"`
}

type Probe struct {
	// +optional
	HttpGetList []HttpGet `json:"httpGetList,omitempty" protobuf:"bytes,1,rep,name=httpGetList"`
	// +optional
	TcpSocketList []TcpSocket `json:"tcpSocketList,omitempty" protobuf:"bytes,2,rep,name=tcpSocketList"`
	// +optional
	TaskStatusList []TaskStatus `json:"taskStatusList,omitempty" protobuf:"bytes,3,rep,name=taskStatusList"`
}

type HttpGet struct {
	// +kubebuilder:validation:MaxLength=253
	// +optional
	TaskName string `json:"taskName,omitempty" protobuf:"bytes,1,opt,name=taskName"`
	// +optional
	Path string `json:"path,omitempty" protobuf:"bytes,2,opt,name=path"`
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	Port int `json:"port,omitempty" protobuf:"varint,3,opt,name=port"`
	// +optional
	HTTPHeader v1.HTTPHeader `json:"httpHeader,omitempty" protobuf:"bytes,4,opt,name=httpHeader"`
}

type TcpSocket struct {
	// +kubebuilder:validation:MaxLength=253
	// +optional
	TaskName string `json:"taskName,omitempty" protobuf:"bytes,1,opt,name=taskName"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	// +required
	Port int `json:"port" protobuf:"varint,2,opt,name=port"`
}

type TaskStatus struct {
	// +kubebuilder:validation:MaxLength=253
	// +optional
	TaskName string `json:"taskName,omitempty" protobuf:"bytes,1,opt,name=taskName"`
	// +kubebuilder:validation:MaxLength=63
	// +optional
	Phase string `json:"phase,omitempty" protobuf:"bytes,2,opt,name=phase"`
}

// WorkflowStatus defines the observed state of Workflow
type WorkflowStatus struct {
	// +optional
	PendingJobs []string `json:"pendingJobs,omitempty" protobuf:"bytes,1,rep,name=pendingJobs"`
	// +optional
	RunningJobs []string `json:"runningJobs,omitempty" protobuf:"bytes,2,rep,name=runningJobs"`
	// +optional
	FailedJobs []string `json:"failedJobs,omitempty" protobuf:"bytes,3,rep,name=failedJobs"`
	// +optional
	CompletedJobs []string `json:"completedJobs,omitempty" protobuf:"bytes,4,rep,name=completedJobs"`
	// +optional
	TerminatedJobs []string `json:"terminatedJobs,omitempty" protobuf:"bytes,5,rep,name=terminatedJobs"`
	// +optional
	UnKnowJobs []string `json:"unKnowJobs,omitempty" protobuf:"bytes,6,rep,name=unKnowJobs"`
	// +optional
	JobStatusList []JobStatus `json:"jobStatusList,omitempty" protobuf:"bytes,8,rep,name=jobStatusList"`
	// +optional
	Conditions map[string]Condition `json:"conditions,omitempty" protobuf:"bytes,8,rep,name=conditions"`
	// +optional
	State State `json:"state,omitempty" protobuf:"bytes,9,opt,name=state"`
}

type JobStatus struct {
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// +optional
	State batchv1alpha1.JobPhase `json:"state,omitempty" protobuf:"bytes,2,opt,name=state"`
	// +optional
	StartTimestamp metav1.Time `json:"startTimestamp,omitempty" protobuf:"bytes,3,opt,name=startTimestamp"`
	// +optional
	EndTimestamp metav1.Time `json:"endTimestamp,omitempty" protobuf:"bytes,4,opt,name=endTimestamp"`
	// +kubebuilder:validation:Minimum=0
	// +optional
	RestartCount int32 `json:"restartCount,omitempty" protobuf:"varint,5,opt,name=restartCount"`
	// +optional
	RunningHistories []JobRunningHistory `json:"runningHistories,omitempty" protobuf:"bytes,6,rep,name=runningHistories"`
}

type JobRunningHistory struct {
	// +optional
	StartTimestamp metav1.Time `json:"startTimestamp,omitempty" protobuf:"bytes,1,opt,name=startTimestamp"`
	// +optional
	EndTimestamp metav1.Time `json:"endTimestamp,omitempty" protobuf:"bytes,2,opt,name=endTimestamp"`
	// +optional
	State batchv1alpha1.JobPhase `json:"state,omitempty" protobuf:"bytes,3,opt,name=state"`
}

type State struct {
	// +optional
	Phase Phase `json:"phase,omitempty" protobuf:"bytes,1,opt,name=phase"`
}

// +kubebuilder:validation:Enum=retain;delete;delete-on-success
type RetainPolicy string

const (
	Retain          RetainPolicy = "retain"
	Delete          RetainPolicy = "delete"
	DeleteOnSuccess RetainPolicy = "delete-on-success"
)

// +kubebuilder:validation:Enum=Succeed;Terminating;Failed;Running;Pending
type Phase string

const (
	Succeed     Phase = "Succeed"
	Terminating Phase = "Terminating"
	Failed      Phase = "Failed"
	Running     Phase = "Running"
	Pending     Phase = "Pending"
)

// +k8s:deepcopy-gen=true
type Condition struct {
	Phase           batchv1alpha1.JobPhase             `json:"phase,omitempty" protobuf:"bytes,1,opt,name=phase"`
	CreateTimestamp metav1.Time                        `json:"createTime,omitempty" protobuf:"bytes,2,opt,name=createTime"`
	RunningDuration *metav1.Duration                   `json:"runningDuration,omitempty" protobuf:"bytes,3,opt,name=runningDuration"`
	TaskStatusCount map[string]batchv1alpha1.TaskState `json:"taskStatusCount,omitempty" protobuf:"bytes,4,rep,name=taskStatusCount"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+kubebuilder:object:root=true
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=workflows,shortName=jf
//+kubebuilder:subresource:status

// Workflow is the Schema for the workflows API
type Workflow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   WorkflowSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status WorkflowStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+kubebuilder:object:root=true

// WorkflowList contains a list of Workflow
type WorkflowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Workflow `json:"items" protobuf:"bytes,2,rep,name=items"`
}
