package main

import (
	"context"

	grainoneway "github.com/asynkron/protoactor-go/service/example/grain-oneway/proto"
)

type pingHandler struct {
	received chan uint64
}

func (h *pingHandler) Ping(_ context.Context, req *grainoneway.PingRequest) error {
	h.received <- req.GetServerId()
	return nil
}
