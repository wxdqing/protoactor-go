// Package consul provides a Consul-based cluster provider.
package consul

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/asynkron/protoactor-go/actor"

	"github.com/asynkron/protoactor-go/service/cluster"
	"github.com/hashicorp/consul/api"
)

// ErrProviderShuttingDown is returned when operations occur during provider shutdown.
var ErrProviderShuttingDown = fmt.Errorf("consul cluster provider is shutting down")

// ProviderShuttingDownError is retained for backward compatibility.
//
//lint:ignore ST1012 deprecated: use ErrProviderShuttingDown instead
var ProviderShuttingDownError = ErrProviderShuttingDown

// Provider integrates Consul as a cluster provider for Proto.Actor.
type Provider struct {
	cluster            *cluster.Cluster
	deregistered       bool
	shutdown           bool
	id                 string
	clusterName        string
	address            string
	port               int
	knownKinds         []string
	index              uint64 // consul blocking index
	client             *api.Client
	ttl                time.Duration
	refreshTTL         time.Duration
	updateTTLWaitGroup sync.WaitGroup
	deregisterCritical time.Duration
	blockingWaitTime   time.Duration
	clusterError       error
	pid                *actor.PID
	consulConfig       *api.Config
	startTimeout       time.Duration
	serviceMetadata    map[string]string
	stateMu            sync.RWMutex
	leaseGrant         ConsulLeaseGrant
	health             HealthState
}

// ConsulLeaseGrant is the local conservative authorization from a successful TTL refresh.
type ConsulLeaseGrant struct {
	ValidUntil time.Time
}

// HealthState describes the latest Consul provider operation.
type HealthState struct {
	Healthy     bool
	LastSuccess time.Time
	Err         error
}

// New creates a new Consul provider with default configuration.
func New(opts ...Option) (*Provider, error) {
	return NewWithConfig(&api.Config{}, opts...)
}

// NewWithConfig creates a new Consul provider using the supplied Consul config.
func NewWithConfig(consulConfig *api.Config, opts ...Option) (*Provider, error) {
	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, err
	}
	p := &Provider{
		client:             client,
		ttl:                3 * time.Second,
		refreshTTL:         1 * time.Second,
		deregisterCritical: 60 * time.Second,
		blockingWaitTime:   20 * time.Second,
		startTimeout:       10 * time.Second,
		consulConfig:       consulConfig,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p, nil
}

func (p *Provider) init(c *cluster.Cluster) error {
	knownKinds := c.GetClusterKinds()
	clusterName := c.Config.Name
	memberID := c.ActorSystem.ID

	host, port, err := c.ActorSystem.GetHostPort()
	if err != nil {
		return err
	}

	p.cluster = c
	p.id = memberID
	p.clusterName = clusterName
	p.address = host
	p.port = port
	p.knownKinds = knownKinds
	return nil
}

// StartMember connects the provider to Consul and registers the node as a member.
func (p *Provider) StartMember(c *cluster.Cluster) error {
	err := p.init(c)
	if err != nil {
		return err
	}

	ready := make(chan error, 1)
	p.pid, err = c.ActorSystem.Root.SpawnNamed(actor.PropsFromProducer(func() actor.Actor {
		return newProviderActor(p, ready)
	}), "consul-provider")
	if err != nil {
		p.cluster.Logger().Error("Failed to start consul-provider actor", slog.Any("error", err))
		return err
	}

	timer := time.NewTimer(p.startTimeout)
	defer timer.Stop()
	select {
	case err := <-ready:
		if err != nil {
			_ = c.ActorSystem.Root.StopFuture(p.pid).Wait()
			p.pid = nil
			_ = p.deregisterService()
		}
		return err
	case <-timer.C:
		_ = c.ActorSystem.Root.StopFuture(p.pid).Wait()
		p.pid = nil
		_ = p.deregisterService()
		err := fmt.Errorf("consul provider start timed out after %s", p.startTimeout)
		p.setHealth(err)
		return err
	}
}

// StartClient connects the provider to Consul without registering the node as a member.
func (p *Provider) StartClient(c *cluster.Cluster) error {
	if err := p.init(c); err != nil {
		return err
	}
	if err := p.notifyStatuses(); err != nil {
		return err
	}
	p.monitorMemberStatusChanges()
	return nil
}

// DeregisterMember removes the provider's service registration from Consul.
func (p *Provider) DeregisterMember() error {
	p.stateMu.Lock()
	if p.deregistered {
		p.stateMu.Unlock()
		return nil
	}
	p.deregistered = true
	p.stateMu.Unlock()
	err := p.deregisterService()
	if err != nil {
		p.stateMu.Lock()
		p.deregistered = false
		p.stateMu.Unlock()
		return err
	}
	return nil
}

