# identitylookup 概览

## 它是做什么的

`identitylookup` 是一个基于 Proto.Actor Cluster 的自定义 `IdentityLookup` 实现。它的职责不是承载业务 grain 逻辑，而是负责把：

- `ClusterIdentity(kind, identity)`

映射为：

- 当前可用的 `PID`
- 对应 actor 所属的 `MemberID`

同时，这个映射不是纯内存态，而是会落到外部存储中，以便多个集群节点对同一个 identity 的激活过程进行协调。

## 它解决的问题

Proto.Actor 默认会面临一个典型问题：多个节点可能同时尝试激活同一个虚拟 actor。这个库通过“持久化记录 + 锁 + 成员校验”的方式解决几个关键问题：

1. 同一个 `kind/identity` 尽量只由一个节点完成激活。
2. 其他节点可以从存储中找到已经激活好的 PID，而不是重复创建。
3. 节点重启后，旧 PID 不会因为缓存残留而被误认为有效。
4. actor 结束后，可以把路由记录标记为终止状态，允许后续重新激活。

## 核心能力

### 1. 持久化 identity -> PID

通过 `IdentityStorage` 抽象把映射存到 Redis 或 TcaplusDB。

### 2. 分布式激活互斥

通过 `TryAcquireLock` 与版本/记录锁语义，避免并发重复激活。

### 3. 自动激活与手动激活并存

- 普通 kind：`Get` 找不到时自动激活
- manual kind：`Get` 只查不建，必须显式调用 `Activate`

### 4. 节点有效性校验

存储中除了 PID，还保存 `MemberID`。读取旧记录时，会对 `member name + epoch` 做校验，防止把旧节点的 PID 当成存活对象。

## 不负责什么

这个库不负责：

- grain 业务逻辑
- cluster provider 的成员发现实现
- 存储底层的高可用与迁移
- actor 状态持久化

它只负责“身份定位”和“激活协调”。

## 典型使用方式

示例代码见 `example/lookup/lookup.go:1`：

1. 构造 Redis 或 Tcaplus 的 `IdentityStorage`
2. 通过 `NewStorageIdentityLookup(storage, manualKinds)` 创建 lookup
3. 在 `cluster.Configure(...)` 中把该 lookup 注册给 Proto.Actor Cluster
4. 普通 grain 直接通过 grain client 调用
5. manual grain 先 `Activate(...)`，再由客户端访问

## 一句话总结

这个库本质上是：

“给 Proto.Actor Cluster 增加一个依赖外部存储的分布式 identity 路由层，用来稳定地查找、激活、校验和回收虚拟 actor。”
