package cluster

import (
	"sync/atomic"

	"github.com/asynkron/protoactor-go/actor"
)

// Kind represents the kinds of actors a cluster can manage
type Kind struct {
	Kind            string
	Props           *actor.Props
	StrategyBuilder func(*Cluster) MemberStrategy
}

// NewKind creates a new instance of a kind
func NewKind(kind string, props *actor.Props) *Kind {
	// add cluster middleware
	p := props.Clone(withClusterReceiveMiddleware())
	return &Kind{
		Kind:            kind,
		Props:           p,
		StrategyBuilder: nil,
	}
}

// WithMemberStrategy sets the member strategy builder for this kind.
func (k *Kind) WithMemberStrategy(strategyBuilder func(*Cluster) MemberStrategy) {
	k.StrategyBuilder = strategyBuilder
}

// Build creates an activated kind for the given cluster.
func (k *Kind) Build(cluster *Cluster) *ActivatedKind {
	var strategy MemberStrategy = nil
	if k.StrategyBuilder != nil {
		strategy = k.StrategyBuilder(cluster)
	}

	return &ActivatedKind{
		Kind:     k.Kind,
		Props:    k.Props,
		Strategy: strategy,
	}
}

// ActivatedKind tracks runtime state for a kind registered in the cluster.
type ActivatedKind struct {
	Kind     string
	Props    *actor.Props
	Strategy MemberStrategy
	count    int32
}

// Inc increments the activated instance count.
func (ak *ActivatedKind) Inc() {
	atomic.AddInt32(&ak.count, 1)
}

// Dec decrements the activated instance count.
func (ak *ActivatedKind) Dec() {
	atomic.AddInt32(&ak.count, -1)
}

// Count returns the current activated instance count.
func (ak *ActivatedKind) Count() int32 {
	return atomic.LoadInt32(&ak.count)
}
