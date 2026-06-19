package grainactor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/service/cluster"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type statePeerSessionReceiver struct {
	set     []PeerSession
	cleared []PeerSession
}

func (s *statePeerSessionReceiver) SetPeerSession(session PeerSession) {
	s.set = append(s.set, session)
}

func (s *statePeerSessionReceiver) ClearPeerSession(session PeerSession) {
	s.cleared = append(s.cleared, session)
}

type toContextPeerSessionHandler struct{}

func (h *toContextPeerSessionHandler) Receive(ctx Context, _ *cluster.GrainRequest) (proto.Message, error) {
	handlerCtx := ToContext(ctx)
	session, ok := PeerSessionFromContext(handlerCtx)
	if !ok {
		return nil, ErrPeerSessionNotBound
	}
	if err := session.SendData([]byte("via-to-context")); err != nil {
		return nil, err
	}
	return wrapperspb.String("ok"), nil
}

type activeCloseHandler struct{}

func (h *activeCloseHandler) Receive(ctx Context, _ *cluster.GrainRequest) (proto.Message, error) {
	session, ok := PeerSessionFromContext(ctx)
	if !ok {
		return nil, ErrPeerSessionNotBound
	}
	if err := session.Close("bye"); err != nil {
		return nil, err
	}
	return wrapperspb.String("closed"), nil
}

func TestBDDGivenUnboundActorWhenGrainRequestThenReturnsPeerSessionNotBound(t *testing.T) {
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player", "player_equip", &peerSessionHandler{})
	}))
	initGrainActor(t, system, pid, "player-1", "player_equip")

	result, err := system.Root.RequestFuture(pid, &cluster.GrainRequest{}, time.Second).Result()
	if err != nil {
		t.Fatal(err)
	}
	response, ok := result.(*cluster.GrainErrorResponse)
	if !ok {
		t.Fatalf("response = %T, want *cluster.GrainErrorResponse", result)
	}
	if !strings.Contains(response.Message, ErrPeerSessionNotBound.Error()) {
		t.Fatalf("response message = %q, want %q", response.Message, ErrPeerSessionNotBound.Error())
	}
}

func TestBDDGivenWrongIdentityWhenBindThenReturnsIdentityMismatch(t *testing.T) {
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player", "player_equip", &peerSessionHandler{})
	}))
	initGrainActor(t, system, pid, "player-1", "player_equip")

	err := BindPeerSession(system.Root, pid, "player-2", "player_equip", &fakePeerSession{})
	if !errors.Is(err, ErrPeerSessionIdentityMismatch) {
		t.Fatalf("err = %v, want ErrPeerSessionIdentityMismatch", err)
	}
}

func TestBDDGivenNilSessionWhenBindThenReturnsNotBoundError(t *testing.T) {
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player", "player_equip", &peerSessionHandler{})
	}))
	initGrainActor(t, system, pid, "player-1", "player_equip")

	err := BindPeerSession(system.Root, pid, "player-1", "player_equip", nil)
	if !errors.Is(err, ErrPeerSessionNotBound) {
		t.Fatalf("err = %v, want ErrPeerSessionNotBound", err)
	}
}

func TestBDDGivenStateImplementsPeerSessionReceiverWhenBindThenNotifiesState(t *testing.T) {
	state := &statePeerSessionReceiver{}
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player", "player_equip", &peerSessionHandler{}, WithState(state))
	}))
	initGrainActor(t, system, pid, "player-1", "player_equip")

	session := &fakePeerSession{}
	if err := BindPeerSession(system.Root, pid, "player-1", "player_equip", session); err != nil {
		t.Fatal(err)
	}
	if len(state.set) != 1 || state.set[0] != session {
		t.Fatalf("state SetPeerSession = %#v, want session", state.set)
	}
}

func TestBDDGivenHandlerUsesToContextWhenBoundThenPeerSessionIsAvailable(t *testing.T) {
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player", "player_equip", &toContextPeerSessionHandler{})
	}))
	initGrainActor(t, system, pid, "player-1", "player_equip")

	session := &fakePeerSession{}
	if err := BindPeerSession(system.Root, pid, "player-1", "player_equip", session); err != nil {
		t.Fatal(err)
	}

	result, err := system.Root.RequestFuture(pid, &cluster.GrainRequest{}, time.Second).Result()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.(*wrapperspb.StringValue); !ok {
		t.Fatalf("response = %T, want *wrapperspb.StringValue", result)
	}
	if len(session.sent) != 1 || string(session.sent[0]) != "via-to-context" {
		t.Fatalf("sent = %#v, want via-to-context", session.sent)
	}
}

func TestBDDGivenBoundSessionWhenHandlerClosesThenSessionReceivesReason(t *testing.T) {
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player", "player_equip", &activeCloseHandler{})
	}))
	initGrainActor(t, system, pid, "player-1", "player_equip")

	session := &fakePeerSession{}
	if err := BindPeerSession(system.Root, pid, "player-1", "player_equip", session); err != nil {
		t.Fatal(err)
	}

	result, err := system.Root.RequestFuture(pid, &cluster.GrainRequest{}, time.Second).Result()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.(*wrapperspb.StringValue); !ok {
		t.Fatalf("response = %T, want *wrapperspb.StringValue", result)
	}
	if session.closeCount != 1 || session.closeReason != "bye" {
		t.Fatalf("close = %d/%q, want 1/bye", session.closeCount, session.closeReason)
	}
}

func TestBDDGivenStoppedActorWhenBindThenRejectsBinding(t *testing.T) {
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player", "player_equip", &peerSessionHandler{})
	}))
	initGrainActor(t, system, pid, "player-1", "player_equip")

	if err := system.Root.PoisonFuture(pid).Wait(); err != nil {
		t.Fatal(err)
	}

	err := BindPeerSession(system.Root, pid, "player-1", "player_equip", &fakePeerSession{})
	if !errors.Is(err, ErrPeerSessionActorNotLocal) {
		t.Fatalf("err = %v, want ErrPeerSessionActorNotLocal", err)
	}
}

func TestBDDGivenBoundSessionWhenClearedThenPeerSessionFromContextIsMissing(t *testing.T) {
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player", "player_equip", &peerSessionHandler{})
	}))
	initGrainActor(t, system, pid, "player-1", "player_equip")

	session := &fakePeerSession{}
	if err := BindPeerSession(system.Root, pid, "player-1", "player_equip", session); err != nil {
		t.Fatal(err)
	}
	if err := ClearPeerSession(system.Root, pid, "player-1", "player_equip", session); err != nil {
		t.Fatal(err)
	}

	actorCtx := newContext(context.Background(), nil, "player-1", "player_equip", "player", nil, func() PeerSession { return nil })
	handlerCtx := ToContext(actorCtx)
	if got, ok := PeerSessionFromContext(handlerCtx); ok || got != nil {
		t.Fatalf("PeerSessionFromContext = %#v, %v; want nil, false", got, ok)
	}
}
