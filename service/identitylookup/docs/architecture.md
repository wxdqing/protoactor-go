# identitylookup 架构说明

## 总体结构

核心对象分 5 层：

1. `StorageIdentityLookup`
2. `Manager`
3. `placementActor` / `manualProxyActor`
4. `IdentityStorage`
5. 具体存储实现 `redis_storage` / `tcaplus_storage`

## 组件职责

### StorageIdentityLookup

入口定义在 `identity_lookup.go:8`。

职责：

- 适配 Proto.Actor Cluster 的 `IdentityLookup`
- 对外暴露 `Get`、`Activate`、`RemovePid`
- 在 `Setup()` 中创建并启动 `Manager`
- 在 `Shutdown()` 中停止 `Manager`

它自己几乎不承载业务决策，主要是委托给 `Manager`。

### Manager

核心实现位于 `manager.go:31`。

它是整个库的控制中枢，负责：

- 启动本地两个 actor：`storage-activator` 与 `storage-manual-proxy`
- 订阅集群拓扑事件
- 维护成员选择策略
- 读写持久化路由记录
- 决定是否需要激活 actor
- 在存储删除失败时进入重试队列

### placementActor

实现位于 `placement_actor.go:20`。

职责：

- 在本节点真正 `Spawn` grain actor
- 维护本节点已激活 grain 的内存映射
- 记录 `identity -> PID` 与 `pid -> identity`
- 监听子 actor `Terminated`
- 在 actor 终止时广播 `ActivationTerminated`

可以把它理解为“本机激活执行器”。

### manualProxyActor

实现位于 `manual_proxy_actor.go:12`。

职责很单一：

- 接收远程 `ActivationRequest`
- 把请求转成 `ManualActivateRequest`
- 再转发给本地 `placementActor`

它存在的原因是区分“自动激活入口”和“手动激活入口”。

### IdentityStorage

抽象定义在 `interfaces.go:13`。

它提供 5 个关键能力：

- `TryGetExistingActivation`
- `TryAcquireLock`
- `WaitForActivation`
- `StoreActivation`
- `RemoveActivation`

这几个接口共同构成了整个分布式互斥激活协议。

## 存储记录模型

统一语义由 `identity_storage.go:12` 和各存储的 `DataRecord` 实现承载。

一条记录至少包含这些字段：

- `Kind`
- `Identity`
- `Address`
- `ActorID`
- `MemberID`
- `LockTime`
- `ActivateTime`
- `TerminateTime`
- `Version`

字段语义：

- `Address + ActorID`：恢复 PID 所需的最小信息
- `MemberID`：记录 PID 所属 ActorSystem，带 epoch
- `LockTime`：激活锁占用时间
- `ActivateTime`：成功激活时间
- `TerminateTime`：终止标记
- `Version`：乐观锁版本

## 自动激活流程

当上层通过 `Get(clusterIdentity)` 查询时：

1. 先查存储
2. 如果存储中已有 PID，做 `MemberID` 有效性校验
3. 如果记录有效，直接返回 PID
4. 如果记录无效且不是 manual kind，进入激活流程
5. 通过存储加锁
6. 选择目标成员
7. 远程请求目标节点的 activator actor
8. 目标节点本地 `Spawn` grain
9. 把新 PID 回写到存储
10. 返回 PID

关键入口：

- `Manager.Get()`：`manager.go:175`
- `Manager.doActivate()`：`manager.go:235`
- `placementActor.activateActor()`：`placement_actor.go:95`

## 手动激活流程

手动 kind 有一条额外规则：

- `Get()` 不负责创建
- 必须显式调用 `Activate()`

执行路径：

1. 上层调用 `StorageIdentityLookup.Activate()`
2. 进入 `Manager.Activate()`
3. 走同样的查存储与加锁逻辑
4. 远程请求的是 `storage-manual-proxy`
5. `manualProxyActor` 再把请求包装成 `ManualActivateRequest`
6. 转给本节点 `placementActor`
7. `placementActor` 允许 manual kind 被创建

这样做的目的，是避免业务代码仅通过一次普通 `Get()` 就隐式创建某些需要显式生命周期管理的 grain。

## 成员选择策略

默认实现位于 `member_strategy/default.go:10`。

策略分两步：

1. 如果存储记录里已有 `memberID`，优先尝试复用该 member
2. 否则从支持对应 kind 的 member 中随机挑一个

这个策略的好处是简单，缺点也很明显：

- 没有一致性哈希
- 没有负载感知
- 没有机房/标签感知

因此它更像一个默认实现，而不是成熟的路由调度策略。

## MemberID 与 Epoch

实现位于 `system_id.go:10`。

`MemberID` 形如：

- `instance-name-epoch-123`

读取旧 PID 时会做两层校验：

1. 通过实例名找到当前 cluster member
2. 对比 epoch 是否一致

如果实例名还在，但 epoch 变化了，说明节点可能重启过，旧 PID 不应继续使用。

## 终止与回收

当本地 grain 被停止时：

1. `placementActor` 收到 `*actor.Terminated`
2. 减少 cluster kind 计数
3. 广播 `ActivationTerminated`
4. 调用 `Manager.RemovePid()`
5. 由存储把 `TerminateTime` 标记为当前时间
6. 其他节点收到终止事件后从 `PidCache` 中移除该映射

这个设计不是物理删除记录，而是标记终止。后续重新激活时，存储层会把旧终止记录视为“可重新抢占”。

## 存储层实现差异

### Redis

实现位于：

- `redis_storage/identity_access.go:12`
- `redis_storage/date_record.go:7`

特点：

- 记录用 JSON 存储
- `Insert` 用 Lua 实现“若不存在才写入”
- `Update` 用 Lua 做版本检查
- 维护简单，适合快速接入

### Tcaplus

实现位于：

- `tcaplus_storage/identity_access.go:13`
- `tcaplus_storage/tcaplus_table.go:16`

特点：

- 基于 protobuf 表结构
- 通过版本号做 CAS 更新
- 字段访问使用反射辅助函数
- 更贴近业务表结构，但配置复杂度更高

## 代码中的实际边界

当前代码展现出来的真实边界是：

- `StorageIdentityLookup`：Proto.Actor 适配层
- `Manager`：协议编排层
- `placementActor`：本地执行层
- `IdentityStorage`：持久化协议层
- `DataAccess`：具体数据库访问层

这个分层总体是清晰的，也是当前项目里最值得保留的设计点。
