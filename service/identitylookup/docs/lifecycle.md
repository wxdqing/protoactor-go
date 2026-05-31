# identitylookup 生命周期梳理

## 1. 初始化阶段

### 创建存储

业务先构造一个 `IdentityStorage`：

- Redis：`redis_storage.NewIdentityDataAccess(...)` + `identitylookup.NewIdentityStorage(...)`
- Tcaplus：`tcaplus_storage.NewIdentityDataAccess(...)` + `identitylookup.NewIdentityStorage(...)`

### 创建 lookup

调用：

- `identitylookup.NewStorageIdentityLookup(storage, manualKinds)`

这里会保存：

- 存储实现
- manual kinds 列表

### Cluster Setup

当 Proto.Actor Cluster 初始化 lookup 时，会调用：

- `identity_lookup.go:35` `Setup(cluster, kinds, isClient)`

这里会：

1. 创建 `Manager`
2. 启动 `Manager`

## 2. Manager 启动阶段

`Manager.Start()` 位于 `manager.go:64`。

启动动作包括：

1. 启动 `storage-activator`
2. 启动 `storage-manual-proxy`
3. 启动失败回收 ticker
4. 订阅 EventStream

### EventStream 监听两个事件

- `ClusterTopology`
- `ActivationTerminated`

这意味着 Manager 在整个运行过程中既参与“新成员拓扑学习”，也参与“PID 缓存失效”。

## 3. 集群拓扑收敛阶段

当集群成员变化时，会收到 `ClusterTopology`，然后：

1. 遍历拓扑中的 member
2. 从 member ID 中抽出 `systemID`
3. 解析出 `name + epoch`
4. 更新 `memberStrategy`

这一步是后续“旧 PID 是否仍可信”的基础。

## 4. 正常查询阶段

这是最频繁的运行期路径。

### 4.1 先查持久化记录

`Manager.Get()` 先调用 `TryGetExistingActivation()`：

- 没有记录：返回空
- 有 `TerminateTime`：视为无效
- `Address/ActorID/MemberID` 不完整：视为无效
- 有完整记录：返回 `StoredActivation`

### 4.2 校验记录是否仍可信

`Manager.getPidFromActivation()` 会做：

1. 取出 `activation.MemberID`
2. 解析 `member name + epoch`
3. 根据名字找当前 member
4. 对比 epoch

只有全部通过，才返回 PID。

### 4.3 决定是否触发激活

- 如果是普通 kind：无有效 PID 时自动激活
- 如果是 manual kind：直接返回 `nil`

这就是“自动激活”和“手动激活”语义分叉点。

## 5. 自动激活阶段

### 5.1 抢占激活锁

`TryAcquireLock()` 的逻辑是：

- 没记录：插入一条带 `LockTime` 的新记录
- 有记录但锁未过期：返回 `ErrDBRecordLocked`
- 有记录且锁过期：更新 `LockTime`，重新抢占

### 5.2 处理锁竞争

如果拿锁失败且原因是“已被占用”，`Manager.doActivate()` 会调用：

- `WaitForActivation()`

它会按几个固定退避时间轮询存储，等待其他节点把 PID 写回。

### 5.3 选择目标节点

选择规则：

1. 优先复用记录中的 `memberID`
2. 如果该 member 已不存在，则从支持该 kind 的节点中随机选

### 5.4 发起远程激活

`Manager.spawnActor()` 会：

1. 计算目标节点上的 activator PID
2. 发送 `ActivationRequest`
3. 等待 `ActivationResponse`

### 5.5 本地真正 Spawn

目标节点上的 `placementActor` 收到请求后：

1. 查本地内存 `actors`，若已经创建则直接复用
2. 找到 cluster kind
3. 用 `WithClusterIdentity` 包装 props
4. `SpawnPrefix(...)`
5. 本地记录 `identity -> PID`
6. 调用 `Manager.SavePid(...)` 回写存储

至此激活完成。

## 6. 手动激活阶段

手动激活的生命周期和自动激活几乎一致，但入口不同。

调用链：

1. 业务调用 `Activate(clusterIdentity)`
2. `Manager.Activate()`
3. `doActivate(...)`
4. 远程消息发给 `storage-manual-proxy`
5. `manualProxyActor` 转发成 `ManualActivateRequest`
6. `placementActor` 执行真正创建

### 设计上的语义

manual kind 生命周期是：

- 默认不自动创建
- 需要业务明确声明“现在可以启动”
- 一旦创建完成，之后普通调用可以复用存量 PID

## 7. 运行中终止阶段

当 grain actor 自然退出或被 `Poison` 后：

1. `placementActor` 收到 `*actor.Terminated`
2. 广播 `ActivationTerminated`
3. 调用 `RemovePid`
4. 存储把 `TerminateTime` 写为当前时间
5. 其他节点在 `onActivationTerminated()` 中清理本地 `PidCache`

### 为什么不是删除记录

当前实现选择“标记终止”而不是“删除记录”，这样可以保留：

- 曾经的 member 归属
- 最近的终止时间
- 版本演进基础

后续如果终止时间过久，存储层允许不再强制复用原 member。

## 8. 删除失败补偿阶段

如果 `RemoveActivation()` 失败：

- `Manager.RemovePid()` 会把动作塞进 `failedRemoveAction`
- ticker 周期性执行 `HandleFailRemoveAction()`
- 再次尝试删除/标记终止

这是一种简单的异步补偿。

### 当前限制

补偿没有持久化，进程退出就丢。

## 9. 关闭阶段

`StorageIdentityLookup.Shutdown()` 会调用 `Manager.Stop()`：

1. 取消 EventStream 订阅
2. 停掉补偿 ticker
3. `Poison` manual proxy actor
4. `Poison` placement actor

而 `placementActor` 在 `Stopping` 时会进一步：

1. 遍历当前所有子 actor
2. 逐个 `Poison`
3. 等待它们结束

## 10. 生命周期总结

可以把这套生命周期概括为：

1. `Setup`
2. `Start actors + topology watch`
3. `Get/Activate`
4. `Storage lock`
5. `Remote spawn`
6. `Store PID`
7. `Use PID`
8. `Terminate`
9. `Mark terminated`
10. `Shutdown`

## 11. 对接入方最重要的规则

### 普通 kind

- 直接访问即可
- 首次访问可能触发远程激活

### manual kind

- 先 `Activate`
- 再让业务流量进入

### 存储要求

- 必须支持按 identity 唯一定位
- 必须支持带版本或锁语义的更新
- 必须允许等待其他节点完成激活后再读取结果
