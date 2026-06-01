# Grainactor Grain Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add generated grain actor grouping so multiple services that declare the same actor run inside one shared `service/cluster/grainactor` base actor.

**Architecture:** Extend service-level proto options, teach the generator to group services by actor, and add a lightweight runtime package that dispatches generated grain requests to service bindings. Generated clients continue to use `service/cluster` placement APIs so `node_type` and call-time `RouteKey` flow into identitylookup/staticrouter without new coupling.

**Tech Stack:** Go, protobuf `protogen`, `service/protobuf/protoc-gen-go-grain`, `service/cluster`, `service/cluster/grainactor`, standard Go tests and generated-code compile tests.

---

## File Map

- Modify `service/protobuf/protoc-gen-go-grain/options/options.proto`: add service-level options.
- Regenerate `service/protobuf/protoc-gen-go-grain/options/options.pb.go`.
- Modify `service/protobuf/protoc-gen-go-grain/generate.go`: parse service options, group services by actor, validate collisions.
- Modify `service/protobuf/protoc-gen-go-grain/template.go`: add templates for actor aggregates and bindings.
- Modify `service/protobuf/protoc-gen-go-grain/templates/grain.tmpl`: use generated binding/runtime flow.
- Create `service/protobuf/protoc-gen-go-grain/templates/actor.tmpl`: aggregate actor interface and constructor output.
- Create `service/protobuf/protoc-gen-go-grain/templates/binding.tmpl`: generated service binding output.
- Create `service/cluster/grainactor/context.go`: actor-aware context wrapper.
- Create `service/cluster/grainactor/base_actor.go`: shared base actor runtime.
- Create `service/cluster/grainactor/binding.go`: binding interfaces and request dispatch helpers.
- Create `service/cluster/grainactor/options.go`: runtime options such as timeout and state factory.
- Create or modify `service/protobuf/protoc-gen-go-grain/test/grainactor/grainactor.proto`: generator fixture.
- Create generated fixture files under `service/protobuf/protoc-gen-go-grain/test/grainactor`.
- Add tests in `service/cluster/grainactor`.
- Add generator tests in `service/protobuf/protoc-gen-go-grain`.

## Task 1: Service Option Contract

**Files:**
- Modify: `service/protobuf/protoc-gen-go-grain/options/options.proto`
- Modify generated: `service/protobuf/protoc-gen-go-grain/options/options.pb.go`
- Test: `service/protobuf/protoc-gen-go-grain/test/grainactor/grainactor.proto`

- [ ] **Step 1: Write proto fixture using new options**

Create `service/protobuf/protoc-gen-go-grain/test/grainactor/grainactor.proto`:

```proto
syntax = "proto3";

package grainactor;

import "service/protobuf/protoc-gen-go-grain/options/options.proto";

option go_package = "github.com/asynkron/protoactor-go/service/protobuf/protoc-gen-go-grain/test/grainactor";

message RoleSimpleRequest {
  uint64 role_id = 1;
}

message RoleSimpleResponse {
  string message = 1;
}

message KeepaliveRequest {
  uint64 role_id = 1;
}

message KeepaliveResponse {
  int64 server_time = 1;
}

message LoadMailRequest {
  uint64 role_id = 1;
}

message LoadMailResponse {
  int32 count = 1;
}

service CrossSns {
  option (options.kind) = "player_equip";
  option (options.node_type) = "game";
  option (options.actor) = "player";

  rpc RoleSimple(RoleSimpleRequest) returns (RoleSimpleResponse) {}
  rpc Keepalive(KeepaliveRequest) returns (KeepaliveResponse) {}
}

service CrossMail {
  option (options.kind) = "player_equip";
  option (options.node_type) = "game";
  option (options.actor) = "player";

  rpc LoadMail(LoadMailRequest) returns (LoadMailResponse) {}
}
```

- [ ] **Step 2: Run generation to verify RED**

Run:

```bash
go test ./service/protobuf/protoc-gen-go-grain -run TestGenerateGrainactor -count=1
```

Expected: fail because `options.kind`, `options.node_type`, and `options.actor` are not defined.

