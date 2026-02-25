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
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1flow "github.com/workflow.sh/work-flow/pkg/apis/flow/v1alpha1"
	"github.com/workflow.sh/work-flow/pkg/client/clientset/versioned/scheme"
)

// createJob 根据 WorkTemplate 和 Flow 配置，创建一个 Job 资源。
func (wc *workflowcontroller) createJob(workflow *v1alpha1flow.Workflow, flow v1alpha1flow.Flow, index int) error {
	job, gvr, err := wc.constructJobFromTemplate(workflow, flow, getJobName(workflow.Name, flow.Name, index))
	if err != nil {
		klog.Errorf("failed to construct job for flow %s in workflow %s: %v", flow.Name, workflow.Name, err)
		return err
	}

	if _, err := wc.dynamicClient.Resource(*gvr).Namespace(workflow.Namespace).Create(context.Background(), job, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	wc.recorder.Eventf(workflow, corev1.EventTypeNormal, "Created", "Successfully created job %s", job.GetName())
	return nil
}

// buildGVR 将 WorkTemplate 中定义的 GVR 元数据转换为 k8s 标准 GVR 类型。
// 此辅助函数消除了多处重复的 GroupVersionResource 构建代码（DRY 原则）。
func buildGVR(tmplGVR metav1.GroupVersionResource) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    tmplGVR.Group,
		Version:  tmplGVR.Version,
		Resource: tmplGVR.Resource,
	}
}

// constructJobFromTemplate 根据 WorkTemplate 构建完整的非结构化 Job 对象。
//
// 采用 Builder 模式，将构建过程拆分为 5 个有序步骤：
//  1. buildJobMeta    — 初始化 metadata（name、namespace、labels、annotations）
//  2. fillJobKind     — 通过 API Discovery 填充 Kind 字段
//  3. fillJobSpec     — 反序列化 JobSpec 并挂载到 spec
//  4. applyPatch      — 应用用户在 Flow 中自定义的 Patch 覆盖
//  5. injectGlobalSettings — 注入工作流级别的全局配置（schedulerName、plugins）
func (wc *workflowcontroller) constructJobFromTemplate(workflow *v1alpha1flow.Workflow, flow v1alpha1flow.Flow, jobName string) (*unstructured.Unstructured, *schema.GroupVersionResource, error) {
	jobTemplate, err := wc.workTemplateLister.WorkTemplates(workflow.Namespace).Get(flow.Name)
	if err != nil {
		return nil, nil, err
	}

	gvr := buildGVR(jobTemplate.Spec.GVR)

	// 步骤1：初始化 Job 基础元数据
	job := wc.buildJobMeta(workflow, flow, jobName, &gvr)

	// 步骤2：通过 Discovery 接口填充 Kind 字段
	if err := wc.fillJobKind(job, &gvr); err != nil {
		return nil, nil, err
	}

	// 步骤3：反序列化 JobSpec 并填入 spec 字段
	if err := fillJobSpec(job, jobTemplate); err != nil {
		return nil, nil, err
	}

	// 步骤4：应用 Flow 中的自定义 Patch
	if err := applyFlowPatch(job, flow); err != nil {
		return nil, nil, err
	}

	// 步骤5：注入工作流全局配置（schedulerName、plugins）
	injectGlobalSettings(job, workflow)

	if err := controllerutil.SetControllerReference(workflow, job, scheme.Scheme); err != nil {
		return nil, nil, fmt.Errorf("failed to set controller reference: %v", err)
	}

	return job, &gvr, nil
}

// buildJobMeta 初始化 Job 的 unstructured 基础结构，含 apiVersion、kind（留空待填）、metadata。
func (wc *workflowcontroller) buildJobMeta(workflow *v1alpha1flow.Workflow, flow v1alpha1flow.Flow, jobName string, gvr *schema.GroupVersionResource) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": gvr.GroupVersion().String(),
			"kind":       "", // 由 fillJobKind 步骤填充
			"metadata": map[string]interface{}{
				"name":      jobName,
				"namespace": workflow.Namespace,
				"labels": map[string]interface{}{
					CreatedByWorkTemplate: GenerateObjectString(workflow.Namespace, flow.Name),
					CreatedByWorkflow:     GenerateObjectString(workflow.Namespace, workflow.Name),
				},
				"annotations": map[string]interface{}{
					CreatedByWorkTemplate: GenerateObjectString(workflow.Namespace, flow.Name),
					CreatedByWorkflow:     GenerateObjectString(workflow.Namespace, workflow.Name),
				},
			},
		},
	}
}

