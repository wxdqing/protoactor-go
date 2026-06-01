package grainactor

import (
	"context"
	"fmt"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/service/cluster"
	"google.golang.org/protobuf/proto"
)

// BaseActor hosts one logical actor group and delegates grain requests to a generated handler.
type BaseActor struct {
	actorName string
	kind      string
	handler   Handler
	config    config
	ctx       Context
}

// NewBaseActor creates a shared actor for generated grain service handlers.
func NewBaseActor(actorName string, kind string, handler Handler, opts ...Option) actor.Actor {
	cfg := config{}
	for _, opt := range opts {
		opt(&cfg)
	}

	return &BaseActor{
		actorName: actorName,
		kind:      kind,
		handler:   handler,
		config:    cfg,
	}
}

// Receive handles lifecycle and generated grain requests.
func (a *BaseActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
	case *cluster.ClusterInit:
		a.initialize(ctx, msg)
	case *actor.ReceiveTimeout:
		ctx.Poison(ctx.Self())
	case *actor.Stopped:
	case actor.AutoReceiveMessage:
	case actor.SystemMessage:
	case *cluster.GrainRequest:
		a.receiveGrainRequest(ctx, msg)
	}
}

func (a *BaseActor) initialize(ctx actor.Context, msg *cluster.ClusterInit) {
	grainContext := cluster.NewGrainContext(ctx, msg.Identity, msg.Cluster)
	state := a.config.state
	if a.config.stateFactory != nil {
		state = a.config.stateFactory(msg.Identity.Identity)
	}
	a.ctx = newContext(context.Background(), grainContext, msg.Identity.Identity, a.kind, a.actorName, state)
	if a.config.receiveTimeout > 0 {
		ctx.SetReceiveTimeout(a.config.receiveTimeout)
	}
}

func (a *BaseActor) receiveGrainRequest(ctx actor.Context, msg *cluster.GrainRequest) {
	if a.handler == nil {
		ctx.Respond(cluster.NewGrainErrorResponse(
			cluster.ErrorReason_NOT_FOUND,
			fmt.Sprintf("grain method index %d not found", msg.MethodIndex),
		))
		return
	}

	resp, err := a.callHandler(msg)
	if err != nil {
		ctx.Respond(cluster.FromError(err))
		return
	}
	ctx.Respond(resp)
}

func (a *BaseActor) callHandler(msg *cluster.GrainRequest) (resp proto.Message, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = cluster.NewGrainErrorResponse(
				cluster.ErrorReason_INTERNAL,
				fmt.Sprintf("grain handler panic: %v", recovered),
			)
		}
	}()
	return a.handler.Receive(a.ctx, msg)
}
