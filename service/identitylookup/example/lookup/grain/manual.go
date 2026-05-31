package grain

import (
	"time"

	proto "gitee.com/wxdqing/identitylookup/example/lookup/grain/proto"

	"github.com/asynkron/protoactor-go/cluster"
)

type ManualGrain struct{}

func (h ManualGrain) Init(ctx cluster.GrainContext) {
	ctx.Logger().Info("Manual Init", "grain", ctx.Identity())
	time.Sleep(1 * time.Second)
}
func (h ManualGrain) Terminate(ctx cluster.GrainContext) {
	ctx.Logger().Info("Manual Terminate", "grain", ctx.Identity())
}
func (h ManualGrain) ReceiveDefault(ctx cluster.GrainContext) {}

func (h ManualGrain) Ping(request *proto.PingRequest, ctx cluster.GrainContext) (*proto.PingResponse, error) {
	ctx.Logger().Info("Ping", "Name", request.Name)
	return &proto.PingResponse{Message: "Pong " + request.Name}, nil
}
