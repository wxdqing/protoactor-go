# Grainactor Grain Use-Case-Driven Test Cases

## Testing Style

Use use-case-driven development. Each implementation task starts by writing a failing test that expresses a user-visible behavior, then implements the smallest change that makes that behavior pass.

The tests should prefer generated fixture code and runtime behavior over checking private implementation details. String assertions on generated output are acceptable for generator behavior that has no runtime entry point yet.

## Use Case 1: Service Options Are Accepted

**Scenario:** A proto service declares `kind`, `node_type`, and `actor`.

**Input proto:**

```proto
service CrossSns {
  option (options.kind) = "player_equip";
  option (options.node_type) = "game";
  option (options.actor) = "player";

  rpc RoleSimple(RoleSimpleRequest) returns (RoleSimpleResponse) {}
}
```

**Expected behavior:**

- protoc accepts the options
- `options.pb.go` exposes generated extension descriptors
- generated grain code can read the values

**Primary test command:**

```bash
go test ./service/protobuf/protoc-gen-go-grain -run TestGenerateGrainactorOptions -count=1
```

## Use Case 2: Kind Replaces Service Name

**Scenario:** `CrossSns` declares `kind = "player_equip"`.

**Expected behavior:**

- generated `cluster.NewKind` uses `player_equip`
- generated client requests use `player_equip`
- generated constants expose the resolved kind

**Assertions:**

```go
require.Contains(t, generated, `const ActorKindNameCrossSns = "player_equip"`)
require.Contains(t, generated, `cluster.NewKind(ActorKindNameCrossSns`)
require.Contains(t, generated, `ActorKindNameCrossSns`)
```

## Use Case 3: NodeType Becomes Default Placement

**Scenario:** `CrossSns` declares `node_type = "game"` and caller uses `ByRouteKey`.

**Expected behavior:**

Generated client constructs:

```go
&cluster.PlacementContext{
  NodeType: "game",
  RouteKey: routeKey,
}
```

**Assertions:**

```go
require.Contains(t, generated, `NodeType: "game"`)
require.Contains(t, generated, `RouteKey: routeKey`)
require.Contains(t, generated, `RoleSimpleByRouteKey`)
require.Contains(t, generated, `RoleSimpleWithPlacement`)
```

## Use Case 4: RouteKey Is Dynamic

**Scenario:** Two callers invoke the same service method with different route keys.

**Expected behavior:**

- route key is a method parameter
- no route key is stored in service options
- generated `WithPlacement` accepts a full caller-supplied placement context

**Runtime test shape:**

```go
placementA := &cluster.PlacementContext{NodeType: "game", RouteKey: 1001}
placementB := &cluster.PlacementContext{NodeType: "game", RouteKey: 2002}

_, _ = client.RoleSimpleWithPlacement(placementA, "player-1", &RoleSimpleRequest{})
_, _ = client.RoleSimpleWithPlacement(placementB, "player-2", &RoleSimpleRequest{})

require.Equal(t, uint64(1001), router.calls[0].RouteKey)
require.Equal(t, uint64(2002), router.calls[1].RouteKey)
```

## Use Case 5: Multiple Services Produce One Actor Interface

**Scenario:** `CrossSns` and `CrossMail` both declare `actor = "player"` and `kind = "player_equip"`.

**Expected behavior:**

Generated code contains:

```go
type PlayerActor interface {
  CrossSns
  CrossMail
}
```

and one constructor:

```go
func NewPlayerBaseActor(handler PlayerActor, state any, opts ...grainactor.Option) actor.Actor
```

**Negative test:**

If both services define `rpc Keepalive(...)`, generation fails with an error naming actor `player` and method `Keepalive`.

## Use Case 6: Shared BaseActor State

**Scenario:** One actor group has two services. One service writes state, another reads it.

**Expected behavior:**

- one `BaseActor` instance hosts both services
- both handlers receive context containing the same state pointer

**Runtime test shape:**

```go
type playerState struct {
  keepaliveCount int
}

type handler struct{}

func (h *handler) Keepalive(ctx context.Context, req *KeepaliveRequest) (*KeepaliveResponse, error) {
  state, _ := grainactor.State[*playerState](ctx)
  state.keepaliveCount++
  return &KeepaliveResponse{}, nil
}

func (h *handler) RoleSimple(ctx context.Context, req *RoleSimpleRequest) (*RoleSimpleResponse, error) {
  state, _ := grainactor.State[*playerState](ctx)
  return &RoleSimpleResponse{Message: strconv.Itoa(state.keepaliveCount)}, nil
}
```

Expected after one keepalive call: role simple returns `"1"`.

## Use Case 7: BaseActor Delegates Method Index Requests

**Scenario:** `BaseActor` receives a `cluster.GrainRequest` with a generated actor-scoped `method_index`.

**Expected behavior:**

- generated handler receives the request
- response is sent to actor context
- unknown method index returns a grain error response

**Test command:**

```bash
go test ./service/grainactor -run TestBaseActorDelegatesMethodIndexRequestToHandler -count=1
```

## Use Case 7b: Multiple Services Share Actor-Scoped Method Indexes

**Scenario:** `CrossSns` and `CrossMail` both declare `actor = "player"`.

**Expected behavior:**

- `CrossSns.RoleSimple` uses `MethodIndex: 0`
- `CrossSns.Keepalive` uses `MethodIndex: 1`
- `CrossMail.LoadMail` uses `MethodIndex: 2`
- generated `Player` dispatcher switches on `req.MethodIndex`

**Test command:**

```bash
go test ./service/protobuf/protoc-gen-go-grain -run TestBuildActorDescsAssignsActorScopedMethodIndexes -count=1
```

## Use Case 8: Context Is Simple For Handlers

**Scenario:** Handler methods accept `context.Context`, not `cluster.GrainContext`.

**Expected behavior:**

- handler can call `grainactor.FromContext(ctx)`
- handler can read identity, kind, actor name, and state
- no generated service implementation needs `WrapCtx() func() context.Context`

**Assertions:**

```go
gctx := grainactor.FromContext(ctx)
require.Equal(t, "player-1", gctx.Identity())
require.Equal(t, "player_equip", gctx.Kind())
require.Equal(t, "player", gctx.Actor())
```

## Use Case 9: Identitylookup Receives Generated Placement

**Scenario:** Generated client calls `RoleSimpleByRouteKey("player-1", 1001, req)`.

**Expected behavior:**

The static router receives:

```go
&cluster.PlacementContext{
  NodeType: "game",
  RouteKey: 1001,
}
```

through the existing path:

```text
client -> cluster.Request -> identitylookup.Manager -> StaticRouter.Route
```

**Test command:**

```bash
go test ./service/identitylookup ./service/staticrouter -run 'Placement|StaticRouter|Route' -count=1
```

## Use Case 10: Existing Proto Services Remain Compatible

**Scenario:** A proto service does not declare `kind`, `node_type`, or `actor`.

**Expected behavior:**

- generated kind defaults to service name
- actor group defaults to service name
- no default placement context is constructed
- existing tests under `hello`, `reenter`, `error`, and `multi-services` continue to compile

**Test command:**

```bash
go test ./service/protobuf/protoc-gen-go-grain/... -count=1
```

## Test Documentation Checklist

- [x] Covers proto option acceptance
- [x] Covers kind override
- [x] Covers node type placement
- [x] Covers dynamic RouteKey
- [x] Covers actor aggregate generation
- [x] Covers shared actor state
- [x] Covers runtime dispatch
- [x] Covers simple handler context
- [x] Covers identitylookup/staticrouter propagation
- [x] Covers compatibility with existing generated services