// Shutdown stops the provider and its internal actor.
func (p *Provider) Shutdown(_ bool) error {
	p.stateMu.Lock()
	if p.shutdown {
		p.stateMu.Unlock()
		return nil
	}
	p.shutdown = true
	p.stateMu.Unlock()
	if p.pid != nil {
		if err := p.cluster.ActorSystem.Root.StopFuture(p.pid).Wait(); err != nil {
			p.cluster.Logger().Error("Failed to stop consul-provider actor", slog.Any("error", err))
		}
		p.pid = nil
	}

	return nil
}

func blockingUpdateTTL(p *Provider) error {
	requestStart := time.Now()
	err := p.client.Agent().UpdateTTL("service:"+p.id, "", api.HealthPassing)
	p.clusterError = err
	if err != nil {
		p.setHealth(err)
		return err
	}
	p.stateMu.Lock()
	p.leaseGrant = ConsulLeaseGrant{ValidUntil: requestStart.Add(p.ttl)}
	p.health = HealthState{Healthy: true, LastSuccess: time.Now()}
	p.stateMu.Unlock()
	return nil
}

func (p *Provider) registerService() error {
	metadata := cloneMetadata(p.serviceMetadata)
	metadata["id"] = p.id
	s := &api.AgentServiceRegistration{
		ID:      p.id,
		Name:    p.clusterName,
		Tags:    p.knownKinds,
		Address: p.address,
		Port:    p.port,
		Meta:    metadata,
		Check: &api.AgentServiceCheck{
			DeregisterCriticalServiceAfter: p.deregisterCritical.String(),
			TTL:                            p.ttl.String(),
		},
	}
	return p.client.Agent().ServiceRegister(s)
}

func (p *Provider) deregisterService() error {
	return p.client.Agent().ServiceDeregister(p.id)
}

// call this directly after registering the service
func (p *Provider) notifyStatuses() error {
	statuses, meta, err := p.client.Health().Service(p.clusterName, "", true, &api.QueryOptions{
		WaitIndex: p.index,
		WaitTime:  p.blockingWaitTime,
	})
	p.cluster.Logger().Info("Consul health check")

	if err != nil {
		p.cluster.Logger().Error("notifyStatues", slog.Any("error", err))
		p.setHealth(err)
		return err
	}
	p.index = meta.LastIndex

	members := passingMembers(p.clusterName, statuses)
	// the reason why we want this in a batch and not as individual messages is that
	// if we have an atomic batch, we can calculate what nodes have left the cluster
	// passing events one by one, we can't know if someone left or just haven't changed status for a long time

	// publish the current cluster topology onto the event stream
	p.cluster.MemberList.UpdateClusterTopology(members)
	p.stateMu.Lock()
	p.health = HealthState{Healthy: true, LastSuccess: time.Now()}
	p.stateMu.Unlock()
	return nil
}

func (p *Provider) monitorMemberStatusChanges() {
	go func() {
		for !p.isShutdown() {
			if err := p.notifyStatuses(); err != nil {
				time.Sleep(min(p.refreshTTL, time.Second))
			}
		}
	}()
}

// LeaseGrant returns the latest successful local TTL authorization.
func (p *Provider) LeaseGrant() (ConsulLeaseGrant, bool) {
	p.stateMu.RLock()
	defer p.stateMu.RUnlock()
	return p.leaseGrant, p.health.Healthy && !p.leaseGrant.ValidUntil.IsZero()
}

// Health returns a snapshot of provider health.
func (p *Provider) Health() HealthState {
	p.stateMu.RLock()
	defer p.stateMu.RUnlock()
	return p.health
}

func (p *Provider) setHealth(err error) {
	p.stateMu.Lock()
	p.health.Healthy = false
	p.health.Err = err
	p.stateMu.Unlock()
}

func (p *Provider) isShutdown() bool {
	p.stateMu.RLock()
	defer p.stateMu.RUnlock()
	return p.shutdown
}

func passingMembers(clusterName string, entries []*api.ServiceEntry) []*cluster.Member {
	members := make([]*cluster.Member, 0, len(entries))
	for _, entry := range entries {
		if len(entry.Checks) == 0 || entry.Checks.AggregatedStatus() != api.HealthPassing {
			continue
		}
		memberID := entry.Service.Meta["id"]
		if memberID == "" {
			memberID = fmt.Sprintf("%v@%v:%v", clusterName, entry.Service.Address, entry.Service.Port)
		}
		members = append(members, &cluster.Member{Id: memberID, Name: clusterName, Host: entry.Service.Address, Port: int32(entry.Service.Port), Kinds: entry.Service.Tags})
	}
	return members
}

func cloneMetadata(metadata map[string]string) map[string]string {
	cloned := make(map[string]string, len(metadata)+1)
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}
