// Package cluster enables distributed actors and grain management.
package cluster

import (
	"log/slog"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/asynkron/gofun/set"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/extensions"
	"github.com/asynkron/protoactor-go/remote"
	clustermetrics "github.com/asynkron/protoactor-go/service/cluster/metrics"
)

var extensionID = extensions.NextExtensionID()

// Cluster manages distributed actor membership, identity lookup, gossip, and pub-sub.
type Cluster struct {
	ActorSystem    *actor.ActorSystem
	Config         *Config
	Gossip         *Gossiper
	PubSub         *PubSub
	Remote         *remote.Remote
	PidCache       *PidCacheValue
	MemberList     *MemberList
	IdentityLookup IdentityLookup
	kinds          map[string]*ActivatedKind
	context        Context

	metrics        *clustermetrics.ClusterMetrics
	metricsEnabled bool
}

var _ extensions.Extension = &Cluster{}

// New creates a cluster extension for the given actor system and config.
func New(actorSystem *actor.ActorSystem, config *Config) *Cluster {
	c := &Cluster{
		ActorSystem: actorSystem,
		Config:      config,
		kinds:       map[string]*ActivatedKind{},
	}
	actorSystem.Extensions.Register(c)

	if actorSystem.Config.MetricsEnabled {
		c.metrics = clustermetrics.NewClusterMetrics(actorSystem.Logger())
		c.metricsEnabled = true
	}

	c.context = config.ClusterContextProducer(c)
	c.PidCache = NewPidCache()
	c.MemberList = NewMemberList(c)
	c.subscribeToTopologyEvents()

	actorSystem.Extensions.Register(c)

	var err error
	c.Gossip, err = newGossiper(c)
	c.PubSub = NewPubSub(c)

	if err != nil {
		panic(err)
	}

	return c
}

func (c *Cluster) subscribeToTopologyEvents() {
	c.ActorSystem.EventStream.Subscribe(func(evt interface{}) {
		if clusterTopology, ok := evt.(*ClusterTopology); ok {
			for _, member := range clusterTopology.Left {
				c.PidCache.RemoveByMember(member)
			}
			if c.metricsEnabled {
				c.metrics.ClusterMembersCount.Set(int64(len(clusterTopology.Members)))
			}
		}
	})
}

// MetricsEnabled reports whether cluster metrics are enabled.
func (c *Cluster) MetricsEnabled() bool { return c.metricsEnabled }

// Metrics returns the cluster metrics instruments.
func (c *Cluster) Metrics() *clustermetrics.ClusterMetrics { return c.metrics }

// VirtualActorCount returns the total number of activated virtual actors.
func (c *Cluster) VirtualActorCount() int64 {
	var total int64
	for _, k := range c.kinds {
		total += int64(k.Count())
	}
	return total
}

// ExtensionID returns the cluster extension ID.
func (c *Cluster) ExtensionID() extensions.ExtensionID {
	return extensionID
}

// GetCluster returns the cluster extension registered in the actor system.
//
//goland:noinspection GoUnusedExportedFunction
func GetCluster(actorSystem *actor.ActorSystem) *Cluster {
	c := actorSystem.Extensions.Get(extensionID)

	return c.(*Cluster)
}

// GetBlockedMembers returns the currently blocked cluster members.
func (c *Cluster) GetBlockedMembers() set.Set[string] {
	return c.Remote.BlockList().BlockedMembers()
}

// StartMember starts this cluster instance as a member.
func (c *Cluster) StartMember() {
	cfg := c.Config
	c.Remote = remote.NewRemote(c.ActorSystem, c.Config.RemoteConfig)

	c.initKinds()

	// TODO: make it possible to become a cluster even if remoting is already started
	c.Remote.Start()

	address := c.ActorSystem.Address()
	c.Logger().Info("Starting Proto.Actor cluster member", slog.String("address", address))

	c.IdentityLookup = cfg.IdentityLookup
	c.IdentityLookup.Setup(c, c.GetClusterKinds(), false)

	// TODO: Disable Gossip for now until API changes are done
	// gossiper must be started whenever any topology events starts flowing
	if err := c.Gossip.StartGossiping(); err != nil {
		panic(err)
	}
	c.PubSub.Start()
	c.MemberList.InitializeTopologyConsensus()

	if err := cfg.ClusterProvider.StartMember(c); err != nil {
		panic(err)
	}

	time.Sleep(1 * time.Second)
}

