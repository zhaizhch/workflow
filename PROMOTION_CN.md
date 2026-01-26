# 🚀 Work-Flow: Kubernetes 上的高性能 AI 与批处理工作流引擎集大成者

在云原生 AI 和大数据处理的浪潮中，如何高效、稳定地编排成百上千个复杂的任务依赖？如何确保编排引擎在面对大规模并发请求时依然坚如磐石？

**Work-Flow** 正是为了解决这些挑战而生。它是一个从零开始为 Kubernetes 构建的高性能、模块化工作流引擎。

---

## 📺 工作流执行全景演示 (以红色光球代表任务启动)

为了让您能够身临其境地理解 Work-Flow 的执行逻辑，我们通过“红色光球”代表当前活跃任务，为您展示各种复杂场景下的调度过程。

````carousel
![顺序启动](file:///Users/zhaizhicheng/.gemini/antigravity/brain/01e8c586-d35d-4101-afa3-9516da0f1eba/seq_start_png_1769397073238.png)
**1. 顺序启动 (Sequential)**：任务按照依赖关系精准有序地开启。红色光球在首个节点跳动，代表正在进行的数据摄取。
<!-- slide -->
![并行循环](file:///Users/zhaizhicheng/.gemini/antigravity/brain/01e8c586-d35d-4101-afa3-9516da0f1eba/parallel_for_png_1769397094272.png)
**2. 并行循环 (For Logic)**：当进入 `For` 逻辑时，主光球会瞬间分裂成多个子光球，并行驱动多个训练任务（如 `Train-0` 到 `Train-3`）。
<!-- slide -->
![自动重试](file:///Users/zhaizhicheng/.gemini/antigravity/brain/01e8c586-d35d-4101-afa3-9516da0f1eba/retry_flow_png_1769397112905.png)
**3. 自动重试 (Retry)**：遇到瞬时故障（红色 X）时，Work-Flow 会自动捕获异常。光球会顺着重试路径再次进入任务节点，确保存量任务不丢失。
<!-- slide -->
![条件判断与探测](file:///Users/zhaizhicheng/.gemini/antigravity/brain/01e8c586-d35d-4101-afa3-9516da0f1eba/probe_condition_png_1769397136250.png)
**4. 条件探测 (Probe)**：光球通过 `Probe` 检测点。根据实时的 HTTP/TCP 或任务状态反馈，动态选择“成功”或“失败”分支，实现智能化路径切换。
<!-- slide -->
![多进多出 DAG](file:///Users/zhaizhicheng/.gemini/antigravity/brain/01e8c586-d35d-4101-afa3-9516da0f1eba/multi_io_png_1769397156051.png)
**5. 复杂 DAG (Multi-In/Out)**：展示了多任务聚合（多个光球汇聚到一个节点）和多任务分发（一个节点触发多个下游光球）的顶级编排能力。
````

---

## 🏛 底层架构与黑科技

除了直观的任务流转，Work-Flow 在底层同样具备企业级的硬核实力。

````carousel
![高性能分片架构](file:///Users/zhaizhicheng/.gemini/antigravity/brain/01e8c586-d35d-4101-afa3-9516da0f1eba/sharding_visualization_png_1769396865935.png)
**高性能分片 (Sharding)**：不同于传统单列队，我们通过 Namespace/Name 哈希，将大并发压力均匀分摊，让吞吐量不再有上限。
<!-- slide -->
![主备高可用](file:///Users/zhaizhicheng/.gemini/antigravity/brain/01e8c586-d35d-4101-afa3-9516da0f1eba/architectural_overview_png_1769396629557.png)
**全集群高可用 (HA)**：基于 Leader Election 机制，副本秒级接管，为您的任务流提供“永远在线”的保障。
````

---

## 🌟 核心亮点

### 1. 🌈 极其丰富的负载编排能力

支持 **Batch**, **Kubeflow**, **Native K8s** 等全栈负载。

### 2. ⚡ 巅峰级的并发处理性能

采用 **Sharded Workqueue Architecture**，支撑千万级任务编排。

### 3. � 企业级的高可用性 (HA)

主备选举机制，故障自动恢复。

---

## � 立即体验

```bash
git clone https://github.com/zhaizhch/workflow.git
cd workflow
make install-crds
kubectl apply -f installer/controller/
```

让 Work-Flow 成为你云原生基础设施中的强力马达！

---

## 🤝 加入我们

如果您觉得有趣，请给我们一个 **Star** 🌟，这是对我们最大的鼓励！!