// fillJobKind 通过 Kubernetes Discovery 接口查询资源对应的 Kind，并写入 job 对象。
func (wc *workflowcontroller) fillJobKind(job *unstructured.Unstructured, gvr *schema.GroupVersionResource) error {
	resources, err := wc.kubeClient.Discovery().ServerResourcesForGroupVersion(gvr.GroupVersion().String())
	if err != nil {
		return fmt.Errorf("failed to find server resources for %v: %v", gvr.GroupVersion(), err)
	}
	for _, resource := range resources.APIResources {
		if resource.Name == gvr.Resource {
			job.Object["kind"] = resource.Kind
			return nil
		}
	}
	return fmt.Errorf("failed to find kind for resource %s", gvr.Resource)
}

// fillJobSpec 将 WorkTemplate 中存储的 JobSpec（raw JSON）反序列化后挂载到 job.spec。
func fillJobSpec(job *unstructured.Unstructured, jobTemplate *v1alpha1flow.WorkTemplate) error {
	var specContent map[string]interface{}
	if err := json.Unmarshal(jobTemplate.Spec.JobSpec.Raw, &specContent); err != nil {
		return fmt.Errorf("failed to unmarshal job spec: %v", err)
	}
	job.Object["spec"] = specContent
	return nil
}

// applyFlowPatch 将 Flow 中用户自定义的 Patch 合并到 job 对象中（支持 metadata 和 spec 两个层级）。
func applyFlowPatch(job *unstructured.Unstructured, flow v1alpha1flow.Flow) error {
	if flow.Patch == nil || len(flow.Patch.Raw) == 0 {
		return nil
	}
	var patchMap map[string]interface{}
	if err := json.Unmarshal(flow.Patch.Raw, &patchMap); err != nil {
		return fmt.Errorf("failed to unmarshal patch: %v", err)
	}
	applyPatchMap(job, patchMap)
	return nil
}

// injectGlobalSettings 将工作流级别的全局设置（schedulerName、plugins）注入到 job.spec 中。
func injectGlobalSettings(job *unstructured.Unstructured, workflow *v1alpha1flow.Workflow) {
	if workflow.Spec.SchedulerName != "" {
		unstructured.SetNestedField(job.Object, workflow.Spec.SchedulerName, "spec", "schedulerName")
	}
	if len(workflow.Spec.Plugins) == 0 {
		return
	}
	// []string 需转为 []interface{}，因为 unstructured.DeepCopyJSONValue 不支持 []string
	pluginsMap := make(map[string]interface{}, len(workflow.Spec.Plugins))
	for k, v := range workflow.Spec.Plugins {
		interfaceSlice := make([]interface{}, len(v))
		for i, s := range v {
			interfaceSlice[i] = s
		}
		pluginsMap[k] = interfaceSlice
	}
	unstructured.SetNestedMap(job.Object, pluginsMap, "spec", "plugins")
}

// applyPatchMap 将 patchMap 中的各层级内容合并到 job 对象中。
// 支持 metadata（labels/annotations 合并）和 spec（字段覆盖）两种合并策略。
func applyPatchMap(job *unstructured.Unstructured, patchMap map[string]interface{}) {
	for k, v := range patchMap {
		if v == nil {
			continue
		}
		switch k {
		case "metadata":
			mergeMetadata(job, v)
		case "spec":
			mergeSpec(job, v)
		default:
			unstructured.SetNestedField(job.Object, v, "spec", k)
		}
	}
}

