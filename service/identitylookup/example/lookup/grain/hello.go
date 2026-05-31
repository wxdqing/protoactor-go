package grain

import (
	"time"

	proto "gitee.com/wxdqing/identitylookup/example/lookup/grain/proto"

	"github.com/asynkron/protoactor-go/cluster"
)

type HelloGrain struct{}

func (h HelloGrain) Init(ctx cluster.GrainContext) {
	ctx.Logger().Info("Hello Init", "grain", ctx.Identity())
	time.Sleep(1 * time.Second)
}
func (h HelloGrain) Terminate(ctx cluster.GrainContext) {
	ctx.Logger().Info("Hello Terminate", "grain", ctx.Identity())
}
func (h HelloGrain) ReceiveDefault(ctx cluster.GrainContext) {}

func (h HelloGrain) SayHello(request *proto.HelloRequest, ctx cluster.GrainContext) (*proto.HelloResponse, error) {
	ctx.Logger().Info("SayHello", "Name", request.Name)
	return &proto.HelloResponse{Message: "Hello " + request.Name}, nil
}
