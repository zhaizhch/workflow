# Work-Flow 项目详细设计文档

## 1. 项目简介
`Work-Flow` 是一款基于 Kubernetes 生态原生的作业编排与工作流控制器（Workflow Controller）。该系统旨在通过自定义资源（CRD）建立一套声明式的、支持高并发及多重依赖策略的分布式作业流转平台。它允许用户编排和聚合复杂的、不同类型的异构工作负载（如 Volcano Batch Job，原生 Kubernetes Deployment，以及 Kubeflow 相关的各种 AI Training Jobs）。

## 2. 核心架构与模块组成

系统采用了标准的 Kubernetes Operator 架构（即基于控制器模式 Controller Pattern 实现）。

### 2.1 API 设计 (Custom Resource Definitions)
本项目的核心定义在 `pkg/apis/flow/v1alpha1` 下面：
1. **Workflow (wf)**: 代表一个完整的工作流定义。核心字段 `WorkflowSpec` 包括：
   - `Flows`: 一个 `[]Flow` 的列表，每个 Flow 描述了工作流中的一个具体节点或任务。
   - `SuccessPolicy`: 定义整个工作流被视为“成功”的策略（包括 `All` 全部成功、`Any` 任一叶子节点成功、`Critical` 所有的关键路径均成功）。
2. **WorkTemplate (wt / jt)**: 定义了实际的任务模版定义。
   - `JobSpec`: 支持以 `runtime.RawExtension` 形式接收任意格式的任务定义。
   - `GVR (GroupVersionResource)`: 明确指点了具体的 Kubernetes 组、版本和资源类型（便于多工作负载调度）。

### 2.2 控制器 (Controllers) 架构
控制器分为三大核心控制器进行协同工作：
1. **Workflow Controller** (`pkg/controllers/workflow`):
   - **职责**：整个系统的核心大脑。解析 Workflow CR，根据内部依赖关系、成功策略等构建有向无环图（DAG）。
   - **状态机**：维护工作流自身的生命周期状态 (`Pending` -> `Running` -> `Succeed` / `Failed` / `Terminating`)。通过不同的 state 文件解耦状态转移。
   - **依赖解析**：通过深度扫描依赖配置 (`DependsOn`, 涵盖 `Targets`, `Probe`, `OrGroups`)，决策接下来应该执行（未被 Block）的任务。
2. **WorkTemplate Controller** (`pkg/controllers/worktemplate`):
   - **职责**：对模板进行监听与处理，将单纯的模板根据运行时的需要扩展或组装为具体的任务资源。
3. **Generic Workload Controller 机制** (`pkg/controllers/workload`):
   - **职责**：由于工作负载是异构的，通过定义统一的 `Workload` 接口进行扩展，实现动态资源的状态收集与判定。
   - **已支持的负载**：
     - `Native`: Kubernetes 原生 `apps/v1` `Deployments`。
     - `Volcano`: `batch.volcano.sh/v1alpha1` 定制的 `Job`。
     - `Kubeflow`: 支持 `pytorchjobs`, `mpijobs`, `paddlejobs`。

### 2.3 Webhook 与验证
放置于 `pkg/webhooks` 中，提供 Webhook 服务进行 API 拦截过滤：
- **Mutating Webhook**: 在创建各类流程前自动补充部分缺省校验值或者填充结构。
- **Validating Webhook**: 对创建或修改 `Workflow` 时的属性进行严格审计（比如闭环检测、重试配置确认等）。

## 3. 功能特性与设计亮点

### 3.1 弹性的依赖解析树 (`Dependency Strategy`)
- **条件探针 (Probe)**：可以直接依赖其他运行任务的精确相切状态 (`Phase`), 也可以借助 HTTP 或 TCP 探针来判断网络可用性再流转。
- **多副本遍历 (`forEachReplica`)**: 因为一个 Flow 可以被 For 循环拉起多个实例，控制器支持对底层所有衍生子作业状态的批量判定（支持策略包含 `All` 和 `Any`）。
- **复合依赖网络**: 特别设计了 `OrGroups`，支持类似 (A && B) || (C && D) 的逻辑判断树进行任务放行。

### 3.2 灵活可靠的成功策略 (`Success Policy`)
区别于传统的单一“全部完成即成功”，`work-flow` 提供三种机制进行最后结果确认：
- **All**: （缺省方案）必须所有的任务组件均未 Failed 且全部完成。
- **Any**: 只要图中存在一条通路能够顺利抵达叶子节点即视为最终成功（常用于主备降级编排等策略架构）。
- **Critical**: 用户指定一批任务名称集合，只有核心被满足，流程即算过关。
此外配合 `ContinueOnFail` 属性，能够做到遇到边缘崩溃静默掠过而不影响主干分支。

### 3.3 无缝集成多异构工作负载
由于大量深度学习以及 HPC 项目涉及并非传统的结构，基于 `unstructured.Unstructured` 解析动态的 CR 对象使得只要实现 `Workload` 接口的方法：
- `GetJobStatus()` 处理状态解析转换
- `GetGVR()` 注册监听资源
- `GetPodLabels()` 获取 Pod 元数据以进行更细粒度的控制，
开发者就可以随时扩展其他例如 SparkJob 或 RayJob 等生态方案的集成。

## 4. 总结
`Work-Flow` 系统不仅兼具出色的容错调度机制以及微服务解耦特性，其内置的高度扩展作业池以及基于依赖的有向调度能够完全满足从通用自动化到精密 AI 训练流程流转的工业级需求，是一款具备高性能和高度灵活性的 Operator 平台。
