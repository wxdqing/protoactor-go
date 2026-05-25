package cluster

// PlacementContext carries explicit placement and routing inputs for identity lookup.
type PlacementContext struct {
	NodeType   string
	RouteKey   uint64
	Affinity   string
	ForceLocal bool
	Labels     map[string]string
}
