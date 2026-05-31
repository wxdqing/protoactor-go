package identitylookup

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gitee.com/wxdqing/identitylookup/errorx"
	"gitee.com/wxdqing/identitylookup/member_strategy"
	"gitee.com/wxdqing/identitylookup/types"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	clustering "github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/eventstream"
)

const (
	StorageActivatorActorName   = "storage-activator"
	StorageManualProxyActorName = "storage-manual-proxy"
)

type RemoveAction struct {
	ClusterIdentity *cluster.ClusterIdentity
	Pid             *actor.PID
}

type Manager struct {
	cluster                        *clustering.Cluster
	placementActor                 *actor.PID
	manualProxyActor               *actor.PID
	storage                        IdentityStorage
	topologySub                    *eventstream.Subscription
	memberStrategy                 MemberStrategy
	failedRemoveAction             *sync.Map
	handleFailRemoveActionCount    int
	handleFailRemoveActionInterval time.Duration
	ticker                         *time.Ticker
	tickerDone                     chan struct{}

	manualKinds map[string]bool
}

func newStorageManager(c *clustering.Cluster, storage IdentityStorage, manualKinds []string) *Manager {
	kindsMap := make(map[string]bool)
	for _, kind := range manualKinds {
		kindsMap[kind] = true
	}

	return &Manager{
		cluster:                        c,
		storage:                        storage,
		memberStrategy:                 member_strategy.NewDefaultMemberStrategy(c),
		manualKinds:                    kindsMap,
		failedRemoveAction:             &sync.Map{},
		handleFailRemoveActionCount:    10,
		handleFailRemoveActionInterval: time.Minute,
	}
}

func (pm *Manager) Start() {
	pm.cluster.Logger().Info("Started storage manager")
	system := pm.cluster.ActorSystem

	activatorProps := actor.PropsFromProducer(func() actor.Actor { return newPlacementActor(pm.cluster, pm) })
	pm.placementActor, _ = system.Root.SpawnNamed(activatorProps, StorageActivatorActorName)
	pm.cluster.Logger().Info("Started storage placement actor")

	proxyProps := actor.PropsFromProducer(func() actor.Actor { return newManualProxyActor(pm.cluster, pm.placementActor) })
	pm.manualProxyActor, _ = system.Root.SpawnNamed(proxyProps, StorageManualProxyActorName)
	pm.cluster.Logger().Info("Started storage manual proxy actor")

	pm.ticker = time.NewTicker(pm.handleFailRemoveActionInterval)
	pm.tickerDone = make(chan struct{})
	go func(pm *Manager) {
		pm.cluster.Logger().Info("Start Handle Fail RemoveAction Ticker")
		for {
			select {
			case <-pm.ticker.C:
				pm.HandleFailRemoveAction()
			case <-pm.tickerDone:
				pm.cluster.Logger().Info("Stop Handle Fail RemoveAction Ticker")
				return
			}
		}
	}(pm)

	pm.topologySub = system.EventStream.
		Subscribe(func(ev interface{}) {
			switch msg := ev.(type) {
			case *clustering.ClusterTopology:
				pm.onClusterTopology(msg)
			case *clustering.ActivationTerminated:
				pm.onActivationTerminated(msg)
			}
		})
}

func (pm *Manager) HandleFailRemoveAction() {
	handleList := make([]*RemoveAction, 0)
	pm.failedRemoveAction.Range(func(key, value any) bool {
		ra := value.(*RemoveAction)
		handleList = append(handleList, ra)
		return len(handleList) < pm.handleFailRemoveActionCount
	})

	if len(handleList) == 0 {
		pm.cluster.Logger().Debug("HandleFailRemoveAction No Fail Action")
		return
	}

	for _, v := range handleList {
		pm.cluster.Logger().Info("HandleFailRemoveAction", slog.Any("RemoveAction", v))
		// RemovePid 内部会在失败时重新加入失败队列，这里先删除旧记录，避免重复执行 removePid。
		pm.failedRemoveAction.Delete(v.ClusterIdentity.AsKey())
		pm.RemovePid(v.ClusterIdentity, v.Pid)
	}
}

func (pm *Manager) stopTicker() {
	if pm.ticker == nil {
		return
	}

	pm.ticker.Stop()
	close(pm.tickerDone)
	pm.ticker = nil
}

