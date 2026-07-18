package identitylookup

import (
	"testing"

	"github.com/asynkron/protoactor-go/service/cluster"
	"github.com/asynkron/protoactor-go/service/identitylookup/types"
)

type recordingMemberStrategy struct {
	memberByName      *types.Member
	activator         *types.Member
	getActivatorCalls int
}

func (r *recordingMemberStrategy) GetMemberByName(memberName string) *types.Member {
	return r.memberByName
}

func (r *recordingMemberStrategy) GetActivator(clusterIdentity *cluster.ClusterIdentity) *types.Member {
	r.getActivatorCalls++
	return r.activator
}

func (r *recordingMemberStrategy) UpdateMember(members []*types.Member) {}

func TestSelectMemberUsesDefaultStrategy(t *testing.T) {
	clusterIdentity := cluster.NewClusterIdentity("player-1", "player")
	expected := &types.Member{Name: "default-node"}
	strategy := &recordingMemberStrategy{activator: expected}
	manager := &Manager{
		cluster:        &cluster.Cluster{Config: &cluster.Config{}},
		memberStrategy: strategy,
	}

	got := manager.selectMember(&cluster.PlacementContext{NodeType: "game", RouteKey: 42}, clusterIdentity, "")

	if got != expected {
		t.Fatalf("selectMember() = %#v, want default strategy member", got)
	}
	if strategy.getActivatorCalls != 1 {
		t.Fatalf("GetActivator called %d times, want 1", strategy.getActivatorCalls)
	}
}
