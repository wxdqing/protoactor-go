package staticrouter

import (
	"context"
	"math"
	"testing"

	cluster "github.com/asynkron/protoactor-go/service/cluster"
	staticrouter "github.com/wxdqing/staticrouter"
)

func TestRouterRoutesStaticrouterRecordToClusterMember(t *testing.T) {
	staticRouter := staticrouter.NewRouter(nil)
	err := staticRouter.ReplaceAll(context.Background(), &staticrouter.RouteSnapshot{
		Routes: []*staticrouter.RouteRecord{
			{
				Kind:      "player",
				NodeType:  "game",
				RouteKeys: []int32{42},
				NodeId:    "node-b",
			},
		},
	})
	if err != nil {
		t.Fatalf("ReplaceAll() error = %v", err)
	}

	router := New(staticRouter)
	members := cluster.Members{
		{Id: "node-a", Kinds: []string{"player"}},
		{Id: "node-b", Kinds: []string{"player"}},
	}

	member, ok := router.Route(
		&cluster.PlacementContext{NodeType: "game", RouteKey: 42},
		cluster.NewClusterIdentity("user-42", "player"),
		members,
	)

	if !ok {
		t.Fatalf("Route() ok = false, want true")
	}
	if member != members[1] {
		t.Fatalf("Route() member = %v, want %v", member, members[1])
	}
}

func TestRouterReturnsFalseWhenPlacementContextMissing(t *testing.T) {
	router := New(staticrouter.NewRouter(nil))

	member, ok := router.Route(nil, cluster.NewClusterIdentity("user-42", "player"), nil)

	if ok {
		t.Fatalf("Route() ok = true, want false")
	}
	if member != nil {
		t.Fatalf("Route() member = %v, want nil", member)
	}
}

func TestRouterReturnsFalseWhenRouteKeyOverflowsStaticrouterRange(t *testing.T) {
	router := New(staticrouter.NewRouter(nil))

	member, ok := router.Route(
		&cluster.PlacementContext{NodeType: "game", RouteKey: math.MaxInt32 + 1},
		cluster.NewClusterIdentity("user-42", "player"),
		nil,
	)

	if ok {
		t.Fatalf("Route() ok = true, want false")
	}
	if member != nil {
		t.Fatalf("Route() member = %v, want nil", member)
	}
}

func TestRouterReturnsFalseWhenRoutedNodeIsNotClusterMember(t *testing.T) {
	staticRouter := staticrouter.NewRouter(nil)
	err := staticRouter.ReplaceAll(context.Background(), &staticrouter.RouteSnapshot{
		Routes: []*staticrouter.RouteRecord{
			{
				Kind:      "player",
				NodeType:  "game",
				RouteKeys: []int32{42},
				NodeId:    "node-missing",
			},
		},
	})
	if err != nil {
		t.Fatalf("ReplaceAll() error = %v", err)
	}

	router := New(staticRouter)

	member, ok := router.Route(
		&cluster.PlacementContext{NodeType: "game", RouteKey: 42},
		cluster.NewClusterIdentity("user-42", "player"),
		cluster.Members{{Id: "node-a"}},
	)

	if ok {
		t.Fatalf("Route() ok = true, want false")
	}
	if member != nil {
		t.Fatalf("Route() member = %v, want nil", member)
	}
}
