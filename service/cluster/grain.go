package cluster

import (
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

type GrainCallConfig struct {
	RetryCount  int
	Timeout     time.Duration
	RetryAction func(n int) int
	Context     actor.SenderContext
	RouteKey    uint64
	HasRouteKey bool
}

type GrainCallOption func(config *GrainCallConfig)

var defaultGrainCallOptions *GrainCallConfig

func DefaultGrainCallConfig(cluster *Cluster) *GrainCallConfig {
	if defaultGrainCallOptions == nil {
		defaultGrainCallOptions = NewGrainCallOptions(cluster)
	}
	return defaultGrainCallOptions
}

func NewGrainCallOptions(cluster *Cluster) *GrainCallConfig {
	return &GrainCallConfig{
		// TODO: set default in config
		RetryCount: 3,
		Context:    cluster.ActorSystem.Root,
		Timeout:    cluster.Config.RequestTimeoutTime,
		RetryAction: func(i int) int {
			i++
			time.Sleep(time.Duration(i * i * 50))
			return i
		},
	}
}

func WithTimeout(timeout time.Duration) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.Timeout = timeout
	}
}

func WithRetryCount(count int) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.RetryCount = count
	}
}

func WithRetryAction(act func(i int) int) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.RetryAction = act
	}
}

func WithContext(ctx actor.SenderContext) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.Context = ctx
	}
}

// WithRouteKey routes a grain call by the given route key.
func WithRouteKey(routeKey uint64) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.RouteKey = routeKey
		config.HasRouteKey = true
	}
}

type ClusterInit struct {
	Identity *ClusterIdentity
	Cluster  *Cluster
}
