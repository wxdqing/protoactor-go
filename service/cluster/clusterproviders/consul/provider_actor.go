// Package consul provides a Consul-based cluster provider.
package consul

import (
	"log/slog"
	"sync"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/scheduler"
	"github.com/asynkron/protoactor-go/service/cluster"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
)

type providerActor struct {
	*Provider
	actor.Behavior
	refreshCanceller scheduler.CancelFunc
	ready            chan<- error
	readyOnce        sync.Once
}

type (
	// RegisterService asks the provider actor to register its service in Consul.
	RegisterService struct{}
	// UpdateTTL triggers a TTL refresh in Consul for the registered service.
	UpdateTTL struct{}
	// MemberListUpdated carries the latest set of cluster members retrieved from Consul.
	MemberListUpdated struct {
		members []*cluster.Member
		index   uint64
	}
)

func (pa *providerActor) Receive(ctx actor.Context) {
	pa.Behavior.Receive(ctx)
}

func newProviderActor(provider *Provider, ready chan<- error) actor.Actor {
	pa := &providerActor{
		Behavior: actor.NewBehavior(),
		Provider: provider,
		ready:    ready,
	}
	pa.Become(pa.init)
	return pa
}

func (pa *providerActor) init(ctx actor.Context) {
	switch ctx.Message().(type) {
	case *actor.Started:
		ctx.Send(ctx.Self(), &RegisterService{})
	case *RegisterService:
		if err := pa.registerService(); err != nil {
			ctx.Logger().Error("Failed to register service to consul", slog.Any("error", err))
			pa.signalReady(err)
		} else {
			ctx.Logger().Info("Registered service to consul")
			if err := blockingUpdateTTL(pa.Provider); err != nil {
				pa.signalReady(err)
				return
			}
			refreshScheduler := scheduler.NewTimerScheduler(ctx)
			pa.refreshCanceller = refreshScheduler.SendRepeatedly(pa.refreshTTL, pa.refreshTTL, ctx.Self(), &UpdateTTL{})
			if err := pa.startWatch(ctx); err == nil {
				pa.Become(pa.running)
				pa.signalReady(nil)
			} else {
				pa.signalReady(err)
			}
		}
	}
}

func (pa *providerActor) running(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *UpdateTTL:
		if err := blockingUpdateTTL(pa.Provider); err != nil {
			ctx.Logger().Warn("Failed to update TTL", slog.Any("error", err))
		}
	case *MemberListUpdated:
		pa.cluster.MemberList.UpdateClusterTopology(msg.members)
	case *actor.Stopping:
		if pa.refreshCanceller != nil {
			pa.refreshCanceller()
		}
		if err := pa.DeregisterMember(); err != nil {
			ctx.Logger().Error("Failed to deregister service from consul", slog.Any("error", err))
		} else {
			ctx.Logger().Info("De-registered service from consul")
		}
	}
}

func (pa *providerActor) startWatch(ctx actor.Context) error {
	params := make(map[string]interface{})
	params["type"] = "service"
	params["service"] = pa.clusterName
	params["passingonly"] = true
	plan, err := watch.Parse(params)
	if err != nil {
		ctx.Logger().Error("Failed to parse consul watch definition", slog.Any("error", err))
		return err
	}
	plan.Handler = func(index uint64, result interface{}) {
		pa.processConsulUpdate(index, result, ctx)
	}

	go func() {
		for !pa.isShutdown() {
			if runErr := plan.RunWithConfig(pa.consulConfig.Address, pa.consulConfig); runErr != nil {
				pa.setHealth(runErr)
				ctx.Logger().Error("Consul watch stopped", slog.Any("error", runErr))
			}
			if !pa.isShutdown() {
				time.Sleep(min(pa.refreshTTL, time.Second))
			}
		}
	}()

	return nil
}

func (pa *providerActor) processConsulUpdate(index uint64, result interface{}, ctx actor.Context) {
	serviceEntries, ok := result.([]*api.ServiceEntry)
	if !ok {
		ctx.Logger().Warn("Didn't get expected data from consul watch")
		return
	}
	members := passingMembers(pa.clusterName, serviceEntries)
	ctx.Send(ctx.Self(), &MemberListUpdated{members: members, index: index})
}

func (pa *providerActor) signalReady(err error) {
	pa.readyOnce.Do(func() { pa.ready <- err })
}
