package cluster

import (
	"sync"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/require"
)

func TestPlacementContextCarriesPlacementParameters(t *testing.T) {
	placementContext := &PlacementContext{
		NodeType:   "edge",
		RouteKey:   42,
		Affinity:   "tenant-a",
		ForceLocal: true,
		Labels: map[string]string{
			"region": "ap-east",
		},
	}

	require.Equal(t, "edge", placementContext.NodeType)
	require.Equal(t, uint64(42), placementContext.RouteKey)
	require.Equal(t, "tenant-a", placementContext.Affinity)
	require.True(t, placementContext.ForceLocal)
	require.Equal(t, "ap-east", placementContext.Labels["region"])
}

type recordingIdentityLookup struct {
	placementContext *PlacementContext
	clusterIdentity  *ClusterIdentity
	pid              *actor.PID
	mu               sync.Mutex
}

func (l *recordingIdentityLookup) Get(placementContext *PlacementContext, clusterIdentity *ClusterIdentity) *actor.PID {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.placementContext = placementContext
	l.clusterIdentity = clusterIdentity

	return l.pid
}

func (l *recordingIdentityLookup) RemovePid(_ *PlacementContext, _ *ClusterIdentity, _ *actor.PID) {
}

func (l *recordingIdentityLookup) Setup(_ *Cluster, _ []string, _ bool) {
}

func (l *recordingIdentityLookup) Shutdown() {
}

func TestClusterGetPassesPlacementContextToIdentityLookup(t *testing.T) {
	lookup := &recordingIdentityLookup{pid: actor.NewPID("127.0.0.1:1", "target")}
	c := &Cluster{IdentityLookup: lookup}
	placementContext := &PlacementContext{NodeType: "edge", RouteKey: 7}

	pid := c.Get(placementContext, "identity", "kind")

	require.Same(t, lookup.pid, pid)
	require.Same(t, placementContext, lookup.placementContext)
	require.Equal(t, "identity", lookup.clusterIdentity.Identity)
	require.Equal(t, "kind", lookup.clusterIdentity.Kind)
}

func TestClusterRequestPassesPlacementContextToIdentityLookup(t *testing.T) {
	lookup := &recordingIdentityLookup{}
	c := &Cluster{
		ActorSystem: &actor.ActorSystem{
			Config: &actor.Config{},
		},
		Config: &Config{
			RequestTimeoutTime:        time.Second,
			RequestsLogThrottlePeriod: time.Second,
		},
		IdentityLookup: lookup,
		PidCache:       NewPidCache(),
	}
	c.context = newDefaultClusterContext(c)
	placementContext := &PlacementContext{NodeType: "worker", RouteKey: 99}

	_, err := c.Request(placementContext, "identity", "kind", &struct{}{}, WithRetryCount(1))

	require.ErrorContains(t, err, "max retries")
	require.Same(t, placementContext, lookup.placementContext)
	require.Equal(t, "identity", lookup.clusterIdentity.Identity)
	require.Equal(t, "kind", lookup.clusterIdentity.Kind)
}
