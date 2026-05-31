package identitylookup

import (
	"context"
	"log/slog"
	"time"

	"rmini-migration/internal/go-sprout/identitylookup/types"
	"github.com/asynkron/protoactor-go/actor"
	clustering "github.com/asynkron/protoactor-go/cluster"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type GrainMeta struct {
	ID  *clustering.ClusterIdentity
	PID *actor.PID
}

type placementActor struct {
	cluster *clustering.Cluster
	actors  map[string]GrainMeta
	pidMap  map[string]string
	pm      *Manager
}

func newPlacementActor(c *clustering.Cluster, pm *Manager) *placementActor {
	return &placementActor{
		cluster: c,
		pm:      pm,
		actors:  make(map[string]GrainMeta),
		pidMap:  make(map[string]string),
	}
}

func (p *placementActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		ctx.Logger().Info("storage placement actor started")
	case *actor.Stopping:
		ctx.Logger().Info("storage placement actor stopping")
		p.onStopping(ctx)
	case *actor.Stopped:
		ctx.Logger().Info("storage placement actor stopped")
	case *actor.Terminated:
		p.onTerminated(msg, ctx)
	case *clustering.ActivationRequest:
		p.onActivationRequest(msg, ctx)
	case *types.ManualActivateRequest:
		p.onManualActivateRequest(msg, ctx)
	default:
		ctx.Logger().Error("storage placement actor received unknown message", slog.Any("message", msg), slog.Any("sender", ctx.Sender()))
	}
}

func (p *placementActor) onTerminated(msg *actor.Terminated, ctx actor.Context) {
	found, key, meta := p.pidToMeta(msg.Who)
	if found {
		ctx.Logger().Info("Storage placement sub actor terminated", slog.String("identity", *key))
		clusterKind := p.cluster.GetClusterKind(meta.ID.Kind)
		clusterKind.Dec()

		p.updateVirtualActorsGauge()
		activationTerminated := &clustering.ActivationTerminated{
			Pid:             msg.Who,
			ClusterIdentity: meta.ID,
		}
		p.pm.cluster.MemberList.BroadcastEvent(activationTerminated, true)

		p.pm.RemovePid(meta.ID, msg.Who)
		delete(p.actors, *key)
		delete(p.pidMap, msg.Who.Id)
	} else {
		ctx.Logger().Error("Failed to find actor", slog.String("actorid", msg.Who.Id))
	}
}

func (p *placementActor) onStopping(ctx actor.Context) {
	ctx.Logger().Info("Storage placement actor onStopping", "actornum", len(p.actors))
	futures := make(map[string]actor.Future, len(p.actors))

	for key, meta := range p.actors {
		ctx.Logger().Info("poison actor success", slog.String("identity", key))
		futures[key] = ctx.PoisonFuture(meta.PID)
	}

	for key, future := range futures {
		err := future.Wait()
		if err != nil {
			ctx.Logger().Error("Failed to poison actor", slog.String("identity", key), slog.Any("error", err))
		}
	}
}

func (p *placementActor) activateActor(msg *clustering.ActivationRequest, ctx actor.Context) *actor.PID {
	key := msg.ClusterIdentity.AsKey()
	if meta, found := p.actors[key]; found {
		return meta.PID
	}

	clusterKind := p.cluster.GetClusterKind(msg.ClusterIdentity.Kind)
	if clusterKind == nil {
		ctx.Logger().Error("Unknown cluster kind", slog.String("kind", msg.ClusterIdentity.Kind))
		// TODO: what to do here?
		return nil
	}

	props := clustering.WithClusterIdentity(clusterKind.Props, msg.ClusterIdentity)

	start := time.Now()
	pid := ctx.SpawnPrefix(props, msg.ClusterIdentity.Identity)
	clusterKind.Inc()
	if p.cluster.MetricsEnabled() {
		_ctx := context.Background()
		attrs := append(
			actor.SystemLabels(p.cluster.ActorSystem),
			attribute.String("clusterkind", msg.ClusterIdentity.Kind),
		)
		p.cluster.Metrics().ClusterActorSpawnDuration.Record(_ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
		p.updateVirtualActorsGauge()
	}

	p.actors[key] = GrainMeta{
		ID:  msg.ClusterIdentity,
		PID: pid,
	}
	p.pidMap[pid.Id] = key

	if err := p.pm.SavePid(msg.ClusterIdentity, pid, msg.RequestId); err != nil {
		ctx.Logger().Error("Failed to persist pid", slog.Any("error", err),
			slog.String("requestId", msg.RequestId), slog.String("identity", key))
		ctx.Poison(pid)
		return nil
	}

	return pid
}

func (p *placementActor) onActivationRequest(msg *clustering.ActivationRequest, ctx actor.Context) {
	if p.pm.IsManualKind(msg.ClusterIdentity.Kind) {
		ctx.Logger().Error("manual kind should be create by ManualActivateRequest", slog.String("kind", msg.ClusterIdentity.Kind))
		ctx.Respond(&clustering.ActivationResponse{})
		return
	}

	pid := p.activateActor(msg, ctx)
	ctx.Respond(&clustering.ActivationResponse{Pid: pid})
}

func (p *placementActor) onManualActivateRequest(msg *types.ManualActivateRequest, ctx actor.Context) {
	pid := p.activateActor(msg.Request, ctx)
	ctx.Respond(&clustering.ActivationResponse{Pid: pid})
}

func (p *placementActor) pidToMeta(pid *actor.PID) (bool, *string, *GrainMeta) {
	key := p.pidMap[pid.Id]
	if key != "" {
		meta := p.actors[key]
		return true, &key, &meta
	}
	return false, nil, nil
}

func (p *placementActor) updateVirtualActorsGauge() {
	if !p.cluster.MetricsEnabled() {
		return
	}
	p.cluster.Metrics().VirtualActorsCount.Set(p.cluster.VirtualActorCount())
}
