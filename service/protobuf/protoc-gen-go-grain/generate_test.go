package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/asynkron/protoactor-go/service/protobuf/protoc-gen-go-grain/options"
)

func Test_toCamel(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "snake1",
			args: args{"SYSTEM_ERROR"},
			want: "SystemError",
		},
		{
			name: "snake2",
			args: args{"System_Error"},
			want: "SystemError",
		},
		{
			name: "snake3",
			args: args{"system_error"},
			want: "SystemError",
		},
		{
			name: "snake4",
			args: args{"System_error"},
			want: "SystemError",
		},
		{
			name: "snake initialism",
			args: args{"server_id"},
			want: "ServerID",
		},
		{
			name: "upper1",
			args: args{"UNKNOWN"},
			want: "Unknown",
		},
		{
			name: "camel1",
			args: args{"SystemError"},
			want: "SystemError",
		},
		{
			name: "camel2",
			args: args{"systemError"},
			want: "SystemError",
		},
		{
			name: "lower1",
			args: args{"system"},
			want: "System",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toCamel(tt.args.s); got != tt.want {
				t.Errorf("toCamel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateGrainactorOptions(t *testing.T) {
	cmd := exec.Command("protoc",
		"--go_out=.",
		"--go_opt=paths=source_relative",
		"-I../../..",
		"-I.",
		"test/grainactor/grainactor.proto",
	)
	cmd.Dir = "."

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, out)
	}
}

func TestGenerateGrainactorSplitFiles(t *testing.T) {
	pluginPath := buildProtocGenGoGrainPlugin(t)
	t.Cleanup(func() {
		_ = os.Remove(pluginPath)
	})

	_ = os.Remove("test/grainactor/grainactor_grain_client.pb.go")
	_ = os.Remove("test/grainactor/grainactor_grain_server.pb.go")
	_ = os.Remove("test/grainactor/grain_client_init.pb.go")

	cmd := exec.Command("protoc",
		"--go_out=.",
		"--go_opt=paths=source_relative",
		"--plugin=protoc-gen-go-grain="+pluginPath,
		"--go-grain_out=.",
		"--go-grain_opt=paths=source_relative",
		"-I../../..",
		"-I.",
		"test/grainactor/grainactor.proto",
	)
	cmd.Dir = "."

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, out)
	}

	client := readGeneratedFile(t, "test/grainactor/grainactor_grain_client.pb.go")
	server := readGeneratedFile(t, "test/grainactor/grainactor_grain_server.pb.go")
	init := readGeneratedFile(t, "test/grainactor/grain_client_init.pb.go")

	requireContains(t, client, "func GetCrossSnsGrainClient(c *cluster.Cluster, id string) *CrossSnsGrainClient")
	requireContains(t, client, "type BaseCrossSnsGrainClient struct {")
	requireContains(t, client, "func WithServerIDKey(serverID uint64) cluster.GrainCallOption")
	requireNotContains(t, client, "type PlayerActor interface {")
	requireNotContains(t, client, "func NewPlayerKind(handler PlayerActor, state any, opts ...actor.PropsOption) *cluster.Kind")

	requireContains(t, server, "type PlayerActor interface {")
	requireContains(t, server, "func NewPlayerKind(handler PlayerActor, state any, opts ...actor.PropsOption) *cluster.Kind")
	requireNotContains(t, server, "type BaseCrossSnsGrainClient struct {")
	requireNotContains(t, server, "func GetCrossSnsGrainClient(c *cluster.Cluster, id string) *CrossSnsGrainClient")

	requireContains(t, init, "func InitServiceGrainClients(fn func() *cluster.Cluster, opts ...cluster.GrainCallOption)")
	requireContains(t, init, "baseCrossSnsGrainClient")
	requireContains(t, init, "*BaseCrossSnsGrainClient")
	requireContains(t, init, "baseCrossMailGrainClient")
	requireContains(t, init, "*BaseCrossMailGrainClient")
	requireContains(t, init, "func GetBaseCrossSnsGrainClient() *BaseCrossSnsGrainClient")
	requireContains(t, init, "func GetBaseCrossMailGrainClient() *BaseCrossMailGrainClient")
}

func TestGeneratedFixturePackagesCompile(t *testing.T) {
	cmd := exec.Command("go", "test", "./test/...")
	cmd.Dir = "."

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test generated fixtures failed: %v\n%s", err, out)
	}
}

