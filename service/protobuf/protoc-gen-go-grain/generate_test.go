package main

import (
	"os/exec"
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
		Name:  "Player",
		Actor: "player",
		Kind:  "player_equip",
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
		Name:  "Player",
		Actor: "player",
		Kind:  "player_equip",
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
	requireContains(t, got, "return h.handler.RoleSimple(ctx, msg)")
	requireContains(t, got, "case 2:")
	requireContains(t, got, "return h.handler.LoadMail(ctx, msg)")
	requireContains(t, got, `cluster.NewGrainErrorResponse(cluster.ErrorReason_NOT_FOUND, fmt.Sprintf("unknown grain method index %d", req.MethodIndex))`)
	requireNotContains(t, got, "Binding")
	requireNotContains(t, got, "MessageTypeName")
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

func TestBuildActorDescsRejectsNodeTypeMismatch(t *testing.T) {
	_, err := buildActorDescs([]*serviceDesc{
		{Name: "CrossSns", Kind: "player_equip", NodeType: "game", Actor: "player"},
		{Name: "CrossMail", Kind: "player_equip", NodeType: "mail", Actor: "player"},
	})

	requireErrorContains(t, err, `actor "player" uses multiple node types`)
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

func TestBuildActorDescsRejectsKindMismatch(t *testing.T) {
	_, err := buildActorDescs([]*serviceDesc{
		{Name: "CrossSns", Kind: "player_equip", Actor: "player"},
		{Name: "CrossMail", Kind: "player_mail", Actor: "player"},
	})

	requireErrorContains(t, err, `actor "player" uses multiple kinds`)
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
