# PeerSession Grainactor Integration Plan

> **For agentic workers:** implement this plan task-by-task. Keep `PeerSession`
> as a local runtime handle. Do not serialize it, put it into
> `ClusterIdentity`, or send it through remote transport. Local actor control
> messages may carry the handle inside the same process.

**Goal:** Let a gateway-selected game node bind a local peer connection handle
to the corresponding grain actor while preserving normal `service/cluster`
identity lookup, static routing, and global cluster access.

**Architecture:** Routing and local capabilities stay separate. The gateway and
cluster identity lookup both use the same `PlacementContext` inputs and
`StaticRouter` algorithm, so the grain actor is activated on the same game node
selected by the peer path. Once the actor exists on that local node,
`grainactor` sends a local control message carrying the `PeerSession` handle to
the actor mailbox. The base actor assigns the handle to its runtime context and
optional receiver hook. Cluster grain requests remain serializable.

**Tech Stack:** Go, protoactor service cluster, `service/grainactor`,
`service/protobuf/protoc-gen-go-grain`, `libs/peer` gateway/game stream bridge,
Go unit and integration tests.

---

## Decisions

### Name And Shape

Use `PeerSession` as the generic runtime capability name. It should not mention
gRPC or stream in the public interface because the underlying transport can
change later.

Initial interface:

```go
type PeerSession interface {
	SendData(payload []byte) error
	Close(reason string) error
}
```

`PeerSession` is a process-local handle. It may wrap a gRPC stream today, but
that wrapper is an implementation detail owned by the peer gateway/game bridge.

### Routing Contract

The gateway-to-game target selection remains:

1. Gateway receives peer hello / route inputs such as `target_name` and route
   key.
2. Gateway builds the same `cluster.PlacementContext` used for grain lookup.
3. `IdentityLookup.Get(placementContext, clusterIdentity)` delegates placement
   to `StaticRouter`.
4. The grain actor activation lands on the game node selected by the same
   routing algorithm.

`ClusterIdentity` remains just `{Identity, Kind}`. It must not embed
`target_name`, stream state, or peer session state.

### Binding Contract

`PeerSession` must remain a local object handle. It may be carried by
`grainactor` local control messages, but those messages must be rejected before
remote transport and must not be serialized.

The binding is exposed through local helper functions plus optional receiver
hooks:

```go
func BindPeerSession(sender actor.SenderContext, pid *actor.PID, identity string, kind string, session PeerSession) error
func ClearPeerSession(sender actor.SenderContext, pid *actor.PID, identity string, kind string, session PeerSession) error
```

```go
type PeerSessionReceiver interface {
	SetPeerSession(session PeerSession)
	ClearPeerSession(session PeerSession)
}
```

```go
func PeerSessionFromContext(ctx context.Context) (PeerSession, bool)
```

The helper functions validate that the PID is local to the caller's
`ActorSystem`, then use the actor mailbox to serialize updates. The single
source of truth lives inside `grainactor.BaseActor` and is exposed to generated
handlers through `grainactor.Context` / `grainactor.ToContext`.

### Lifecycle

- Bind on peer stream open, after cluster lookup returns a PID on the current
  game node.
- If the same identity already has a session, replace the old session and close
  it with a clear reason such as `replaced`.
- Clear only the currently bound session on peer stream close. A late close from
  an old replaced session must not clear the new session.
- Clear on actor stop.
- `SendData` without a bound session should return a typed error such as
  `ErrPeerSessionNotBound`.

### Remote Access

Remote actors and cluster clients should keep sending normal grain requests.
If a remote caller asks the grain to send data or close the peer, the request is
serializable and the local grain actor uses its local `PeerSession` handle to do
the actual I/O.

---

## Implementation Plan

### Task 1: Runtime PeerSession API

**Files:**
- Modify: `service/grainactor/context.go`
- Modify: `service/grainactor/options.go`
- Modify: `service/grainactor/base_actor.go`
- Add/modify tests: `service/grainactor/*_test.go`