func (pm *Manager) Stop() {
	system := pm.cluster.ActorSystem
	system.EventStream.Unsubscribe(pm.topologySub)
	pm.stopTicker()
	err := system.Root.PoisonFuture(pm.manualProxyActor).Wait()
	if err != nil {
		pm.cluster.Logger().Error("Failed to shutdown partition manual proxy actor", slog.Any("error", err))
	}

	err = system.Root.PoisonFuture(pm.placementActor).Wait()
	if err != nil {
		pm.cluster.Logger().Error("Failed to shutdown partition placement actor", slog.Any("error", err))
	}

	pm.cluster.Logger().Info("Stopped storage manager")
}

func (pm *Manager) PidOfActivatorActor(addr string) *actor.PID {
	return actor.NewPID(addr, StorageActivatorActorName)
}

func (pm *Manager) getPidFromActivation(activation *types.StoredActivation) *actor.PID {
	if activation.Pid == nil {
		return nil
	}

	memberName, epoch := ExtractSystemID(activation.MemberID)
	member := pm.memberStrategy.GetMemberByName(memberName)
	if member == nil {
		pm.cluster.ActorSystem.Logger().Warn("can't find member", slog.Any("member", activation.MemberID))
		return nil
	}

	if member.Epoch != epoch {
		pm.cluster.ActorSystem.Logger().Warn("member epoch not equal", slog.Uint64("epoch", member.Epoch),
			slog.Any("member", activation.MemberID))
		return nil
	}

	return activation.Pid
}

func (pm *Manager) Get(clusterIdentity *clustering.ClusterIdentity) *actor.PID {
	activation, err := pm.storage.TryGetExistingActivation(clusterIdentity)
	if err != nil {
		pm.cluster.ActorSystem.Logger().Warn("Failed to get pid from storage", slog.Any("error", err))
		return nil
	}

	if activation.Pid != nil {
		pid := pm.getPidFromActivation(activation)
		if pid != nil {
			return pid
		}
	}

	if pm.manualKinds[clusterIdentity.Kind] {
		// manual kind 需要手动调用 Activate 创建。
		pm.cluster.Logger().Info("get manual kind pid fail", slog.Any("clusterIdentity", clusterIdentity))
		return nil
	}

	return pm.doActivate(clusterIdentity, StorageActivatorActorName)
}

func (pm *Manager) Activate(clusterIdentity *clustering.ClusterIdentity) *actor.PID {
	activation, err := pm.storage.TryGetExistingActivation(clusterIdentity)
	if err != nil {
		pm.cluster.ActorSystem.Logger().Warn("Failed to get pid from storage", slog.Any("error", err))
		return nil
	}

	if activation.Pid != nil {
		pid := pm.getPidFromActivation(activation)
		if pid != nil {
			return pid
		}
	}

	return pm.doActivate(clusterIdentity, StorageManualProxyActorName)
}

func (pm *Manager) RemovePid(clusterIdentity *clustering.ClusterIdentity, pid *actor.PID) {
	pm.cluster.Logger().Info("RemovePid", slog.Any("clusterIdentity", clusterIdentity), slog.Any("pid", pid))
	if err := pm.storage.RemoveActivation(clusterIdentity, pid); err != nil {
		if errors.Is(err, errorx.ErrActorRouterNotFound) || errors.Is(err, errorx.ErrActorRouterPidNotMatch) {
			pm.cluster.Logger().Error("RemovePid Fail With Router Not Match", slog.Any("error", err))
		} else {
			// 删除失败时记录补偿动作，后续由定时任务重试。
			pm.failedRemoveAction.Store(clusterIdentity.AsKey(), &RemoveAction{
				Pid:             pid,
				ClusterIdentity: clusterIdentity,
			})

			if errors.Is(err, errorx.ErrDBOperation) {
				pm.cluster.Logger().Error("RemovePid Fail With DB Error", slog.Any("error", err))
			} else {
				pm.cluster.Logger().Error("RemovePid Fail With Unknown Error", slog.Any("error", err))
			}
		}
	}
}
func (pm *Manager) doActivate(clusterIdentity *clustering.ClusterIdentity, activatorName string) *actor.PID {
	pm.cluster.Logger().Info("Activating", slog.Any("clusterIdentity", clusterIdentity))

	spawnLock, memberID, err := pm.storage.TryAcquireLock(clusterIdentity)
	if err != nil {
		if errors.Is(err, errorx.ErrDBRecordLocked) {
			pm.cluster.Logger().Info("wait for activation", slog.Any("clusterIdentity", clusterIdentity))
			activation, err := pm.storage.WaitForActivation(clusterIdentity)
			if err != nil {
				pm.cluster.Logger().Error("Failed waiting for router activation", slog.Any("error", err))
				return nil
			}

			return activation.Pid
		} else {
			pm.cluster.Logger().Error("activate fail", slog.Any("error", err))
		}
		return nil
	}

	member := pm.selectMember(clusterIdentity, memberID)
	if member == nil {
		pm.cluster.Logger().Error("Failed to select member", slog.Any("kind", clusterIdentity.Kind))
		return nil
	}

	return pm.spawnActor(clusterIdentity, member, spawnLock, activatorName)
}

