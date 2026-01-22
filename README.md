# Work-Flow (Volcano-Flow)

Work-Flow 是一个构建在 Kubernetes 之上的高效工作流引擎，专注于批处理任务和 AI 训练任务的编排。它基于 GitHub 上的 `volcano-flow` 项目演进而来，允许用户通过定义简单的 YAML 配置来管理复杂的任务依赖关系。

## 核心特性

- **多工作负载支持**：
  - **Volcano Job**: 支持 Volcano 批处理作业。
  - **Kubeflow Jobs**: 原生支持 PyTorchJob, MPIJob, PaddleJob 等 AI 训练任务。
  - **Kubernetes Native**: 支持 Deployment 等原生 Kubernetes 资源。
- **动态依赖管理**：支持基于 DAG (有向无环图) 的任务编排，任务可以依赖于一个或多个上游任务的完成。
- **模板化定义**：通过 `WorkTemplate` 定义可重用的任务模板，简化 Workflow 编写。
- **灵活的触发机制**：支持基于任务状态（如 Completed, Running）的触发条件。

## 架构概览

Work-Flow 主要由两个 CRD (Custom Resource Definition) 组成：

1.  **WorkTemplate**: 定义具体的任务规格（如镜像、命令、副本数、资源需求）。它引用底层的 GVR (GroupVersionResource)，如 `batch.volcano.sh/v1alpha1/jobs` 或 `apps/v1/deployments`。
2.  **Workflow**: 定义任务的执行流程。它引用 `WorkTemplate` 并通过 `dependsOn` 字段描述任务间的依赖关系。

## 快速开始

### 前置条件

- Kubernetes 集群 (v1.20+)
- 已安装 Volcano (如果使用 Volcano Job)
- 已安装 Kubeflow Training Operator (如果使用 PyTorch/MPI/Paddle Job)

### 安装

1.  **克隆项目**
    ```bash
    git clone https://github.com/zhaizhch/workflow.git
    cd workflow
    ```

2.  **安装 CRD**
    ```bash
    make install-crds
    # 或者直接
    kubectl apply -f config/crd/bases/
    ```

3.  **部署控制器**
    ```bash
    kubectl apply -f config/controller/
    ```

### 使用示例

#### 1. 定义工作模板 (WorkTemplate)

在 `examples/worktemplates.yaml` 中定义你的任务模板。例如，定义一个 Deployment 任务和一个 MPI 训练任务：

```yaml
apiVersion: flow.workflow.sh/v1alpha1
kind: WorkTemplate
metadata:
  name: deploy-template
  namespace: default
spec:
  gvr:
    group: apps
    version: v1
    resource: deployments
  jobSpec:
    replicas: 1
    selector:
      matchLabels:
        app: deploy-task
    template:
      metadata:
        labels:
          app: deploy-task
      spec:
        containers:
        - name: nginx
          image: nginx:latest
---
apiVersion: flow.workflow.sh/v1alpha1
kind: WorkTemplate
metadata:
  name: mpijob-template
spec:
  gvr:
    group: kubeflow.org
    version: v1
    resource: mpijobs
  jobSpec:
    slotsPerWorker: 1
    cleanPodPolicy: Running
    mpiReplicaSpecs:
      Launcher:
        replicas: 1
        template:
           spec:
             containers:
             - image: horovod/horovod:latest
               name: mpi-launcher
               command: ["mpirun", "..."]
      Worker:
        replicas: 2
        template:
          spec:
            containers:
            - image: horovod/horovod:latest
              name: mpi-worker
```

#### 2. 定义工作流 (Workflow)

在 `examples/workflows.yaml` 中编排这些模板。以下示例展示了一个 Workflow，其中 `mpijob-template` 只有在 `deploy-template` 达到预期状态后才会运行（注意：Deployment 通常达到 Running 即视为可用，而 Job 需要 Completed）：

```yaml
apiVersion: flow.workflow.sh/v1alpha1
kind: Workflow
metadata:
  name: sample-workflow
  namespace: default
spec:
  jobRetainPolicy: delete
  flows:
    - name: deploy-template
    - name: mpijob-template
      dependsOn:
        targets:
          - deploy-template
```

#### 3. 运行示例

```bash
# 部署模板和工作流
kubectl apply -f examples/worktemplates.yaml
kubectl apply -f examples/workflows.yaml

# 查看工作流状态
kubectl get workflows -A
```

## 开发指南

### 构建与测试

项目根目录提供了 `Makefile` 以辅助开发：

- **构建二进制**:
  ```bash
  make build
  ```
- **构建 Docker 镜像**:
  ```bash
  make images
  ```
- **运行单元测试**:
  ```bash
  make test
  ```
- **生成代码 (Deepcopy/Client)**:
  ```bash
  make generate
  ```
  *注意：如果修改了 `pkg/apis` 下的定义，必须运行此命令。*

### 目录结构

- `pkg/apis`: CRD 类型定义。
- `pkg/controllers/workflow`: Workflow 控制器核心逻辑。
- `pkg/controllers/workload`: 不同工作负载的适配层 (如 `volcano.go`, `kubeflow.go`, `native.go`)。如果需要支持新的 CRD，请在此处扩展。
- `examples`: 示例 YAML 文件。

## 贡献

欢迎提交 Issue 和 Pull Request！添加新特性时，请确保同时更新 `README.md` 和相关测试用例。
