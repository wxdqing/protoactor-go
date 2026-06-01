# Grainactor Grain Development Requirements

## Purpose

Add a lightweight `service/cluster` runtime layer that lets generated grain services belonging to the same logical actor run inside one shared base actor. The design borrows the useful parts of `gmod-stateful`'s composed actor model while keeping this repository's service cluster API, identity lookup, static router, and generated grain code small enough to maintain.

The feature should make this proto shape natural:

```proto
service CrossSns {
  option (options.kind) = "player_equip";
  option (options.node_type) = "game";
  option (options.actor) = "player";

  rpc RoleSimple(RoleSimpleRequest) returns (RoleSimpleResponse) {}
  rpc Keepalive(KeepaliveRequest) returns (KeepaliveResponse) {}
}
```

The generated code should route `CrossSns` through the `player` actor group, use `player_equip` as the cluster kind, and default client placement to `NodeType: "game"` while leaving `RouteKey` as a dynamic call-time parameter.

## Scope

This work covers:

- service-level grain options: `kind`, `node_type`, and `actor`
- generated metadata and interfaces for grouping services by actor
- a new lightweight runtime package for composed grain actors
- generated actor dispatchers that route multiple service methods inside one base actor
- generated clients that can use default `node_type` and caller-provided `RouteKey`
- tests proving generated code, runtime dispatch, and identitylookup/staticrouter flow work together

This work does not cover:

- persistence, snapshot storage, event broker, scheduler, or mailbox extensions from `gmod-stateful`
- dependency injection framework integration
- automatic route key extraction from request fields
- replacing existing non-service `cluster` generated code

## Directory

Create the runtime package at:

```text
service/grainactor
```

Rationale:

- the feature is a service-layer runtime helper, not an identity lookup implementation
- it wraps generated grain services into actor instances
- the name is clear at call sites: `grainactor.NewBaseActor`, `grainactor.Handler`

## Proto Options

Extend `service/protobuf/protoc-gen-go-grain/options/options.proto` with service-level options:

```proto
extend google.protobuf.ServiceOptions {
  string kind = 50001;
  string node_type = 50002;
  string actor = 50003;
}
```

Semantics:

- `kind`: cluster kind used for activation and client requests. If empty, default to the service Go name.
- `node_type`: default node type used by generated clients when constructing `PlacementContext`. If empty, generated clients pass nil placement context unless the caller provides one.
- `actor`: logical actor group. Services with the same non-empty actor value share one generated actor interface and one runtime base actor kind. If empty, default to the service Go name to preserve one-service-per-actor behavior.

The new option name is `kind`, not `kind_name`.

## RouteKey

`RouteKey` remains a dynamic call-time value. It should not be encoded as a service option because the key usually comes from request data or caller context, such as player ID, role ID, zone ID, or room ID.

Generated clients should provide convenience methods:

```go
RoleSimple(identity string, req *RoleSimpleRequest, opts ...cluster.GrainCallOption) (*RoleSimpleResponse, error)
RoleSimpleByRouteKey(identity string, routeKey uint64, req *RoleSimpleRequest, opts ...cluster.GrainCallOption) (*RoleSimpleResponse, error)
RoleSimpleWithPlacement(placementContext *cluster.PlacementContext, identity string, req *RoleSimpleRequest, opts ...cluster.GrainCallOption) (*RoleSimpleResponse, error)
```

For a service with `node_type = "game"`, `RoleSimpleByRouteKey` constructs:

```go
&cluster.PlacementContext{
  NodeType: "game",
  RouteKey: routeKey,
}
```

The plain method uses `NodeType: "game"` and `RouteKey: 0` only when `node_type` is configured. The `WithPlacement` method is the escape hatch for custom routing parameters.

## Runtime Model

The new `grainactor` package should provide a minimal composed actor:

```go
type BaseActor struct {
  // private runtime fields
}

type Handler interface {
  Receive(ctx Context, req *cluster.GrainRequest) (proto.Message, error)
}

type Context interface {
  context.Context
  GrainContext() cluster.GrainContext
  Identity() string
  Kind() string
  Actor() string
  State() any
}
```

The base actor should:

- initialize one `cluster.GrainContext` on `ClusterInit`
- keep one shared state object per actor instance
- delegate `cluster.GrainRequest` to one generated actor dispatcher
- dispatch methods by actor-scoped `method_index`
- call lifecycle hooks once for the actor, not once per service
- respond with generated proto responses or `cluster.GrainErrorResponse`