- [ ] **Step 3: Add service option extensions**

Update `options.proto`:

```proto
extend google.protobuf.ServiceOptions {
  string kind = 50001;
  string node_type = 50002;
  string actor = 50003;
}
```

- [ ] **Step 4: Regenerate option bindings**

Run:

```bash
protoc --go_out=. --go_opt=paths=source_relative service/protobuf/protoc-gen-go-grain/options/options.proto
```

Expected: `options.pb.go` contains `E_Kind`, `E_NodeType`, and `E_Actor`.

- [ ] **Step 5: Commit**

```bash
git add service/protobuf/protoc-gen-go-grain/options/options.proto service/protobuf/protoc-gen-go-grain/options/options.pb.go service/protobuf/protoc-gen-go-grain/test/grainactor/grainactor.proto
git commit -m "protobuf: add grain actor service options"
```

## Task 2: Generator Option Parsing And Validation

**Files:**
- Modify: `service/protobuf/protoc-gen-go-grain/generate.go`
- Modify: `service/protobuf/protoc-gen-go-grain/template.go`
- Test: `service/protobuf/protoc-gen-go-grain/generate_test.go`

- [ ] **Step 1: Add failing parser tests**

Add test helpers that compile the fixture and assert generated output contains:

```go
type PlayerActor interface {
	CrossSns
	CrossMail
}
```

and:

```go
const ActorKindNameCrossSns = "player_equip"
const ActorNodeTypeCrossSns = "game"
const ActorGroupNameCrossSns = "player"
```

Run:

```bash
go test ./service/protobuf/protoc-gen-go-grain -run TestGenerateGrainactor -count=1
```

Expected: fail because generator does not parse service options.

- [ ] **Step 2: Extend descriptors**

Add fields:

```go
type serviceDesc struct {
	Name                  string
	Methods               []*methodDesc
	ClusterImportPath     string
	ClusterImportPathName string
	UsePlacementContext   bool
	Kind                  string
	NodeType              string
	Actor                 string
}

type actorDesc struct {
	Name     string
	Kind     string
	NodeType string
	Services []*serviceDesc
}
```

- [ ] **Step 3: Parse service options**

In `generateService`, read:

```go
kind := string(service.GoName)
nodeType := ""
actorName := string(service.GoName)

if opts := service.Desc.Options(); opts != nil {
	if v, ok := proto.GetExtension(opts, options.E_Kind).(string); ok && v != "" {
		kind = v
	}
	if v, ok := proto.GetExtension(opts, options.E_NodeType).(string); ok {
		nodeType = v
	}
	if v, ok := proto.GetExtension(opts, options.E_Actor).(string); ok && v != "" {
		actorName = v
	}
}
```

Use a helper to avoid panics when the extension is absent.

- [ ] **Step 4: Validate actor grouping**

Rules:

```text
same actor group + different non-empty kind => generation error
same actor group + duplicate method name => generation error
empty kind after defaulting => generation error
```

Return errors from generation instead of panicking where practical.

- [ ] **Step 5: Run parser tests**

Run:

```bash
go test ./service/protobuf/protoc-gen-go-grain -run TestGenerateGrainactor -count=1
```

Expected: pass option parsing assertions.

- [ ] **Step 6: Commit**

```bash
git add service/protobuf/protoc-gen-go-grain/generate.go service/protobuf/protoc-gen-go-grain/template.go service/protobuf/protoc-gen-go-grain/generate_test.go
git commit -m "protobuf: parse grain actor service options"
```

## Task 3: Grainactor Runtime Package

**Files:**
- Create: `service/cluster/grainactor/context.go`
- Create: `service/cluster/grainactor/binding.go`
- Create: `service/cluster/grainactor/options.go`
- Create: `service/cluster/grainactor/base_actor.go`
- Test: `service/cluster/grainactor/base_actor_test.go`

- [ ] **Step 1: Write failing context test**

Create `service/cluster/grainactor/context_test.go`:

