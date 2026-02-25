# 配额管理错误码文档

## 概述
本文档详细列出了平台配额管理模块中使用的所有错误码及其中文翻译说明。这些错误码定义在 `internal/common/codes/quota.go` 文件中。

---

## 错误码列表

### 基础配额错误

| 错误码 | 常量名 | 英文描述 | 中文描述 | HTTP状态码 | gRPC状态码 |
|------|------|--------|--------|----------|----------|
| 010001 | QuotaNotEnough | quota not enough | 配额不足 | 400 | InvalidArgument |
| 010003 | QueryQuotaErr | query quota error | 查询配额错误 | 500 | Internal |
| 010006 | CreateQuotaErr | create quota package error | 创建配额错误 | 500 | Internal |
| 010008 | QuotaCheckErr | quota check error | 配额检查错误 | 400 | Internal |
| 010009 | QuotaUpdateErr | quota update error | 配额更新错误 | 500 | Internal |
| 010019 | DeleteQuotaErr | delete quota error | 删除配额错误 | 500 | Internal |

### 配额检查和验证

| 错误码 | 常量名 | 英文描述 | 中文描述 | HTTP状态码 | gRPC状态码 |
|------|------|--------|--------|----------|----------|
| 010007 | QuotaPreCheckErr | pre check error | 配额预检查错误 | 500 | Internal |
| 010018 | QuotaPreCheckFailed | pre check failed | 配额预检查失败 | 500 | Internal |

### 物理资源错误

| 错误码 | 常量名 | 英文描述 | 中文描述 | HTTP状态码 | gRPC状态码 |
|------|------|--------|--------|----------|----------|
| 010004 | PhysicalMachineQueryErr | physical machine query error | 物理资源查询错误 | 500 | Internal |
| 010010 | PhysicalMachineUpdateErr | physical machine update error | 物理资源更新错误 | 500 | Internal |

### 加速器套餐错误

| 错误码 | 常量名 | 英文描述 | 中文描述 | HTTP状态码 | gRPC状态码 |
|------|------|--------|--------|----------|----------|
| 010005 | AcceleratorPackageQueryErr | accelerator package query error | 加速卡套餐查询错误 | 500 | Internal |

### 存储配额错误

| 错误码 | 常量名 | 英文描述 | 中文描述 | HTTP状态码 | gRPC状态码 |
|------|------|--------|--------|----------|----------|
| 010011 | StorageQuotaUpdateErr | storage quota update error | 存储配额更新错误 | 500 | Internal |
| 010016 | StorageQuotaCreateErr | storage quota create error | 存储配额创建错误 | 500 | Internal |
| 010017 | StorageQuotaDBUpdateErr | storage quota db update error | 存储配额数据库更新错误 | 500 | Internal |

### 目录和队列管理错误

| 错误码 | 常量名 | 英文描述 | 中文描述 | HTTP状态码 | gRPC状态码 |
|------|------|--------|--------|----------|----------|
| 010012 | CreateDirErr | create directory error | 创建目录错误 | 500 | Internal |
| 010013 | CreateVolcanoQueueErr | create volcano queue error | 创建Volcano队列错误 | 500 | Internal |
| 010014 | UpdateVolcanoQueueErr | update volcano queue error | 更新Volcano队列错误 | 500 | Internal |
| 010015 | VolcanoQueueNotExistErr | volcano queue not exist error | Volcano队列不存在错误 | 500 | Internal |