func (pm *Manager) spawnActor(clusterIdentity *clustering.ClusterIdentity, member *types.Member, spawnLock *types.SpawnLock, activatorName string) *actor.PID {
	pm.cluster.Logger().Info("Spawning", slog.Any("clusterIdentity", clusterIdentity), slog.Any("member", member))

	identityOwnerPid := &actor.PID{
		Address: member.Address(),
		Id:      activatorName,
	}

	request := &clustering.ActivationRequest{
		ClusterIdentity: clusterIdentity,
		RequestId:       spawnLock.LockID,
	}
	future := pm.cluster.ActorSystem.Root.RequestFuture(identityOwnerPid, request, 5*time.Second)
	res, err := future.Result()
	if err != nil {
		pm.cluster.Logger().Info("Spawning Failed", slog.Any("clusterIdentity", clusterIdentity), slog.Any("member", member))
		return nil
	}
	typed, ok := res.(*clustering.ActivationResponse)
	if !ok {
		pm.cluster.Logger().Info("Spawning Failed", slog.Any("clusterIdentity", clusterIdentity), slog.Any("member", member))
		return nil
	}
	return typed.Pid
}

func (pm *Manager) selectMember(clusterIdentity *cluster.ClusterIdentity, memberID string) *types.Member {
	if memberID != "" {
		memberName, _ := ExtractSystemID(memberID)
		pm.cluster.Logger().Info("router table member", slog.Any("member_id", memberID))
		// 优先尝试复用路由表中记录的 member。
		member := pm.memberStrategy.GetMemberByName(memberName)
		if member != nil {
			return member
		}

		pm.cluster.Logger().Warn("Failed to get member", slog.Any("member_id", memberID))
	}

	member := pm.memberStrategy.GetActivator(clusterIdentity)
	if member == nil {
		pm.cluster.Logger().Error("Failed to get activator", slog.Any("kind", clusterIdentity.Kind))
		return nil
	}

	return member
}

func (pm *Manager) onClusterTopology(tplg *clustering.ClusterTopology) {
	pm.cluster.Logger().Info("onClusterTopology", slog.Uint64("topology-hash", tplg.TopologyHash))
	// MemberID 中保存的是去掉 cluster 前缀后的成员标识。
	newMembers := make([]*types.Member, len(tplg.Members))
	for index, member := range tplg.Members {
		memberID, _ := strings.CutPrefix(member.Id, fmt.Sprintf("%s@", pm.cluster.Config.Name))
		memberName, epoch := ExtractSystemID(memberID)
		newMember := &types.Member{
			Member: cluster.Member{
				Host:  member.Host,
				Kinds: member.Kinds,
				Port:  member.Port,
				Id:    memberID,
			},
			Name:  memberName,
			Epoch: epoch,
		}
		newMembers[index] = newMember

		pm.cluster.Logger().Info("Got member", slog.Any("member", newMember), slog.String("name", memberName), slog.Uint64("epoch", epoch))
	}

	pm.memberStrategy.UpdateMember(newMembers)
}

func (pm *Manager) onActivationTerminated(msg *clustering.ActivationTerminated) {
	pm.cluster.Logger().Info("onActivationTerminated", slog.Any("identity", msg.ClusterIdentity))
	pm.cluster.PidCache.RemoveByValue(msg.ClusterIdentity.Identity, msg.ClusterIdentity.Kind, msg.Pid)
}

func (pm *Manager) SavePid(clusterIdentity *clustering.ClusterIdentity, pid *actor.PID, lockID string) error {
	memberID := pm.cluster.ActorSystem.ID
	pm.cluster.Logger().Info("SavePid", slog.Any("Address", pid.Address), slog.Any("ActorID", pid.Id), slog.Any("memberID", memberID))

	spawnLock := &types.SpawnLock{
		LockID:          lockID,
		ClusterIdentity: clusterIdentity,
	}
	if err := pm.storage.StoreActivation(memberID, spawnLock, pid); err != nil {
		return fmt.Errorf("bind identity:%s pid:%s %s failed: %w", clusterIdentity.AsKey(), pid.Address, pid.Id, err)
	}

	return nil
}

func (pm *Manager) IsManualKind(kind string) bool {
	return pm.manualKinds[kind]
}
