package main

import (
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

func TestServiceClusterGrainTemplateUsesPlacementContext(t *testing.T) {
	desc := &serviceDesc{
		Name:                  "Hello",
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
	requireContains(t, got, "g.cluster.RequestFuture(placementContext, g.Identity, \"Hello\", reqMsg, opts...)")
	requireContains(t, got, "func (g *HelloGrainClient) SayHello(placementContext *cluster.PlacementContext, r *HelloRequest, opts ...cluster.GrainCallOption) (*HelloResponse, error)")
	requireContains(t, got, "g.cluster.Request(placementContext, g.Identity, \"Hello\", reqMsg, opts...)")
}

func TestDefaultGrainTemplateKeepsExistingClientShape(t *testing.T) {
	desc := &serviceDesc{
		Name:                  "Hello",
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

func requireNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