func TestGrainactorTemplateUsesResolvedServiceOptions(t *testing.T) {
	desc := &serviceDesc{
		Name:                  "CrossSns",
		ClusterImportPath:     serviceClusterImportPath,
		ClusterImportPathName: "cluster",
		UsePlacementContext:   true,
		Kind:                  "player_equip",
		NodeType:              "game",
		Actor:                 "player",
		UseGrainactor:         true,
		Methods: []*methodDesc{
			{
				Name:    "RoleSimple",
				Input:   "RoleSimpleRequest",
				Output:  "RoleSimpleResponse",
				Index:   0,
				Options: &options.MethodOptions{},
			},
		},
	}

	got := desc.execute()

	requireContains(t, got, `const ActorKindNameCrossSns = "player_equip"`)
	requireContains(t, got, `const ActorNodeTypeCrossSns = "game"`)
	requireContains(t, got, `const ActorGroupNameCrossSns = "player"`)
	requireContains(t, got, `reqMsg := &cluster.GrainRequest{MethodIndex: 0, MessageData: bytes}`)
	requireNotContains(t, got, `MessageTypeName`)
	requireContains(t, got, `g.cluster.Request(placementContext, identity, ActorKindNameCrossSns, reqMsg, opts...)`)
}

func TestActorTemplateComposesServicesByActorGroup(t *testing.T) {
	desc := &actorDesc{
		Name:          "Player",
		Actor:         "player",
		Kind:          "player_equip",
		RouteKeyField: "server_id",
		Services: []*serviceDesc{
			{Name: "CrossSns"},
			{Name: "CrossMail"},
		},
	}

	got := desc.execute()

	requireContains(t, got, "type PlayerActor interface {")
	requireContains(t, got, "CrossSns")
	requireContains(t, got, "CrossMail")
	requireContains(t, got, "type playerHandler struct {")
	requireContains(t, got, "func (h *playerHandler) Receive(ctx grainactor.Context, req *cluster.GrainRequest) (proto.Message, error) {")
	requireContains(t, got, "switch req.MethodIndex {")
	requireContains(t, got, "func NewPlayerBaseActor(handler PlayerActor, state any, opts ...grainactor.Option) actor.Actor")
	requireContains(t, got, `grainactor.NewBaseActor("player", "player_equip", &playerHandler{handler: handler}, opts...)`)
	requireContains(t, got, `cluster.NewKind("player_equip", props)`)
}

func TestActorTemplateDispatchesByActorScopedMethodIndex(t *testing.T) {
	desc := &actorDesc{
		Name:          "Player",
		Actor:         "player",
		Kind:          "player_equip",
		RouteKeyField: "server_id",
		Methods: []*actorMethodDesc{
			{
				Service: &serviceDesc{Name: "CrossSns"},
				Method: &methodDesc{
					Name:    "RoleSimple",
					Input:   "RoleSimpleRequest",
					Output:  "RoleSimpleResponse",
					Index:   0,
					Options: &options.MethodOptions{},
				},
				Index: 0,
			},
			{
				Service: &serviceDesc{Name: "CrossMail"},
				Method: &methodDesc{
					Name:    "LoadMail",
					Input:   "LoadMailRequest",
					Output:  "LoadMailResponse",
					Index:   2,
					Options: &options.MethodOptions{},
				},
				Index: 2,
			},
		},
		Services: []*serviceDesc{
			{Name: "CrossSns"},
			{Name: "CrossMail"},
		},
	}

	got := desc.execute()

	requireContains(t, got, "case 0:")
	requireContains(t, got, "handlerCtx := grainactor.ToContext(ctx)")
	requireContains(t, got, "return h.handler.RoleSimple(handlerCtx, msg)")
	requireContains(t, got, "case 2:")
	requireContains(t, got, "return h.handler.LoadMail(handlerCtx, msg)")
	requireContains(t, got, `cluster.NewGrainErrorResponse(cluster.ErrorReason_NOT_FOUND, fmt.Sprintf("unknown grain method index %d", req.MethodIndex))`)
	requireNotContains(t, got, "Binding")
	requireNotContains(t, got, "MessageTypeName")
}