```go
func TestContextCarriesActorMetadataAndState(t *testing.T) {
	state := &struct{ Count int }{}
	ctx := newContext(context.Background(), nil, "player-1", "player_equip", "player", state)

	if ctx.Identity() != "player-1" {
		t.Fatalf("Identity() = %q", ctx.Identity())
	}
	got, ok := State[*struct{ Count int }](ctx)
	if !ok || got != state {
		t.Fatalf("State() = %#v, %v", got, ok)
	}
}
```

- [ ] **Step 2: Implement context**

Implement:

```go
type Context interface {
	context.Context
	GrainContext() cluster.GrainContext
	Identity() string
	Kind() string
	Actor() string
	State() any
}

func FromContext(ctx context.Context) Context
func State[T any](ctx context.Context) (T, bool)
```

- [ ] **Step 3: Write failing dispatch test**

Use a fake binding:

```go
type fakeBinding struct {
	requestType string
	called      bool
}

func (b *fakeBinding) ActorName() string { return "player" }
func (b *fakeBinding) Kind() string { return "player_equip" }
func (b *fakeBinding) Requests() []proto.Message { return []proto.Message{&testRequest{}} }
func (b *fakeBinding) Receive(ctx grainactor.Context, req *cluster.GrainRequest) (proto.Message, error) {
	b.called = true
	return &testResponse{}, nil
}
```

Expected behavior: `BaseActor` routes by `MessageTypeName` and invokes the binding.

- [ ] **Step 4: Implement BaseActor**

Provide:

```go
func NewBaseActor(actorName string, kind string, bindings []Binding, opts ...Option) actor.Actor
```

Options:

```go
func WithState(state any) Option
func WithStateFactory(fn func(identity string) any) Option
func WithReceiveTimeout(timeout time.Duration) Option
```

Runtime behavior:

- on `ClusterInit`, create `cluster.GrainContext`
- on `GrainRequest`, find binding by `MessageTypeName`
- on handler error, respond with `cluster.FromError(err)`
- on unknown message type, respond with `cluster.NewGrainErrorResponse(cluster.ErrorReason_NOT_FOUND, ...)` if available, otherwise internal error

- [ ] **Step 5: Run runtime tests**

Run:

```bash
go test ./service/cluster/grainactor -count=1
```

Expected: pass.

- [ ] **Step 6: Commit**

```bash
git add service/cluster/grainactor
git commit -m "service: add grainactor runtime"
```

## Task 4: Generate Actor Aggregates And Bindings

**Files:**
- Create: `service/protobuf/protoc-gen-go-grain/templates/actor.tmpl`
- Create: `service/protobuf/protoc-gen-go-grain/templates/binding.tmpl`
- Modify: `service/protobuf/protoc-gen-go-grain/template.go`
- Modify: `service/protobuf/protoc-gen-go-grain/generate.go`
- Test: generated fixture in `service/protobuf/protoc-gen-go-grain/test/grainactor`

- [ ] **Step 1: Write failing golden assertions**

Assert generated file contains:

```go
type PlayerActor interface {
	CrossSns
	CrossMail
}

func NewPlayerBaseActor(handler PlayerActor, state any, opts ...grainactor.Option) actor.Actor
```

and binding constructors:

```go
func NewCrossSnsBinding(handler CrossSns) grainactor.Binding
func NewCrossMailBinding(handler CrossMail) grainactor.Binding
```

- [ ] **Step 2: Generate actor aggregate template**

Template output:

```go
type {{ .Name }}Actor interface {
{{ range .Services }}	{{ .Name }}
{{ end }}}
```

- [ ] **Step 3: Generate binding template**

Each binding should:

- return actor name
- return kind
- return request proto messages
- unmarshal request
- call handler method with `grainactor.Context`
- marshal or return proto response

- [ ] **Step 4: Generate base actor constructor**

Constructor:

```go
func NewPlayerBaseActor(handler PlayerActor, state any, opts ...grainactor.Option) actor.Actor {
	bindings := []grainactor.Binding{
		NewCrossSnsBinding(handler),
		NewCrossMailBinding(handler),
	}
	if state != nil {
		opts = append(opts, grainactor.WithState(state))
	}
	return grainactor.NewBaseActor("player", "player_equip", bindings, opts...)
}
```

