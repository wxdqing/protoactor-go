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
	state     any
	session   PeerSession
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
		a.stop()
	case actor.AutoReceiveMessage:
	case actor.SystemMessage:
	case *PeerSessionBind:
		a.receivePeerSessionBind(ctx, msg)
	case *PeerSessionClear:
		a.receivePeerSessionClear(ctx, msg)
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
	a.state = state
	a.ctx = newContext(context.Background(), grainContext, msg.Identity.Identity, a.kind, a.actorName, state, a.currentPeerSession)
	if a.config.receiveTimeout > 0 {
		ctx.SetReceiveTimeout(a.config.receiveTimeout)
	}
}

func (a *BaseActor) stop() {
	if a.ctx != nil {
		_ = a.clearPeerSession(a.ctx.Identity(), a.ctx.Kind(), a.session)
	}
}

func (a *BaseActor) receivePeerSessionBind(ctx actor.Context, msg *PeerSessionBind) {
	ctx.Respond(&peerSessionResult{err: a.bindPeerSession(msg.Identity, msg.Kind, msg.Session)})
}

func (a *BaseActor) receivePeerSessionClear(ctx actor.Context, msg *PeerSessionClear) {
	ctx.Respond(&peerSessionResult{err: a.clearPeerSession(msg.Identity, msg.Kind, msg.Session)})
}

func (a *BaseActor) receiveGrainRequest(ctx actor.Context, msg *cluster.GrainRequest) {
	if a.handler == nil {
		if !msg.OneWay && ctx.Sender() != nil {
			ctx.Respond(cluster.NewGrainErrorResponse(
				cluster.ErrorReason_NOT_FOUND,
				fmt.Sprintf("grain method index %d not found", msg.MethodIndex),
			))
		}
		return
	}

	resp, err := a.callHandler(msg)
	if msg.OneWay || ctx.Sender() == nil {
		return
	}
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

func (a *BaseActor) currentPeerSession() PeerSession {
	return a.session
}

func (a *BaseActor) bindPeerSession(identity string, kind string, session PeerSession) error {
	if err := a.validatePeerSessionIdentity(identity, kind); err != nil {
		return err
	}
	if session == nil {
		return peerSessionNilError()
	}
	if a.session == session {
		return nil
	}
	if a.session != nil {
		old := a.session
		_ = a.clearPeerSession(identity, kind, old)
		_ = old.Close("replaced")
	}
	a.session = session
	a.setPeerSession(session)
	return nil
}

func (a *BaseActor) clearPeerSession(identity string, kind string, session PeerSession) error {
	if err := a.validatePeerSessionIdentity(identity, kind); err != nil {
		return err
	}
	if session == nil || a.session != session {
		return nil
	}
	a.session = nil
	a.clearPeerSessionReceiver(session)
	return nil
}

func (a *BaseActor) validatePeerSessionIdentity(identity string, kind string) error {
	if a.ctx == nil || a.ctx.Identity() != identity || a.ctx.Kind() != kind {
		return ErrPeerSessionIdentityMismatch
	}
	return nil
}

func (a *BaseActor) setPeerSession(session PeerSession) {
	if receiver, ok := a.handler.(PeerSessionReceiver); ok {
		receiver.SetPeerSession(session)
	}
	if receiver, ok := a.state.(PeerSessionReceiver); ok {
		receiver.SetPeerSession(session)
	}
}

func (a *BaseActor) clearPeerSessionReceiver(session PeerSession) {
	if receiver, ok := a.handler.(PeerSessionReceiver); ok {
		receiver.ClearPeerSession(session)
	}
	if receiver, ok := a.state.(PeerSessionReceiver); ok {
		receiver.ClearPeerSession(session)
	}
}