func TestActorTemplateAddsRouteKeyToHandlerContext(t *testing.T) {
	desc := &actorDesc{
		Name:          "Player",
		Actor:         "player",
		Kind:          "player_equip",
		RouteKeyField: "server_id",
		Methods: []*actorMethodDesc{
			{
				Service: &serviceDesc{Name: "CrossSns", Actor: "player", RouteKeyField: "server_id"},
				Method: &methodDesc{
					Name:    "Keepalive",
					Input:   "KeepaliveRequest",
					Output:  "KeepaliveResponse",
					Index:   0,
					Options: &options.MethodOptions{},
				},
				Index: 0,
			},
		},
		Services: []*serviceDesc{
			{Name: "CrossSns"},
		},
	}

	got := desc.execute()

	requireContains(t, got, "type playerServerIDKey struct{}")
	requireContains(t, got, "func GetServerIDKey(ctx context.Context) (uint64, error) {")
	requireContains(t, got, `return 0, fmt.Errorf("missing route key server_id")`)
	requireContains(t, got, `return nil, fmt.Errorf("missing route key server_id")`)
	requireContains(t, got, "handlerCtx := grainactor.ToContext(ctx, grainactor.WithValue(playerServerIDKey{}, req.RouteKey))")
	requireContains(t, got, "return h.handler.Keepalive(handlerCtx, msg)")
}

func TestActorClientTemplateAddsRouteKeyHelpers(t *testing.T) {
	desc := &actorDesc{
		Name:          "Player",
		Actor:         "player",
		Kind:          "player_equip",
		RouteKeyField: "server_id",
	}

	got := desc.executeClient()

	requireContains(t, got, "func WithServerIDKey(serverID uint64) cluster.GrainCallOption {")
	requireContains(t, got, "return cluster.WithRouteKey(serverID)")
	requireContains(t, got, "func routeKeyFromServerIDOptions(opts []cluster.GrainCallOption) (uint64, error)")
	requireContains(t, got, `return 0, fmt.Errorf("missing route key option WithServerIDKey")`)
	requireNotContains(t, got, "func GetServerIDKey(ctx context.Context) (uint64, error)")
}

func TestBuildActorDescsAssignsActorScopedMethodIndexes(t *testing.T) {
	crossSns := &serviceDesc{
		Name:  "CrossSns",
		Kind:  "player_equip",
		Actor: "player",
		Methods: []*methodDesc{
			{Name: "RoleSimple", Index: 0},
			{Name: "Keepalive", Index: 1},
		},
	}
	crossMail := &serviceDesc{
		Name:  "CrossMail",
		Kind:  "player_equip",
		Actor: "player",
		Methods: []*methodDesc{
			{Name: "LoadMail", Index: 0},
		},
	}

	actors, err := buildActorDescs([]*serviceDesc{crossSns, crossMail})
	requireNoError(t, err)

	if len(actors) != 1 {
		t.Fatalf("len(actors) = %d, want 1", len(actors))
	}
	if crossSns.Methods[0].Index != 0 {
		t.Fatalf("RoleSimple Index = %d, want 0", crossSns.Methods[0].Index)
	}
	if crossSns.Methods[1].Index != 1 {
		t.Fatalf("Keepalive Index = %d, want 1", crossSns.Methods[1].Index)
	}
	if crossMail.Methods[0].Index != 2 {
		t.Fatalf("LoadMail Index = %d, want 2", crossMail.Methods[0].Index)
	}
	if actors[0].Methods[2].Method.Name != "LoadMail" || actors[0].Methods[2].Index != 2 {
		t.Fatalf("third actor method = %#v, want LoadMail index 2", actors[0].Methods[2])
	}
}

func TestValidateActorBaseRejectsMissingFields(t *testing.T) {
	tests := []struct {
		name    string
		base    *options.ActorBase
		contain string
	}{
		{
			name:    "missing actor",
			base:    &options.ActorBase{Kind: "k", NodeType: "n", Field: "f"},
			contain: "missing required field actor",
		},
		{
			name:    "missing kind",
			base:    &options.ActorBase{Actor: "player", NodeType: "n", Field: "f"},
			contain: "missing required field kind",
		},
		{
			name:    "missing node_type",
			base:    &options.ActorBase{Actor: "player", Kind: "k", Field: "f"},
			contain: "missing required field node_type",
		},
		{
			name:    "missing field",
			base:    &options.ActorBase{Actor: "player", Kind: "k", NodeType: "n"},
			contain: "missing required field field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateActorBase("test.proto", 0, tt.base)
			requireErrorContains(t, err, tt.contain)
		})
	}
}

