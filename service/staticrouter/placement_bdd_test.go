package staticrouter

import (
	"context"
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	cluster "github.com/asynkron/protoactor-go/service/cluster"
	staticrouter "github.com/wxdqing/staticrouter"
)

type recordingIdentityLookup struct {
	placementContext *cluster.PlacementContext
	clusterIdentity  *cluster.ClusterIdentity
}

func (l *recordingIdentityLookup) Get(placementContext *cluster.PlacementContext, clusterIdentity *cluster.ClusterIdentity) *actor.PID {
	l.placementContext = placementContext
	l.clusterIdentity = clusterIdentity
	return nil
}

func (l *recordingIdentityLookup) RemovePid(_ *cluster.PlacementContext, _ *cluster.ClusterIdentity, _ *actor.PID) {
}

func (l *recordingIdentityLookup) Setup(_ *cluster.Cluster, _ []string, _ bool) {}

func (l *recordingIdentityLookup) Shutdown() {}

// TestBDDGivenPeerPlacementContextWhenClusterAndStaticRouterRouteThenSameGameNodeIsSelected
// verifies the placement contract shared by peer gateway routing and cluster identity lookup.
func TestBDDGivenPeerPlacementContextWhenClusterAndStaticRouterRouteThenSameGameNodeIsSelected(t *testing.T) {
	staticRouter := staticrouter.NewRouter(nil)
	if err := staticRouter.ReplaceAll(context.Background(), &staticrouter.RouteSnapshot{
		Routes: []*staticrouter.RouteRecord{
			{
				Kind:      "player_equip",
				NodeType:  "game",
				RouteKeys: []int32{1001},
				NodeId:    "game-1",
			},
		},
	}); err != nil {
		t.Fatalf("ReplaceAll() error = %v", err)
	}

	router := New(staticRouter)
	placementContext := &cluster.PlacementContext{
		NodeType: "game",
		RouteKey: 1001,
		Labels: map[string]string{
			"target_name": "game-1",
		},
	}
	clusterIdentity := cluster.NewClusterIdentity("account:demo-user", "player_equip")
	members := cluster.Members{
		{Id: "gateway-1", Name: "gateway-1", Kinds: []string{"gateway"}},
		{Id: "game-1", Name: "game-1", Kinds: []string{"player_equip"}},
	}

	member, ok := router.Route(placementContext, clusterIdentity, members)
	if !ok {
		t.Fatal("Route() ok = false, want true")
	}
	if member.Name != "game-1" {
		t.Fatalf("Route() member = %q, want game-1", member.Name)
	}

	lookup := &recordingIdentityLookup{}
	c := &cluster.Cluster{
		Config:         &cluster.Config{StaticRouter: router},
		IdentityLookup: lookup,
	}
	pid := c.Get(placementContext, clusterIdentity.Identity, clusterIdentity.Kind)
	if pid != nil {
		t.Fatalf("Get() pid = %v, want nil without activation", pid)
	}
	if lookup.placementContext != placementContext {
		t.Fatal("cluster.Get did not pass placement context to identity lookup")
	}
	if lookup.clusterIdentity.Identity != clusterIdentity.Identity || lookup.clusterIdentity.Kind != clusterIdentity.Kind {
		t.Fatalf("cluster.Get identity = %#v, want %#v", lookup.clusterIdentity, clusterIdentity)
	}
}
