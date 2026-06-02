package main

import (
	"context"

	grainoneway "github.com/asynkron/protoactor-go/service/example/grain-oneway/proto"
)

type pingHandler struct {
	received chan uint64
}

func (h *pingHandler) Ping(ctx context.Context, req *grainoneway.PingRequest) error {
	serverID, err := grainoneway.GetServerIDKey(ctx)
	if err != nil {
		serverID = req.GetServerId()
	}
	h.received <- serverID
	return nil
}
