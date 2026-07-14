package grainactor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/service/cluster"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type fakePeerSession struct {
	sent        [][]byte
	closeReason string
	closeCount  int
}

func (s *fakePeerSession) SendData(payload []byte) error {
	s.sent = append(s.sent, append([]byte(nil), payload...))
	return nil
}

func (s *fakePeerSession) Close(reason string) error {
	s.closeReason = reason
	s.closeCount++
	return nil
}

type peerSessionHandler struct {
	set     []PeerSession
	cleared []PeerSession
}

func (h *peerSessionHandler) Receive(ctx Context, _ *cluster.GrainRequest) (proto.Message, error) {
	session, ok := PeerSessionFromContext(ctx)
	if !ok {
		return nil, ErrPeerSessionNotBound
	}
	if err := session.SendData([]byte("hello")); err != nil {
		return nil, err
	}
	return wrapperspb.String("ok"), nil
}

func (h *peerSessionHandler) SetPeerSession(session PeerSession) {
	h.set = append(h.set, session)
}

func (h *peerSessionHandler) ClearPeerSession(session PeerSession) {
	h.cleared = append(h.cleared, session)
}

func TestBindPeerSessionMakesSessionAvailableFromContext(t *testing.T) {
	handler := &peerSessionHandler{}
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player_equip", handler)
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
	if len(session.sent) != 1 || string(session.sent[0]) != "hello" {
		t.Fatalf("sent = %#v, want hello", session.sent)
	}
	if len(handler.set) != 1 || handler.set[0] != session {
		t.Fatalf("SetPeerSession calls = %#v, want session", handler.set)
	}
}

func TestBindPeerSessionReplacesAndClosesOldSession(t *testing.T) {
	handler := &peerSessionHandler{}
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player_equip", handler)
	}))
	initGrainActor(t, system, pid, "player-1", "player_equip")

	oldSession := &fakePeerSession{}
	newSession := &fakePeerSession{}
	if err := BindPeerSession(system.Root, pid, "player-1", "player_equip", oldSession); err != nil {
		t.Fatal(err)
	}
	if err := BindPeerSession(system.Root, pid, "player-1", "player_equip", newSession); err != nil {
		t.Fatal(err)
	}

	if oldSession.closeCount != 1 || oldSession.closeReason != "replaced" {
		t.Fatalf("old close = %d/%q, want 1/replaced", oldSession.closeCount, oldSession.closeReason)
	}
	if len(handler.cleared) != 1 || handler.cleared[0] != oldSession {
		t.Fatalf("ClearPeerSession calls = %#v, want old session", handler.cleared)
	}

	result, err := system.Root.RequestFuture(pid, &cluster.GrainRequest{}, time.Second).Result()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.(*wrapperspb.StringValue); !ok {
		t.Fatalf("response = %T, want *wrapperspb.StringValue", result)
	}
	if len(newSession.sent) != 1 {
		t.Fatalf("newSession.sent len = %d, want 1", len(newSession.sent))
	}
}

func TestClearPeerSessionOnlyClearsCurrentSession(t *testing.T) {
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player_equip", &peerSessionHandler{})
	}))
	initGrainActor(t, system, pid, "player-1", "player_equip")

	oldSession := &fakePeerSession{}
	newSession := &fakePeerSession{}
	if err := BindPeerSession(system.Root, pid, "player-1", "player_equip", oldSession); err != nil {
		t.Fatal(err)
	}
	if err := BindPeerSession(system.Root, pid, "player-1", "player_equip", newSession); err != nil {
		t.Fatal(err)
	}
	if err := ClearPeerSession(system.Root, pid, "player-1", "player_equip", oldSession); err != nil {
		t.Fatal(err)
	}

	result, err := system.Root.RequestFuture(pid, &cluster.GrainRequest{}, time.Second).Result()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.(*wrapperspb.StringValue); !ok {
		t.Fatalf("response = %T, want *wrapperspb.StringValue", result)
	}
	if len(newSession.sent) != 1 {
		t.Fatalf("newSession.sent len = %d, want 1", len(newSession.sent))
	}
}

func TestBindPeerSessionFailsForNonLocalActor(t *testing.T) {
	system := actor.NewActorSystem()
	err := BindPeerSession(system.Root, actor.NewPID("remote", "player-1"), "player-1", "player_equip", &fakePeerSession{})
	if !errors.Is(err, ErrPeerSessionActorNotLocal) {
		t.Fatalf("err = %v, want ErrPeerSessionActorNotLocal", err)
	}
}

func TestBindPeerSessionFailsForNonGrainactorActor(t *testing.T) {
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromFunc(func(actor.Context) {}))

	err := BindPeerSession(system.Root, pid, "player-1", "player_equip", &fakePeerSession{})
	if !errors.Is(err, ErrPeerSessionActorNotLocal) {
		t.Fatalf("err = %v, want ErrPeerSessionActorNotLocal", err)
	}
}

func TestPeerSessionFromContextReturnsMissingSessionError(t *testing.T) {
	if session, ok := PeerSessionFromContext(context.Background()); ok || session != nil {
		t.Fatalf("PeerSessionFromContext = %#v, %v; want nil, false", session, ok)
	}
}

func TestStoppedActorClearsPeerSessionAndUnregistersBinding(t *testing.T) {
	handler := &peerSessionHandler{}
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player_equip", handler)
	}))
	initGrainActor(t, system, pid, "player-1", "player_equip")

	session := &fakePeerSession{}
	if err := BindPeerSession(system.Root, pid, "player-1", "player_equip", session); err != nil {
		t.Fatal(err)
	}

	if err := system.Root.PoisonFuture(pid).Wait(); err != nil {
		t.Fatal(err)
	}

	if len(handler.cleared) != 1 || handler.cleared[0] != session {
		t.Fatalf("ClearPeerSession calls = %#v, want session", handler.cleared)
	}
	err := BindPeerSession(system.Root, pid, "player-1", "player_equip", &fakePeerSession{})
	if !errors.Is(err, ErrPeerSessionActorNotLocal) {
		t.Fatalf("err = %v, want ErrPeerSessionActorNotLocal", err)
	}
}

func initGrainActor(t *testing.T, system *actor.ActorSystem, pid *actor.PID, identity, kind string) {
	t.Helper()
	system.Root.Send(pid, &cluster.ClusterInit{
		Identity: &cluster.ClusterIdentity{Identity: identity, Kind: kind},
	})
	time.Sleep(10 * time.Millisecond)
}