func TestValidateActorBaseAcceptsCompleteEntry(t *testing.T) {
	err := validateActorBase("test.proto", 0, &options.ActorBase{
		Actor:    "player",
		Kind:     "player_equip",
		NodeType: "game",
		Field:    "server_id",
	})
	requireNoError(t, err)
}

func TestGrainactorTemplateDoesNotGenerateServiceBinding(t *testing.T) {
	desc := &serviceDesc{
		Name:                  "CrossSns",
		ClusterImportPath:     serviceClusterImportPath,
		ClusterImportPathName: "cluster",
		UsePlacementContext:   true,
		Kind:                  "player_equip",
		NodeType:              "game",
		Actor:                 "player",
		UseGrainactor:         true,
		Methods: []*methodDesc{
			{
				Name:    "RoleSimple",
				Input:   "RoleSimpleRequest",
				Output:  "RoleSimpleResponse",
				Index:   0,
				Options: &options.MethodOptions{},
			},
		},
	}

	got := desc.execute()

	requireNotContains(t, got, "Binding")
	requireNotContains(t, got, "MessageTypeName")
}

func TestGrainactorTemplateGeneratesPlacementHelpers(t *testing.T) {
	desc := &serviceDesc{
		Name:                  "CrossSns",
		ClusterImportPath:     serviceClusterImportPath,
		ClusterImportPathName: "cluster",
		UsePlacementContext:   true,
		Kind:                  "player_equip",
		NodeType:              "game",
		Actor:                 "player",
		UseGrainactor:         true,
		Methods: []*methodDesc{
			{
				Name:    "RoleSimple",
				Input:   "RoleSimpleRequest",
				Output:  "RoleSimpleResponse",
				Index:   0,
				Options: &options.MethodOptions{},
			},
		},
	}

	got := desc.execute()

	requireContains(t, got, "func (g *CrossSnsGrainClient) RoleSimple(identity string, r *RoleSimpleRequest, opts ...cluster.GrainCallOption) (*RoleSimpleResponse, error)")
	requireContains(t, got, "func (g *CrossSnsGrainClient) RoleSimpleByRouteKey(identity string, routeKey uint64, r *RoleSimpleRequest, opts ...cluster.GrainCallOption) (*RoleSimpleResponse, error)")
	requireContains(t, got, "func (g *CrossSnsGrainClient) RoleSimpleWithPlacement(placementContext *cluster.PlacementContext, identity string, r *RoleSimpleRequest, opts ...cluster.GrainCallOption) (*RoleSimpleResponse, error)")
	requireContains(t, got, `&cluster.PlacementContext{NodeType: "game", RouteKey: routeKey}`)
	requireContains(t, got, `&cluster.PlacementContext{NodeType: "game"}`)
	requireContains(t, got, "g.cluster.Request(placementContext, identity, ActorKindNameCrossSns, reqMsg, opts...)")
}

func TestGrainactorTemplateGeneratesRouteKeyOption(t *testing.T) {
	desc := &serviceDesc{
		Name:                  "CrossSns",
		ClusterImportPath:     serviceClusterImportPath,
		ClusterImportPathName: "cluster",
		UsePlacementContext:   true,
		Kind:                  "player_equip",
		NodeType:              "game",
		Actor:                 "player",
		RouteKeyField:         "server_id",
		UseGrainactor:         true,
		Methods: []*methodDesc{
			{
				Name:    "RoleSimple",
				Input:   "RoleSimpleRequest",
				Output:  "RoleSimpleResponse",
				Index:   0,
				Options: &options.MethodOptions{},
			},
		},
	}

	got := desc.execute()

	requireContains(t, got, "routeKey, err := routeKeyFromServerIDOptions(opts)")
	requireContains(t, got, "return g.RoleSimpleByRouteKey(identity, routeKey, r, opts...)")
}