- [ ] **Step 5: Run generator tests**

Run:

```bash
go test ./service/protobuf/protoc-gen-go-grain -run TestGenerateGrainactor -count=1
```

Expected: pass.

- [ ] **Step 6: Commit**

```bash
git add service/protobuf/protoc-gen-go-grain
git commit -m "protobuf: generate grain actor bindings"
```

## Task 5: Generated Client Placement Convenience

**Files:**
- Modify: `service/protobuf/protoc-gen-go-grain/templates/grain.tmpl`
- Test: generated fixture in `service/protobuf/protoc-gen-go-grain/test/grainactor`

- [ ] **Step 1: Write failing generated client assertions**

Assert generated client contains:

```go
func (g *CrossSnsGrainClient) RoleSimpleByRouteKey(identity string, routeKey uint64, r *RoleSimpleRequest, opts ...cluster.GrainCallOption) (*RoleSimpleResponse, error)

func (g *CrossSnsGrainClient) RoleSimpleWithPlacement(placementContext *cluster.PlacementContext, identity string, r *RoleSimpleRequest, opts ...cluster.GrainCallOption) (*RoleSimpleResponse, error)
```

and that `ByRouteKey` constructs:

```go
&cluster.PlacementContext{NodeType: "game", RouteKey: routeKey}
```

- [ ] **Step 2: Update client template**

Generate three method variants:

- plain method using default placement when `node_type` is set
- `ByRouteKey`
- `WithPlacement`

- [ ] **Step 3: Run generator tests**

Run:

```bash
go test ./service/protobuf/protoc-gen-go-grain -run TestGenerateGrainactor -count=1
```

Expected: pass.

- [ ] **Step 4: Commit**

```bash
git add service/protobuf/protoc-gen-go-grain
git commit -m "protobuf: generate grain placement helpers"
```

## Task 6: Identitylookup And Static Router Verification

**Files:**
- Modify tests only unless failures reveal a missing propagation path:
  - `service/identitylookup/manager_test.go`
  - `service/staticrouter/router_test.go`
  - `service/cluster/placement_context_test.go`

- [ ] **Step 1: Add generated-client-equivalent placement test**

In `service/identitylookup/manager_test.go`, assert:

```go
placementContext := &cluster.PlacementContext{NodeType: "game", RouteKey: 1001}
```

is passed unchanged to `StaticRouter.Route`.

- [ ] **Step 2: Add static router route key test**

In `service/staticrouter/router_test.go`, assert a static route with `NodeType: "game"` and `RouteKey: 1001` selects the expected base member.

- [ ] **Step 3: Run routing tests**

Run:

```bash
go test ./service/identitylookup ./service/staticrouter ./service/cluster -run 'Placement|StaticRouter|Route' -count=1
```

Expected: pass.

- [ ] **Step 4: Commit**

```bash
git add service/identitylookup service/staticrouter service/cluster
git commit -m "service: verify grain placement routing"
```

## Final Verification

- [ ] Run generator tests:

```bash
go test ./service/protobuf/protoc-gen-go-grain/... -count=1
```

- [ ] Run grainactor runtime tests:

```bash
go test ./service/cluster/grainactor -count=1
```

- [ ] Run placement-related tests:

```bash
go test ./service/cluster ./service/identitylookup ./service/staticrouter -count=1
```

- [ ] Run lint:

```bash
make lint
```

- [ ] Run all tests possible in the local environment:

```bash
go test ./... -count=1
```

If external Consul, etcd, Redis, or ZooKeeper dependencies are unavailable, record the exact failing package and command output.

## Plan Self-Review Checklist

- [x] Requirements from the design doc map to tasks
- [x] No task depends on an undefined file path
- [x] TDD red/green steps are included for each implementation area
- [x] RouteKey is call-time data, not a proto option
- [x] Runtime package is `service/cluster/grainactor`
- [x] Implementation tasks are split into commit-sized changes
