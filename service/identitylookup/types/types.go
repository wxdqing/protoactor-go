package types

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/service/cluster"
)

type SpawnLock struct {
	LockID          string
	ClusterIdentity *cluster.ClusterIdentity
}

type StoredActivation struct {
	Pid      *actor.PID
	MemberID string
}

type Member struct {
	cluster.Member
	Name  string
	Epoch uint64
}
