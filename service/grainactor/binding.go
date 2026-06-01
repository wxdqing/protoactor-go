package grainactor

import (
	"github.com/asynkron/protoactor-go/service/cluster"
	"google.golang.org/protobuf/proto"
)

// Handler dispatches generated grain requests inside a base actor.
type Handler interface {
	Receive(ctx Context, req *cluster.GrainRequest) (proto.Message, error)
}
