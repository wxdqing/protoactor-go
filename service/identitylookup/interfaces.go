package identitylookup

import (
	"gitee.com/wxdqing/identitylookup/types"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
)

type IdentityActivator interface {
	Activate(clusterIdentity *cluster.ClusterIdentity) *actor.PID
}

type IdentityStorage interface {
	TryGetExistingActivation(clusterIdentity *cluster.ClusterIdentity) (*types.StoredActivation, error)

	TryAcquireLock(clusterIdentity *cluster.ClusterIdentity) (*types.SpawnLock, string, error)

	WaitForActivation(clusterIdentity *cluster.ClusterIdentity) (*types.StoredActivation, error)

	// RemoveLock(spawnLock SpawnLock)

	StoreActivation(memberID string, spawnLock *types.SpawnLock, pid *actor.PID) error

	RemoveActivation(clusterIdentity *cluster.ClusterIdentity, pid *actor.PID) error

	// RemoveMemberId(memberID string)
}

type MemberStrategy interface {
	GetMemberByName(memberName string) *types.Member
	GetActivator(clusterIdentity *cluster.ClusterIdentity) *types.Member
	UpdateMember(members []*types.Member)
}
