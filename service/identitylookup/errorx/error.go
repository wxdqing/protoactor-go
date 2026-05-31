package errorx

import "errors"

var (
	ErrPidNotReady = errors.New("pid not ready")
	// 数据版本不一致
	ErrDBInvalidVersion = errors.New("db invalid version")
	// 插入数据时，记录已经存在
	ErrDBRecordExist = errors.New("db record exist")
	// 数据已经被锁定
	ErrDBRecordLocked = errors.New("db record locked")
	// 路由数据被其他节点竞争掉（逻辑BUG或者业务节点被集群剔除）
	ErrActorRouterPidNotMatch = errors.New("actor router pid not match")
	// 路由数据被删除（逻辑BUG或者业务节点被集群剔除）
	ErrActorRouterNotFound = errors.New("actor router not found")
	// DB操作报错
	ErrDBOperation = errors.New("db operation error")
)