- [x] Add `PeerSession`, `PeerSessionReceiver`, and
  `ErrPeerSessionNotBound` to `service/grainactor`.
- [x] Extend `grainactor.Context` with a peer-session accessor, or add
  `PeerSessionFromContext(ctx context.Context)`.
- [x] Store the currently bound session inside `BaseActor` state guarded by the
  actor mailbox ordering. Avoid extra locks unless binding can happen outside
  the actor goroutine.
- [x] On `Stopped`, clear the session and notify `PeerSessionReceiver` if the
  handler or state implements it.
- [x] Add unit tests for bind, replace, clear-current-only, stopped cleanup, and
  missing session error.

### Task 2: Local Binding Entry Point

**Files:**
- Modify: `service/grainactor/base_actor.go`
- Modify: `service/grainactor/options.go`
- Modify tests under `service/grainactor`

- [x] Add a local-only binding entry point that can be called after activation.
  The API must reject remote PIDs before sending a control message.
- [x] Use local actor control messages instead of a registry. The mailbox owns
  session mutation ordering.
- [x] Ensure binding fails if the PID is remote or no local process exists.
- [x] Add tests proving the binding path rejects non-local/stopped actors and
  does not clear a newer session from an older close.

### Task 3: Generated Actor Template Support

**Files:**
- Modify: `service/protobuf/protoc-gen-go-grain/templates/actor.tmpl`
- Modify tests under `service/protobuf/protoc-gen-go-grain`

- [x] Keep generated grain request dispatch unchanged: methods still receive
  `context.Context`.
- [x] Ensure generated handlers pass the `grainactor.Context` through
  `grainactor.ToContext`, so business code can call
  `grainactor.PeerSessionFromContext(ctx)`.
- [x] If the aggregate handler implements `PeerSessionReceiver`, have
  `BaseActor` call it directly when binding changes. Do not generate
  transport-specific code.
- [x] Add generator tests confirming generated handler methods receive
  `grainactor.ToContext(ctx)` and no binding/protobuf route is generated.

### Task 4: Cluster And Static Router Compatibility

**Files:**
- Review: `service/cluster/cluster.go`
- Review: `service/identitylookup/manager.go`
- Add tests where needed under `service/cluster` or `service/identitylookup`

- [x] Keep `Cluster.Get(placementContext, identity, kind)` as the activation
  entry point.
- [x] Rely on the existing `StaticRouter` capability to consume
  `PlacementContext` route inputs and reproduce peer target selection.
- [x] Do not add `ForceLocal` as the normal path. It can remain an escape hatch,
  but same-node placement should come from identical router inputs.

### Task 5: Peer Example Integration

**Files:**
- Modify: `../peer/example/game/*`
- Modify: `../peer/example/gateway/*`
- Add/update integration tests under `../peer/example`

- [ ] Gateway builds the exact `PlacementContext` from hello / target inputs.
- [ ] Game side activates or resolves the grain actor through cluster lookup.
- [ ] Game side binds the local peer session handle to the resolved grain actor.
- [ ] Grain actor uses `PeerSession.SendData` to write back to the client.
- [ ] Grain actor uses `PeerSession.Close` for active close.
- [ ] Add a three-node test proving `game -> gateway -> client` data and close
  both pass through the peer session path.

### Task 6: Verification

Run focused tests first:

```bash
go test ./service/grainactor -count=1
go test ./service/protobuf/protoc-gen-go-grain/... -count=1
go test ./service/cluster ./service/identitylookup -count=1
```

Then run repo-wide verification:

```bash
go test ./... -count=1
```

For the peer integration, run the existing three-node example test/bench and
verify:

- the grain actor can send data to the client through `PeerSession`
- active close reaches the peer stream
- no `PeerSession` handle is serialized or routed as a message
- static router and cluster identity lookup choose the same game node

---

## Open Points

- Business handlers receive session access through both
  `grainactor.PeerSessionFromContext(ctx)` and optional
  `PeerSessionReceiver` callbacks.
- Duplicate session policy: replace the old session and close it with reason
  `replaced`.
- `Close(reason string)` is the initial transport-neutral close API.