func TestGrainactorTemplateGeneratesBaseClient(t *testing.T) {
	desc := &serviceDesc{
		Name:                  "CrossSns",
		ClusterImportPath:     serviceClusterImportPath,
		ClusterImportPathName: "cluster",
		UsePlacementContext:   true,
		Kind:                  "player_equip",
		NodeType:              "game",
		Actor:                 "player",
		RouteKeyField:         "server_id",
		UseGrainactor:         true,
		Methods: []*methodDesc{
			{
				Name:    "RoleSimple",
				Input:   "RoleSimpleRequest",
				Output:  "RoleSimpleResponse",
				Index:   0,
				Options: &options.MethodOptions{},
			},
			{
				Name:    "Keepalive",
				Input:   "KeepaliveRequest",
				Output:  "KeepaliveResponse",
				Index:   1,
				Options: &options.MethodOptions{Future: true},
			},
		},
	}

	got := desc.execute()

	requireContains(t, got, "type BaseCrossSnsGrainClient struct {")
	requireContains(t, got, "ClusterFn func() *cluster.Cluster")
	requireContains(t, got, "Opts      []cluster.GrainCallOption")
	requireContains(t, got, "func NewBaseCrossSnsGrainClient(c func() *cluster.Cluster, opts ...cluster.GrainCallOption) *BaseCrossSnsGrainClient")
	requireContains(t, got, "func (h *BaseCrossSnsGrainClient) RoleSimple(identity string, r *RoleSimpleRequest, opts ...cluster.GrainCallOption) (*RoleSimpleResponse, error)")
	requireContains(t, got, "cli := GetCrossSnsGrainClient(h.ClusterFn(), identity)")
	requireContains(t, got, "return cli.RoleSimple(identity, r, h.mergeOpts(opts)...)")
	requireContains(t, got, "func (h *BaseCrossSnsGrainClient) KeepaliveFuture(identity string, r *KeepaliveRequest, opts ...cluster.GrainCallOption) (actor.Future, error)")
	requireContains(t, got, "return cli.KeepaliveFuture(identity, r, h.mergeOpts(opts)...)")
	requireContains(t, got, "func (h *BaseCrossSnsGrainClient) mergeOpts(opts []cluster.GrainCallOption) []cluster.GrainCallOption")
}

func TestGrainactorTemplateGeneratesOnewaySend(t *testing.T) {
	desc := &serviceDesc{
		Name:                  "CrossSns",
		ClusterImportPath:     serviceClusterImportPath,
		ClusterImportPathName: "cluster",
		UsePlacementContext:   true,
		Kind:                  "player_equip",
		NodeType:              "game",
		Actor:                 "player",
		RouteKeyField:         "server_id",
		UseGrainactor:         true,
		Methods: []*methodDesc{
			{
				Name:    "Keepalive",
				Input:   "KeepaliveRequest",
				Output:  "KeepaliveResponse",
				Index:   1,
				Options: &options.MethodOptions{Oneway: true},
			},
		},
	}

	got := desc.execute()

	requireContains(t, got, "Keepalive(ctx context.Context, req *KeepaliveRequest) error")
	requireContains(t, got, "func (g *CrossSnsGrainClient) KeepaliveSend(identity string, r *KeepaliveRequest, opts ...cluster.GrainCallOption) error")
	requireContains(t, got, "func (g *CrossSnsGrainClient) KeepaliveSendByRouteKey(identity string, routeKey uint64, r *KeepaliveRequest, opts ...cluster.GrainCallOption) error")
	requireContains(t, got, "reqMsg := &cluster.GrainRequest{MethodIndex: 1, MessageData: bytes, OneWay: true, RouteKey: routeKey, HasRouteKey: true}")
	requireContains(t, got, "return g.cluster.Send(placementContext, identity, ActorKindNameCrossSns, reqMsg, opts...)")
	requireContains(t, got, "func (h *BaseCrossSnsGrainClient) KeepaliveSend(identity string, r *KeepaliveRequest, opts ...cluster.GrainCallOption) error")
	requireNotContains(t, got, "func (g *CrossSnsGrainClient) Keepalive(identity string, r *KeepaliveRequest, opts ...cluster.GrainCallOption) (*KeepaliveResponse, error)")
}

