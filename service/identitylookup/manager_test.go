package identitylookup

import (
	"testing"

	"github.com/asynkron/protoactor-go/service/cluster"
	"github.com/asynkron/protoactor-go/service/identitylookup/types"
)

type recordingStaticRouter struct {
	placementContext *cluster.PlacementContext
	clusterIdentity  *cluster.ClusterIdentity
	members          cluster.Members
	member           *cluster.Member
	ok               bool
}

func (r *recordingStaticRouter) Route(placementContext *cluster.PlacementContext, clusterIdentity *cluster.ClusterIdentity, members cluster.Members) (*cluster.Member, bool) {
	r.placementContext = placementContext
	r.clusterIdentity = clusterIdentity
	r.members = members
	return r.member, r.ok
}

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

func TestSelectMemberUsesStaticRouterWhenConfigured(t *testing.T) {
	placementContext := &cluster.PlacementContext{NodeType: "game", RouteKey: 42}
	clusterIdentity := cluster.NewClusterIdentity("player-1", "player")
	routedMember := &cluster.Member{
		Name:  "node-a",
		Host:  "127.0.0.1",
		Port:  12001,
		Kinds: []string{"player"},
		Id:    "node-a-epoch-2",
	}

	router := &recordingStaticRouter{member: routedMember, ok: true}
	manager := &Manager{
		cluster: &cluster.Cluster{Config: &cluster.Config{StaticRouter: router}},
		members: cluster.Members{
			routedMember,
		},
		memberStrategy: &recordingMemberStrategy{},
	}

	got := manager.selectMember(placementContext, clusterIdentity, "")

	if got == nil || got.Name != "node-a" || got.Port != 12001 {
		t.Fatalf("selectMember() = %#v, want node-a member", got)
	}
	if router.placementContext != placementContext {
		t.Fatalf("static router did not receive placement context")
	}
	if router.clusterIdentity != clusterIdentity {
		t.Fatalf("static router did not receive cluster identity")
	}
}

func TestSelectMemberReturnsNilWhenStaticRouterMisses(t *testing.T) {
	clusterIdentity := cluster.NewClusterIdentity("player-1", "player")
	strategy := &recordingMemberStrategy{activator: &types.Member{Name: "fallback"}}
	manager := &Manager{
		cluster:        &cluster.Cluster{Config: &cluster.Config{StaticRouter: &recordingStaticRouter{ok: false}}},
		memberStrategy: strategy,
	}

	got := manager.selectMember(&cluster.PlacementContext{NodeType: "game", RouteKey: 42}, clusterIdentity, "")

	if got != nil {
		t.Fatalf("selectMember() = %#v, want nil", got)
	}
	if strategy.getActivatorCalls != 0 {
		t.Fatalf("GetActivator called %d times, want 0", strategy.getActivatorCalls)
	}
}

func TestSelectMemberUsesDefaultStrategyWhenStaticRouterIsNotConfigured(t *testing.T) {
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
