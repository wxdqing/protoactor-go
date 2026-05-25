# Service Cluster IdentityLookup Placement Context Spec

## Purpose

`service/cluster` is the target package for migrating the current `cluster` functionality. The first behavior change is to make `IdentityLookup` receive a caller-provided `PlacementContext` so lookup implementations can consume explicit placement and routing parameters.

This document is the baseline for later code changes.

## Scope

The migration moves the current `cluster` package behavior into `service/cluster`.

The new package should include the equivalent cluster runtime, identity lookup implementations, providers, metrics, pub-sub, grain support, generated protobuf code, and test helpers needed for `service/cluster` to operate independently from the old `cluster` import path.

The old `cluster` package can remain unchanged during the initial migration unless a later task explicitly asks for compatibility shims or deprecation work.

## IdentityLookup API

The `service/cluster.PlacementContext` type must be:

```go
type PlacementContext struct {
	NodeType   string
	RouteKey   uint64
	Affinity   string
	ForceLocal bool
	Labels     map[string]string
}
```

The `service/cluster.IdentityLookup` interface must be:

```go
type IdentityLookup interface {
	Get(placementContext *PlacementContext, clusterIdentity *ClusterIdentity) *actor.PID
	RemovePid(placementContext *PlacementContext, clusterIdentity *ClusterIdentity, pid *actor.PID)
	Setup(cluster *Cluster, kinds []string, isClient bool)
	Shutdown()
}
```

`Get` and `RemovePid` must accept the `PlacementContext` supplied by the public cluster call path.

`Setup` and `Shutdown` do not need placement context in this migration.

## PlacementContext Semantics

The `placementContext *PlacementContext` argument is a dynamic parameter carrier for identity lookup behavior.

It is intended for explicit placement and routing inputs:

- `NodeType` selects or hints the desired node category.
- `RouteKey` provides a stable numeric routing key for custom placement strategies.
- `Affinity` provides an affinity key for colocating or preferring related activations.
- `ForceLocal` requests local placement when the identity lookup implementation supports it.
- `Labels` carries additional implementation-specific placement labels.

`PlacementContext` is intentionally narrower than `context.Context`. It must not be used for cancellation, deadlines, tracing APIs, or generic request storage.

Implementations should treat `PlacementContext` as read-only during lookup. If an implementation needs to store or mutate `Labels`, it should copy the map first.

## Static Router Integration Boundary

`service/cluster` should reserve an abstraction for static routing, but it should not vendor or directly implement the static routing algorithm in the first migration.

The concrete static routing implementation is expected to come from:

```text
https://gitee.com/wxdqing/staticrouter.git
```

At the `service/cluster` layer, define only the routing contract needed by identity lookup and placement.

The reserved interface should be small and independent from the external repository's concrete types:

```go
type StaticRouter interface {
	Route(placementContext *PlacementContext, clusterIdentity *ClusterIdentity, members Members) (*Member, bool)
}
```

The `Route` method receives:

- the caller-provided `PlacementContext`
- the target `ClusterIdentity`
- the current cluster members known to the service cluster runtime

The method returns:

- the selected member and `true` when routing succeeds
- `nil` and `false` when no member can be selected

The `IdentityLookup` implementation may use this router before falling back to the default placement strategy. The fallback policy must be explicit in the implementation or configuration.

`service/cluster` should provide a config hook for the router:

```go
type Config struct {
	// existing fields...
	StaticRouter StaticRouter
}
```

and a config option:

```go
func WithStaticRouter(router StaticRouter) ConfigOption
```

The default value should be `nil`, preserving current distributed-hash behavior.

Future staticrouter integration should be implemented as an adapter package or external adapter, for example:

```text
service/cluster/staticrouteradapter
```

or an external module that implements `service/cluster.StaticRouter`.

The service cluster core must depend only on the `StaticRouter` interface, not on the concrete `staticrouter` package.

## Public Call Path

`service/cluster` should expose placement-context-aware APIs as the primary dynamic lookup path:

```go
func (c *Cluster) Get(placementContext *PlacementContext, identity string, kind string) *actor.PID

func (c *Cluster) Request(placementContext *PlacementContext, identity string, kind string, message interface{}, options ...GrainCallOption) (interface{}, error)

func (c *Cluster) RequestFuture(placementContext *PlacementContext, identity string, kind string, message interface{}, options ...GrainCallOption) (actor.Future, error)
```

The caller-provided `PlacementContext` should flow through:

```text
Cluster.Request(placementContext, ...)
  -> DefaultContext.Request(placementContext, ...)
  -> getPid(placementContext, ...)
  -> Cluster.Get(placementContext, ...)
  -> IdentityLookup.Get(placementContext, ...)
```

For direct PID resolution:

```text
Cluster.Get(placementContext, identity, kind)
  -> IdentityLookup.Get(placementContext, NewClusterIdentity(identity, kind))
```

For PID removal:

```text
caller-provided placementContext
  -> IdentityLookup.RemovePid(placementContext, clusterIdentity, pid)
```

Convenience methods with no placement context may be added if needed, but they must pass `nil` or an empty `PlacementContext` into the placement-aware path instead of bypassing it.

## Internal Timeout Flow

Request timeout handling should remain separate from placement metadata propagation.

The existing request timeout and retry behavior should continue to use the current `GrainCallConfig.Timeout`, retry loop, and actor `RequestFuture` timeout behavior.

