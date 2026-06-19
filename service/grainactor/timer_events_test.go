package grainactor

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/service/cluster"
	"google.golang.org/protobuf/proto"
)

type timerHandler struct {
	fired atomic.Int32
}

func (h *timerHandler) Receive(ctx Context, req *cluster.GrainRequest) (proto.Message, error) {
	switch req.MethodIndex {
	case 1:
		ctx.After("once", 20*time.Millisecond, func(Context) {
			h.fired.Add(1)
		})
	case 2:
		ctx.Every("tick", 15*time.Millisecond, func(Context) {
			h.fired.Add(1)
		})
	case 3:
		ctx.CancelTimer("tick")
	}
	return nil, nil
}

func TestContextAfterTimerFiresInsideMailbox(t *testing.T) {
	handler := &timerHandler{}
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player", "player", handler)
	}))

	system.Root.Send(pid, &cluster.ClusterInit{
		Identity: &cluster.ClusterIdentity{Identity: "player-1", Kind: "player"},
	})
	system.Root.Send(pid, &cluster.GrainRequest{MethodIndex: 1})

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if handler.fired.Load() == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timer did not fire, count=%d", handler.fired.Load())
}

func TestContextEveryTimerCanBeCancelled(t *testing.T) {
	handler := &timerHandler{}
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player", "player", handler)
	}))

	system.Root.Send(pid, &cluster.ClusterInit{
		Identity: &cluster.ClusterIdentity{Identity: "player-1", Kind: "player"},
	})
	system.Root.Send(pid, &cluster.GrainRequest{MethodIndex: 2})
	time.Sleep(40 * time.Millisecond)
	beforeCancel := handler.fired.Load()
	system.Root.Send(pid, &cluster.GrainRequest{MethodIndex: 3})
	time.Sleep(40 * time.Millisecond)
	if handler.fired.Load() != beforeCancel {
		t.Fatalf("timer kept firing after cancel: before=%d after=%d", beforeCancel, handler.fired.Load())
	}
}

type eventHandler struct {
	last any
}

func (h *eventHandler) Receive(ctx Context, req *cluster.GrainRequest) (proto.Message, error) {
	switch req.MethodIndex {
	case 1:
		ctx.On("ready", func(_ Context, payload any) {
			h.last = payload
		})
	case 2:
		ctx.Emit("ready", "ok")
	}
	return nil, nil
}

func TestContextEventEmitDispatchesHandlers(t *testing.T) {
	handler := &eventHandler{}
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player", "player", handler)
	}))

	system.Root.Send(pid, &cluster.ClusterInit{
		Identity: &cluster.ClusterIdentity{Identity: "player-1", Kind: "player"},
	})
	system.Root.Send(pid, &cluster.GrainRequest{MethodIndex: 1})
	system.Root.Send(pid, &cluster.GrainRequest{MethodIndex: 2})

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if handler.last == "ok" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("event handler was not invoked, last=%v", handler.last)
}
