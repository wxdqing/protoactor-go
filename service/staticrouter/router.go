// Package staticrouter adapts staticrouter to service cluster placement.
package staticrouter

import (
	"math"

	cluster "github.com/asynkron/protoactor-go/service/cluster"
	"github.com/wxdqing/staticrouter/model"
)

// Router is the staticrouter lookup behavior required by this adapter.
type Router interface {
	Get(routeContext *model.RouteContext) (*model.RouteRecord, bool)
}

// StaticRouter adapts a staticrouter router to cluster.StaticRouter.
type StaticRouter struct {
	router Router
}

var _ cluster.StaticRouter = (*StaticRouter)(nil)

// New creates a StaticRouter adapter around a staticrouter lookup.
func New(router Router) *StaticRouter {
	return &StaticRouter{router: router}
}

// Route resolves the placement context through staticrouter and maps the result to a cluster member.
func (r *StaticRouter) Route(placementContext *cluster.PlacementContext, clusterIdentity *cluster.ClusterIdentity, members cluster.Members) (*cluster.Member, bool) {
	if r == nil || r.router == nil || placementContext == nil || clusterIdentity == nil {
		return nil, false
	}
	if placementContext.RouteKey > math.MaxInt32 {
		return nil, false
	}

	route, ok := r.router.Get(&model.RouteContext{
		Kind:     clusterIdentity.Kind,
		NodeType: placementContext.NodeType,
		RouteKey: int32(placementContext.RouteKey),
	})
	if !ok || route == nil {
		return nil, false
	}

	return findMemberByBaseName(members, route.GetNodeId())
}

func findMemberByBaseName(members cluster.Members, baseName string) (*cluster.Member, bool) {
	if baseName == "" {
		return nil, false
	}
	fitEpoch := uint64(0)
	var retMember *cluster.Member
	for _, member := range members {
		if member != nil && member.Name == baseName {
			_, epoch, ok := cluster.ParseMemberID(member.Id)
			if !ok && retMember == nil {
				retMember = member
				continue
			}
			if epoch > fitEpoch {
				retMember = member
				fitEpoch = epoch
			}
		}
	}
	return retMember, retMember != nil
}