No timeout or cancellation semantics should be added to `PlacementContext`.

Inside request handling, timeout state must not overwrite or replace the caller-provided placement context.

## Default disthash Behavior

The default distributed hash identity lookup should accept the new placement context parameter.

If it has no dynamic parameters to consume yet, it may ignore the value:

```go
func (p *IdentityLookup) Get(placementContext *cluster.PlacementContext, clusterIdentity *cluster.ClusterIdentity) *actor.PID {
	return p.partitionManager.Get(placementContext, clusterIdentity)
}
```

`Manager.Get` may also accept `placementContext *cluster.PlacementContext` to preserve the forwarding chain, even if the first implementation does not use it.

The important requirement is that custom identity lookup implementations can rely on receiving the original caller-provided placement context.

If `Config.StaticRouter` is configured, the default service cluster identity lookup may use it to choose the activation owner before using the current rendezvous hash behavior. If no router is configured, or if the router returns no member, the existing distributed hash behavior should remain the fallback unless a later design chooses strict static routing.

## Grain Tool Adaptation

The existing root grain generator under `protobuf/protoc-gen-go-grain` should remain unchanged during this migration. `service/cluster` must use a service-scoped grain generator under `service/protobuf/protoc-gen-go-grain` so the original generated-code behavior stays available for the old `cluster` package.

The service grain generator must emit generated grain code that imports the new package path:

```text
github.com/asynkron/protoactor-go/service/cluster
```

The service generator may keep a cluster import option so its behavior is explicit and testable. Its default cluster import path should be:

```text
github.com/asynkron/protoactor-go/service/cluster
```

If a build passes an explicit import path, it should pass:

```text
cluster_import=github.com/asynkron/protoactor-go/service/cluster
```

The generated code should then reference `service/cluster` for:

- `cluster.Cluster`
- `cluster.Kind`
- `cluster.GrainContext`
- `cluster.GrainCallOption`
- `cluster.GrainRequest`
- `cluster.GrainErrorResponse`
- `cluster.ClusterInit`
- helper calls such as `cluster.NewKind`, `cluster.NewGrainContext`, `cluster.FromError`, and `cluster.NewGrainErrorResponse`

Generated grain clients for `service/cluster` must call the placement-context-aware request APIs. The generated client methods should accept placement context as an explicit parameter before call options:

```go
func (g *HelloGrainClient) SayHello(placementContext *cluster.PlacementContext, req *HelloRequest, opts ...cluster.GrainCallOption) (*HelloResponse, error)

func (g *HelloGrainClient) SayHelloFuture(placementContext *cluster.PlacementContext, req *HelloRequest, opts ...cluster.GrainCallOption) (actor.Future, error)
```

The generated call should flow through:

```go
resp, err := g.cluster.Request(placementContext, g.Identity, "Hello", reqMsg, opts...)
```

For the old `cluster` package, the original root generator should keep existing output. The service generator must not require changes in `protobuf/protoc-gen-go-grain`.

`service/cluster` should have its own proto generation script or build target that:

- regenerates service cluster protobuf files with `go_package` pointing at `service/cluster`
- invokes `service/protobuf/protoc-gen-go-grain`
- keeps old `cluster/build.sh` behavior unchanged unless a later task updates both packages together

## Migration Requirements

When implementing `service/cluster`, update package paths from the old cluster package to the new service cluster package.

Key areas to cover:

- `service/cluster` root package files
- `service/cluster/identitylookup/disthash`
- `service/cluster/clusterproviders`
- `service/cluster/cluster_test_tool`
- `service/cluster/metrics`
- protobuf `go_package` values for service cluster protobuf files
- generated `*.pb.go` files
- generated grain code and service generator support where it imports the cluster package
- `service/protobuf/protoc-gen-go-grain` or equivalent service-scoped adapter for service cluster grain output
- static router interface and configuration hook, without requiring the external staticrouter implementation

The root grain generator has a hard-coded cluster import path and should be preserved as-is unless a later task explicitly changes the old generator.

## Compatibility Guidance

The new `service/cluster` API should prefer placement-context-aware methods.

If no-placement-context convenience methods are kept, they should call the placement-aware methods with `nil` or an empty `PlacementContext` and should be clearly treated as convenience wrappers.

Compatibility wrappers must not hide the placement-aware call path in request handling.

## Acceptance Criteria

The migration is complete when:

- `service/cluster.PlacementContext` exists with the fields defined in this document.
- `service/cluster.IdentityLookup` uses the placement-context-aware interface.
- Request and direct lookup paths pass caller-provided placement context to `IdentityLookup.Get`.
- PID removal paths pass caller-provided placement context to `IdentityLookup.RemovePid`.
- Existing timeout and retry behavior remains separate from placement context.
- `service/cluster` exposes a `StaticRouter` interface and config hook suitable for a later adapter over `https://gitee.com/wxdqing/staticrouter.git`.
- The default `disthash` implementation compiles with the new interface.
- Generated protobuf and grain-related code reference `service/cluster` where required.
- The grain generator can emit code that imports `github.com/asynkron/protoactor-go/service/cluster`.
- Generated service cluster grain clients pass `PlacementContext` into `Cluster.Request` and `Cluster.RequestFuture`.
- Tests cover that a value placed in the public call path `PlacementContext` reaches a custom `IdentityLookup`.