// GetClusterKinds returns the names of registered cluster kinds.
func (c *Cluster) GetClusterKinds() []string {
	keys := make([]string, 0, len(c.kinds))
	for k := range c.kinds {
		keys = append(keys, k)
	}

	return keys
}

// StartClient starts this cluster instance as a client.
func (c *Cluster) StartClient() {
	cfg := c.Config
	c.Remote = remote.NewRemote(c.ActorSystem, c.Config.RemoteConfig)

	c.Remote.Start()

	address := c.ActorSystem.Address()
	c.Logger().Info("Starting Proto.Actor cluster-client", slog.String("address", address))

	c.IdentityLookup = cfg.IdentityLookup
	c.IdentityLookup.Setup(c, c.GetClusterKinds(), true)

	if err := cfg.ClusterProvider.StartClient(c); err != nil {
		panic(err)
	}
	c.PubSub.Start()
}

// Shutdown stops the cluster.
func (c *Cluster) Shutdown(graceful bool) {
	c.Gossip.SetState(GracefullyLeftKey, &emptypb.Empty{})
	c.ActorSystem.Shutdown()
	if graceful {
		_ = c.Config.ClusterProvider.Shutdown(graceful)
		c.IdentityLookup.Shutdown()
		// This is to wait ownership transferring complete.
		time.Sleep(time.Millisecond * 2000)
		c.MemberList.stopMemberList()
		c.IdentityLookup.Shutdown()
		c.Gossip.Shutdown()
	}

	c.Remote.Shutdown(graceful)

	address := c.ActorSystem.Address()
	c.Logger().Info("Stopped Proto.Actor cluster", slog.String("address", address))
}

// Get resolves the PID for the given identity and kind.
// It returns nil if the kind is not registered or the activation fails.
func (c *Cluster) Get(placementContext *PlacementContext, identity string, kind string) *actor.PID {
	return c.IdentityLookup.Get(placementContext, NewClusterIdentity(identity, kind))
}

// Request sends a message to the virtual actor identified by identity and kind.
func (c *Cluster) Request(placementContext *PlacementContext, identity string, kind string, message interface{}, option ...GrainCallOption) (interface{}, error) {
	return c.context.Request(placementContext, identity, kind, message, option...)
}

// RequestFuture sends a message and returns a future for the response.
func (c *Cluster) RequestFuture(placementContext *PlacementContext, identity string, kind string, message interface{}, option ...GrainCallOption) (actor.Future, error) {
	return c.context.RequestFuture(placementContext, identity, kind, message, option...)
}

// GetClusterKind returns the activated kind for the given name.
func (c *Cluster) GetClusterKind(kind string) *ActivatedKind {
	k, ok := c.kinds[kind]
	if !ok {
		c.Logger().Error("Invalid kind", slog.String("kind", kind))

		return nil
	}

	return k
}

// TryGetClusterKind returns the activated kind for the given name if it exists.
func (c *Cluster) TryGetClusterKind(kind string) (*ActivatedKind, bool) {
	k, ok := c.kinds[kind]

	return k, ok
}

func (c *Cluster) initKinds() {
	for name, kind := range c.Config.Kinds {
		c.kinds[name] = kind.Build(c)
	}
	c.ensureTopicKindRegistered()
}

// ensureTopicKindRegistered ensures that the topic kind is registered in the cluster
// if topic kind is not registered, it will be registered automatically
func (c *Cluster) ensureTopicKindRegistered() {
	hasTopicKind := false
	for name := range c.kinds {
		if name == TopicActorKind {
			hasTopicKind = true
			break
		}
	}
	if !hasTopicKind {
		store := &EmptyKeyValueStore[*Subscribers]{}

		c.kinds[TopicActorKind] = NewKind(TopicActorKind, actor.PropsFromProducer(func() actor.Actor {
			return NewTopicActor(store, c.Logger())
		})).Build(c)
	}
}

func (c *Cluster) Logger() *slog.Logger {
	if c.ActorSystem == nil {
		return slog.Default()
	}
	logger := c.ActorSystem.Logger()
	if logger == nil {
		return slog.Default()
	}
	return logger
}