func TestActorTemplateGeneratesOnewayHandlerDispatch(t *testing.T) {
	desc := &actorDesc{
		Name:          "Player",
		Actor:         "player",
		Kind:          "player_equip",
		RouteKeyField: "server_id",
		Methods: []*actorMethodDesc{
			{
				Service: &serviceDesc{Name: "CrossSns", Actor: "player", RouteKeyField: "server_id"},
				Method: &methodDesc{
					Name:    "Keepalive",
					Input:   "KeepaliveRequest",
					Output:  "KeepaliveResponse",
					Index:   1,
					Options: &options.MethodOptions{Oneway: true},
				},
				Index: 1,
			},
		},
		Services: []*serviceDesc{
			{Name: "CrossSns"},
		},
	}

	got := desc.execute()

	requireContains(t, got, "if err := h.handler.Keepalive(handlerCtx, msg); err != nil {")
	requireContains(t, got, "return nil, nil")
	requireNotContains(t, got, "return h.handler.Keepalive(handlerCtx, msg)")
}

func TestGrainactorTemplateGeneratesRouteKeyFuture(t *testing.T) {
	desc := &serviceDesc{
		Name:                  "CrossSns",
		ClusterImportPath:     serviceClusterImportPath,
		ClusterImportPathName: "cluster",
		UsePlacementContext:   true,
		Kind:                  "player_equip",
		NodeType:              "game",
		Actor:                 "player",
		RouteKeyField:         "server_id",
		UseGrainactor:         true,
		Methods: []*methodDesc{
			{
				Name:    "Keepalive",
				Input:   "KeepaliveRequest",
				Output:  "KeepaliveResponse",
				Index:   0,
				Options: &options.MethodOptions{Future: true},
			},
		},
	}

	got := desc.execute()

	requireContains(t, got, "func (g *CrossSnsGrainClient) KeepaliveFuture(identity string, r *KeepaliveRequest, opts ...cluster.GrainCallOption) (actor.Future, error)")
	requireContains(t, got, "routeKey, err := routeKeyFromServerIDOptions(opts)")
	requireContains(t, got, `placementContext := &cluster.PlacementContext{NodeType: "game", RouteKey: routeKey}`)
	requireContains(t, got, "reqMsg := &cluster.GrainRequest{MethodIndex: 0, MessageData: bytes, RouteKey: routeKey, HasRouteKey: true}")
	requireContains(t, got, "g.cluster.RequestFuture(placementContext, identity, ActorKindNameCrossSns, reqMsg, opts...)")
}

func TestGrainactorTemplateDoesNotMarkRouteKeyPresentForPlacementOnly(t *testing.T) {
	desc := &serviceDesc{
		Name:                  "CrossSns",
		ClusterImportPath:     serviceClusterImportPath,
		ClusterImportPathName: "cluster",
		UsePlacementContext:   true,
		Kind:                  "player_equip",
		NodeType:              "game",
		Actor:                 "player",
		RouteKeyField:         "server_id",
		UseGrainactor:         true,
		Methods: []*methodDesc{
			{
				Name:    "RoleSimple",
				Input:   "RoleSimpleRequest",
				Output:  "RoleSimpleResponse",
				Index:   0,
				Options: &options.MethodOptions{},
			},
		},
	}

	got := desc.execute()

	requireContains(t, got, "reqMsg := &cluster.GrainRequest{MethodIndex: 0, MessageData: bytes}")
	requireNotContains(t, got, "if placementContext != nil {\n\t\treqMsg.RouteKey = placementContext.RouteKey")
}

func TestBuildActorDescsRejectsDuplicateMethodNames(t *testing.T) {
	_, err := buildActorDescs([]*serviceDesc{
		{
			Name:  "CrossSns",
			Kind:  "player_equip",
			Actor: "player",
			Methods: []*methodDesc{
				{Name: "Keepalive"},
			},
		},
		{
			Name:  "CrossMail",
			Kind:  "player_equip",
			Actor: "player",
			Methods: []*methodDesc{
				{Name: "Keepalive"},
			},
		},
	})

	requireErrorContains(t, err, `actor "player" has duplicate method "Keepalive"`)
}