The first implementation should avoid `WrapCtx() func() context.Context`. The generated handler methods should receive a normal `context.Context`, and `grainactor` should store actor metadata in that context.

## Generated Actor Aggregation

For every actor group, generate one aggregate interface:

```go
type PlayerActor interface {
  CrossSns
  PlayerMail
  PlayerEquip
}
```

Rules:

- group by `option (options.actor)`
- if `actor` is empty, the group name is the service name
- duplicate method names inside the same actor group are generation errors
- services inside one actor group must resolve to the same `kind` and `node_type`
- generated service interfaces keep their own method sets
- aggregate interfaces compose service interfaces; they do not duplicate each method signature

For every actor group, generate one base actor constructor:

```go
func NewPlayerBaseActor(handler PlayerActor, state any, opts ...grainactor.Option) actor.Actor
```

Generated actor dispatchers should be registered into this one actor instead of producing one actor per service.

## Method Index Dispatch

Grainactor routing uses `cluster.GrainRequest.method_index`. For actor groups containing multiple services, generated indexes are scoped to the actor group, not the individual service. Given:

```proto
service CrossSns {
  option (options.actor) = "player";
  rpc RoleSimple(RoleSimpleRequest) returns (RoleSimpleResponse) {}
  rpc Keepalive(KeepaliveRequest) returns (KeepaliveResponse) {}
}

service CrossMail {
  option (options.actor) = "player";
  rpc LoadMail(LoadMailRequest) returns (LoadMailResponse) {}
}
```

the generated actor dispatcher uses:

```go
switch req.MethodIndex {
case 0: // CrossSns.RoleSimple
case 1: // CrossSns.Keepalive
case 2: // CrossMail.LoadMail
}
```

The `CrossMail` client must send `MethodIndex: 2`, not service-local index `0`.

## Context And State

The generated method signature should be:

```go
Method(ctx context.Context, req *Request) (*Response, error)
```

The runtime package should expose helpers:

```go
func FromContext(ctx context.Context) Context
func State[T any](ctx context.Context) (T, bool)
```

This lets external implementations keep temporary memory state in one actor:

```go
type PlayerState struct {
  OnlineAt time.Time
  Dirty    bool
}

type PlayerHandler struct {
}

func (h *PlayerHandler) Keepalive(ctx context.Context, req *KeepaliveRequest) (*KeepaliveResponse, error) {
  state, _ := grainactor.State[*PlayerState](ctx)
  state.OnlineAt = time.Now()
  return &KeepaliveResponse{}, nil
}
```

## Identitylookup And Static Router Flow

Generated clients must keep using the existing service cluster path:

```text
Generated client
  -> cluster.Request(placementContext, identity, kind, grainRequest, opts...)
  -> identitylookup.StorageIdentityLookup.Get(placementContext, clusterIdentity)
  -> identitylookup.Manager.selectMember(...)
  -> StaticRouter.Route(placementContext, clusterIdentity, members)
```

No staticrouter dependency should be added to the generator or `grainactor`.

## Error Handling

Generation errors:

- duplicate actor group method names
- service methods without input or output messages
- invalid option combinations that produce no kind
- actor group with services that resolve to different `kind` or `node_type` values, unless explicitly allowed later

Runtime errors:

- unknown `method_index`: respond with `GrainErrorResponse` reason `NOT_FOUND` or a new internal reason if available
- unmarshal failure: respond with `INVALID_ARGUMENT`
- handler error: wrap with `cluster.FromError`
- panic in handler: recover and respond with an internal error

## Compatibility

Existing generated service code should continue to compile when no new options are used. The default behavior remains one service, one actor group, and kind equal to the service name.

The new generator output can be introduced behind new templates and tests without changing existing handwritten service implementations in one step.

## Documentation Completion Checklist

- [x] Defines the target runtime directory
- [x] Defines `kind`, `node_type`, and `actor` option semantics
- [x] Defines RouteKey as call-time data
- [x] Defines generated actor aggregation behavior
- [x] Defines generated client placement behavior
- [x] Defines runtime dispatch and context expectations
- [x] Defines identitylookup/staticrouter integration boundary
- [x] Defines compatibility constraints
