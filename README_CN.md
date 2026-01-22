# Work-Flow 控制器说明文档

Work-Flow 是一个用于在 Kubernetes 上管理工作流（Workflow）和工作模板（WorkTemplate）的控制器。

## 1. 构建 (Build)

在项目根目录下，使用 `Makefile` 进行构建：

```bash
# 构建二进制文件
make build
```

构建生成的二进制文件将存放在 `_output/bin/flow-controller`。

如果需要构建 Docker 镜像：

```bash
# 构建 Docker 镜像 (默认标签 volcanosh/work-flow:latest)
make images
```

## 2. 更新 (Update / Generate Code)

如果您修改了 `pkg/apis/flow/v1alpha1/` 下的类型定义，需要重新生成客户端代码和深拷贝方法：

```bash
# 生成代码
make generate
```

该命令会调用 `hack/update-gencode.sh` 脚本，使用 `code-generator` 工具更新 `pkg/client` 下的内容。

## 3. 部署 (Deployment)

### 第一步：安装 CRD

首先需要将 Workflow 和 WorkTemplate 的自定义资源定义（CRD）安装到集群中：

```bash
kubectl apply -f config/crd/bases/
```

### 第二步：部署控制器

部署控制器及其相关的 RBAC 权限：

```bash
kubectl apply -f config/controller/
```

> [!NOTE]
> 默认部署在 `volcano-system` 命名空间下。请确保该命名空间已存在，或者根据需要修改 `config/controller/*.yaml` 中的 namespace 字段。

## 4. 清理 (Clean)

移除构建产物：

```bash
make clean
```