// mergeMetadata 将 patch 中的 labels 和 annotations 与 job 现有字段合并（不覆盖已有的 key）。
func mergeMetadata(job *unstructured.Unstructured, v interface{}) {
	patchMeta, ok := v.(map[string]interface{})
	if !ok {
		return
	}
	// 合并 labels
	if labelsMap, ok := patchMeta["labels"].(map[string]interface{}); ok {
		existing, _, _ := unstructured.NestedStringMap(job.Object, "metadata", "labels")
		if existing == nil {
			existing = make(map[string]string)
		}
		for lk, lv := range labelsMap {
			if s, ok := lv.(string); ok {
				existing[lk] = s
			}
		}
		unstructured.SetNestedStringMap(job.Object, existing, "metadata", "labels")
	}
	// 合并 annotations
	if annosMap, ok := patchMeta["annotations"].(map[string]interface{}); ok {
		existing, _, _ := unstructured.NestedStringMap(job.Object, "metadata", "annotations")
		if existing == nil {
			existing = make(map[string]string)
		}
		for ak, av := range annosMap {
			if s, ok := av.(string); ok {
				existing[ak] = s
			}
		}
		unstructured.SetNestedStringMap(job.Object, existing, "metadata", "annotations")
	}
}

// mergeSpec 将 patch 中的 spec 字段逐一覆盖到 job 的 spec 中。
func mergeSpec(job *unstructured.Unstructured, v interface{}) {
	specMap, ok := v.(map[string]interface{})
	if !ok {
		return
	}
	for sk, sv := range specMap {
		unstructured.SetNestedField(job.Object, sv, "spec", sk)
	}
}

// deleteAllJobsCreatedByWorkflow 删除该 Workflow 创建的所有 Job 资源。
func (wc *workflowcontroller) deleteAllJobsCreatedByWorkflow(workflow *v1alpha1flow.Workflow) error {
	for _, flow := range workflow.Spec.Flows {
		jobTemplate, err := wc.workTemplateLister.WorkTemplates(workflow.Namespace).Get(flow.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return err
		}

		// 使用统一的 buildGVR 辅助函数构建 GVR
		gvr := buildGVR(jobTemplate.Spec.GVR)

		replicas := 1
		if flow.For != nil && flow.For.Replicas != nil {
			replicas = int(*flow.For.Replicas)
		}

		for i := 0; i < replicas; i++ {
			index := -1
			if flow.For != nil {
				index = i
			}
			jobName := getJobName(workflow.Name, flow.Name, index)
			if err := wc.dynamicClient.Resource(gvr).Namespace(workflow.Namespace).Delete(context.Background(), jobName, metav1.DeleteOptions{}); err != nil {
				if errors.IsNotFound(err) {
					continue
				}
				klog.Errorf("Failed to delete resource %s %s/%s for workflow %s: %v", gvr.Resource, workflow.Namespace, jobName, workflow.Name, err)
				return err
			}
		}
	}
	return nil
}

// getAllJobsCreatedByWorkflow 查询该 Workflow 创建的所有 Job 资源（跨 GVR 类型）。
func (wc *workflowcontroller) getAllJobsCreatedByWorkflow(workflow *v1alpha1flow.Workflow) ([]unstructured.Unstructured, error) {
	// 收集去重后的 GVR 集合，避免对同类型资源重复查询
	gvrs := make(map[schema.GroupVersionResource]struct{})
	for _, flow := range workflow.Spec.Flows {
		jobTemplate, err := wc.workTemplateLister.WorkTemplates(workflow.Namespace).Get(flow.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		gvrs[buildGVR(jobTemplate.Spec.GVR)] = struct{}{}
	}

	var allJobs []unstructured.Unstructured
	for gvr := range gvrs {
		list, err := wc.getAllResourcesCreatedByWorkflow(context.Background(), workflow, gvr)
		if err != nil {
			return nil, err
		}
		allJobs = append(allJobs, list...)
	}
	return allJobs, nil
}

func (wc *workflowcontroller) getAllResourcesCreatedByWorkflow(ctx context.Context, workflow *v1alpha1flow.Workflow, gvr schema.GroupVersionResource) ([]unstructured.Unstructured, error) {
	labelSelector := map[string]string{
		CreatedByWorkflow: GenerateObjectString(workflow.Namespace, workflow.Name),
	}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector).String(),
	}

	list, err := wc.dynamicClient.Resource(gvr).Namespace(workflow.Namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}
