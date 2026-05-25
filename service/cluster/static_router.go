package cluster

// StaticRouter selects a cluster member using explicit placement context.
type StaticRouter interface {
	Route(placementContext *PlacementContext, clusterIdentity *ClusterIdentity, members Members) (*Member, bool)
}