func TestServiceClusterGrainTemplateUsesPlacementContext(t *testing.T) {
	desc := &serviceDesc{
		Name:                  "Hello",
		Kind:                  "Hello",
		Actor:                 "Hello",
		UsePlacementContext:   true,
		ClusterImportPath:     serviceClusterImportPath,
		ClusterImportPathName: "cluster",
		Methods: []*methodDesc{
			{
				Name:    "SayHello",
				Input:   "HelloRequest",
				Output:  "HelloResponse",
				Index:   0,
				Options: &options.MethodOptions{Future: true},
			},
		},
	}

	got := desc.execute()

	requireContains(t, got, "func (g *HelloGrainClient) SayHelloFuture(placementContext *cluster.PlacementContext, r *HelloRequest, opts ...cluster.GrainCallOption) (actor.Future, error)")
	requireContains(t, got, `const ActorKindNameHello = "Hello"`)
	requireContains(t, got, "g.cluster.RequestFuture(placementContext, g.Identity, ActorKindNameHello, reqMsg, opts...)")
	requireContains(t, got, "func (g *HelloGrainClient) SayHello(placementContext *cluster.PlacementContext, r *HelloRequest, opts ...cluster.GrainCallOption) (*HelloResponse, error)")
	requireContains(t, got, "g.cluster.Request(placementContext, g.Identity, ActorKindNameHello, reqMsg, opts...)")
}

func TestDefaultGrainTemplateKeepsExistingClientShape(t *testing.T) {
	desc := &serviceDesc{
		Name:                  "Hello",
		Kind:                  "Hello",
		Actor:                 "Hello",
		ClusterImportPath:     legacyClusterImportPath,
		ClusterImportPathName: "cluster",
		Methods: []*methodDesc{
			{
				Name:    "SayHello",
				Input:   "HelloRequest",
				Output:  "HelloResponse",
				Index:   0,
				Options: &options.MethodOptions{Future: true},
			},
		},
	}

	got := desc.execute()

	requireContains(t, got, "func (g *HelloGrainClient) SayHelloFuture(r *HelloRequest, opts ...cluster.GrainCallOption) (actor.Future, error)")
	requireContains(t, got, "g.cluster.RequestFuture(g.Identity, \"Hello\", reqMsg, opts...)")
	requireContains(t, got, "func (g *HelloGrainClient) SayHello(r *HelloRequest, opts ...cluster.GrainCallOption) (*HelloResponse, error)")
	requireContains(t, got, "g.cluster.Request(g.Identity, \"Hello\", reqMsg, opts...)")
}

func TestServiceGeneratorDefaultsToServiceClusterImport(t *testing.T) {
	if clusterImportPath != serviceClusterImportPath {
		t.Fatalf("clusterImportPath = %q, want %q", clusterImportPath, serviceClusterImportPath)
	}
}

func TestSetClusterImportPathFromParameter(t *testing.T) {
	t.Cleanup(func() {
		clusterImportPath = serviceClusterImportPath
	})

	requireNoError(t, setClusterImportPath("github.com/asynkron/protoactor-go/service/cluster"))

	if clusterImportPath != serviceClusterImportPath {
		t.Fatalf("clusterImportPath = %q, want %q", clusterImportPath, serviceClusterImportPath)
	}
}

func buildProtocGenGoGrainPlugin(t *testing.T) string {
	t.Helper()

	pluginName := "protoc-gen-go-grain"
	if runtime.GOOS == "windows" {
		pluginName += ".exe"
	}
	pluginPath, err := filepath.Abs(pluginName)
	if err != nil {
		t.Fatalf("resolve plugin path: %v", err)
	}

	build := exec.Command("go", "build", "-o", pluginPath, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build protoc plugin: %v\n%s", err, out)
	}
	return pluginPath
}

func requireContains(t *testing.T, haystack string, needle string) {
	t.Helper()

	if !strings.Contains(haystack, needle) {
		t.Fatalf("generated output missing %q\n\n%s", needle, haystack)
	}
}

func requireNotContains(t *testing.T, haystack string, needle string) {
	t.Helper()

	if strings.Contains(haystack, needle) {
		t.Fatalf("generated output unexpectedly contains %q\n\n%s", needle, haystack)
	}
}

func readGeneratedFile(t *testing.T, path string) string {
	t.Helper()

	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file %q: %v", path, err)
	}

	return string(bytes)
}

func requireNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func requireErrorContains(t *testing.T, err error, needle string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error containing %q", needle)
	}
	if !strings.Contains(err.Error(), needle) {
		t.Fatalf("error %q missing %q", err, needle)
	}
}
