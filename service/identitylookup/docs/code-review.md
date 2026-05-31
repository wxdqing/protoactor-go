# identitylookup 代码审查

## 结论

这个库的核心设计思路是成立的：它试图为 Proto.Actor Cluster 提供一个基于外部存储的 identity 路由与激活协调层。

但当前实现并不处于“可放心交付”的状态。除了代码说明层面的可读性问题，已经存在实际的编译风险、测试缺失和并发/生命周期风险。

## 发现

### 1. `placement_actor.go` 导入路径错误，当前仓库无法稳定编译

文件：

- `placement_actor.go:8`

问题：

- 该文件导入的是 `rmini-migration/internal/go-sprout/identitylookup/types`
- 其他文件使用的是 `gitee.com/wxdqing/identitylookup/types`

影响：

- 这是明显的残留路径污染
- 在正常模块环境下会直接导致构建失败或依赖错位

严重性：

- 高

### 2. `system_id_test.go` 不会被 Go 测试框架执行

文件：

- `system_id_test.go:5`

问题：

- 测试函数名是 `SystemIDTest`
- Go 只会执行 `TestXxx` 形式的测试函数

影响：

- 当前唯一可见的单测实际上没有运行
- `ExtractSystemID()` 的行为没有被自动验证

严重性：

- 高

### 3. `StorageIdentityLookup.Setup()` 忽略 `kinds` 与 `isClient` 入参，接口适配不完整

文件：

- `identity_lookup.go:35`

问题：

- `Setup(cluster, kinds, isClient)` 中完全忽略了 `kinds` 和 `isClient`

影响：

- 作为 Proto.Actor `IdentityLookup` 的实现，这里没有体现 client/member 角色差异
- 如果未来 client 节点也走同一路径，行为边界会变得不明确

严重性：

- 中

### 4. 激活失败后锁释放依赖超时，没有显式回滚路径

文件：

- `manager.go:235`
- `identity_storage.go:69`

问题：

- `TryAcquireLock()` 成功后，如果 `selectMember()` 或 `spawnActor()` 失败，没有显式清锁
- 只能等 `maxLockTime` 超时后再被其他节点抢占

影响：

- 会把一次瞬时故障放大成一个至少 15 秒的 identity 激活不可用窗口

严重性：

- 中

### 5. 失败补偿只在内存中维护，进程退出会丢失

文件：

- `manager.go:102`
- `manager.go:215`

问题：

- 删除失败动作只放进 `sync.Map`
- 没有落盘，没有幂等日志，没有重放恢复

影响：

- 如果节点在补偿前退出，存储中的终止标记可能长期不一致
- 可能拖慢后续重新激活

严重性：

- 中

### 6. `WaitForActivation()` 只做短轮询，超过窗口后直接返回空结果

文件：

- `identity_storage.go:109`

问题：

- 等待窗口只有 `50ms + 100ms + 500ms + 1000ms`
- 如果对方激活耗时更长，调用者会拿到空结果

影响：

- 对于初始化慢的 grain，这个等待策略可能过短
- 示例里的 grain `Init()` 就显式 sleep 了 1 秒

严重性：

- 中

### 7. 代码中存在乱码注释，降低维护性

文件：

- `manager.go:117`
- `manager.go:190`
- `errorx/error.go:6`
- `tcaplus_storage/tcaplus_table.go:31`

问题：

- 注释编码已经损坏

影响：

- 维护者无法从注释中恢复原始设计意图
- 容易误导后续修复

严重性：

- 低

## 验证情况

我执行了：

```powershell
go test ./...
```

结果：

- 失败
- 同时暴露出 `go build cache access denied`
- 以及多处 `missing go.sum entry for module providing package github.com/asynkron/protoactor-go/...`

这说明当前库至少在本环境下不是开箱即测的状态。

## 建议优先级

建议修复顺序：

1. 统一导入路径，先恢复基本编译能力
2. 修正测试命名并补最小单测
3. 给激活失败补显式清锁或快速回滚
4. 重新审视 `WaitForActivation()` 的等待窗口
5. 把乱码注释替换为可读中文或英文
