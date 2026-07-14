package grainactor

import (
	"strings"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/service/cluster"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type fakeHandler struct {
	called      bool
	panic       bool
	methodIndex int32
}

func (h *fakeHandler) Receive(ctx Context, req *cluster.GrainRequest) (proto.Message, error) {
	h.called = true
	if h.panic {
		panic("boom")
	}
	if ctx.Identity() != "player-1" {
		return nil, cluster.NewGrainErrorResponse(cluster.ErrorReason_INTERNAL, "missing identity")
	}
	if req.MethodIndex != h.methodIndex {
		return nil, cluster.NewGrainErrorResponse(cluster.ErrorReason_NOT_FOUND, "unknown grain method index")
	}
	return wrapperspb.String("ok"), nil
}

func TestBaseActorDelegatesMethodIndexRequestToHandler(t *testing.T) {
	handler := &fakeHandler{methodIndex: 7}
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player_equip", handler)
	}))

	system.Root.Send(pid, &cluster.ClusterInit{
		Identity: &cluster.ClusterIdentity{Identity: "player-1", Kind: "player_equip"},
	})

	future := system.Root.RequestFuture(pid, &cluster.GrainRequest{
		MethodIndex: 7,
	}, time.Second)
	result, err := future.Result()
	if err != nil {
		t.Fatal(err)
	}

	if !handler.called {
		t.Fatal("handler was not called")
	}
	response, ok := result.(*wrapperspb.StringValue)
	if !ok {
		t.Fatalf("response = %T, want *wrapperspb.StringValue", result)
	}
	if response.Value != "ok" {
		t.Fatalf("response.Value = %q", response.Value)
	}
}

func TestBaseActorReturnsHandlerErrorForUnknownMethodIndex(t *testing.T) {
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player_equip", &fakeHandler{methodIndex: 7})
	}))

	system.Root.Send(pid, &cluster.ClusterInit{
		Identity: &cluster.ClusterIdentity{Identity: "player-1", Kind: "player_equip"},
	})

	result, err := system.Root.RequestFuture(pid, &cluster.GrainRequest{
		MethodIndex: 8,
	}, time.Second).Result()
	if err != nil {
		t.Fatal(err)
	}

	response, ok := result.(*cluster.GrainErrorResponse)
	if !ok {
		t.Fatalf("response = %T, want *cluster.GrainErrorResponse", result)
	}
	if response.Reason != cluster.ErrorReason_NOT_FOUND {
		t.Fatalf("Reason = %q, want %q", response.Reason, cluster.ErrorReason_NOT_FOUND)
	}
}

func TestBaseActorSkipsRespondForOneWayRequest(t *testing.T) {
	handler := &fakeHandler{methodIndex: 7}
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player_equip", handler)
	}))

	system.Root.Send(pid, &cluster.ClusterInit{
		Identity: &cluster.ClusterIdentity{Identity: "player-1", Kind: "player_equip"},
	})

	system.Root.Send(pid, &cluster.GrainRequest{
		MethodIndex: 7,
		OneWay:      true,
	})

	time.Sleep(50 * time.Millisecond)

	if !handler.called {
		t.Fatal("handler was not called")
	}
}

func TestBaseActorRecoversHandlerPanic(t *testing.T) {
	system := actor.NewActorSystem()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewBaseActor("player_equip", &fakeHandler{panic: true})
	}))

	system.Root.Send(pid, &cluster.ClusterInit{
		Identity: &cluster.ClusterIdentity{Identity: "player-1", Kind: "player_equip"},
	})

	result, err := system.Root.RequestFuture(pid, &cluster.GrainRequest{
		MethodIndex: 7,
	}, time.Second).Result()
	if err != nil {
		t.Fatal(err)
	}

	response, ok := result.(*cluster.GrainErrorResponse)
	if !ok {
		t.Fatalf("response = %T, want *cluster.GrainErrorResponse", result)
	}
	if response.Reason != cluster.ErrorReason_INTERNAL {
		t.Fatalf("Reason = %q, want %q", response.Reason, cluster.ErrorReason_INTERNAL)
	}
	if !strings.Contains(response.Message, "boom") {
		t.Fatalf("Message = %q, want panic detail", response.Message)
	}
}
