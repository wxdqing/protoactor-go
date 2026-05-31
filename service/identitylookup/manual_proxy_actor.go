package identitylookup

import (
	"log/slog"
	"time"

	"gitee.com/wxdqing/identitylookup/types"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	clustering "github.com/asynkron/protoactor-go/cluster"
)

type manualProxyActor struct {
	cluster        *cluster.Cluster
	placementActor *actor.PID
}

func newManualProxyActor(c *clustering.Cluster, placementActor *actor.PID) *manualProxyActor {
	return &manualProxyActor{
		cluster:        c,
		placementActor: placementActor,
	}
}

func (p *manualProxyActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		ctx.Logger().Info("storage manual proxy actor started")
	case *actor.Stopping:
		ctx.Logger().Info("storage manual proxy actor stopping")
	case *actor.Stopped:
		ctx.Logger().Info("storage manual proxy actor stopped")
	case *clustering.ActivationRequest:
		ctx.Logger().Info("storage manual proxy actor activate request")
		p.onActivationRequest(msg, ctx)
	default:
		ctx.Logger().Error("storage placement actor received unknown message", slog.Any("message", msg), slog.Any("sender", ctx.Sender()))
	}
}

func (p *manualProxyActor) onActivationRequest(msg *clustering.ActivationRequest, ctx actor.Context) {
	req := &types.ManualActivateRequest{
		Request: msg,
	}

	ret, err := ctx.ActorSystem().Root.RequestFuture(p.placementActor, req, 15*time.Second).Result()
	if err != nil {
		ctx.Respond(&clustering.ActivationResponse{})
	} else {
		ctx.Respond(ret.(*clustering.ActivationResponse))
	}
}
