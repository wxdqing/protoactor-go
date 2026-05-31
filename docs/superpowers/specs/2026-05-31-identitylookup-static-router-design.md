# Identitylookup Static Router Design

## Goal

Adapt `service/identitylookup` to the current `service/cluster.IdentityLookup` interface and make activation placement support exactly one selection strategy at a time:

- storage reuse for already-active identities
- static routing for new activations when `cluster.Config.StaticRouter` is configured
- default member strategy for new activations only when no static router is configured

## Decisions

### IdentityLookup Interface

`StorageIdentityLookup` must implement:

```go
Get(placementContext *cluster.PlacementContext, clusterIdentity *cluster.ClusterIdentity) *actor.PID
RemovePid(placementContext *cluster.PlacementContext, clusterIdentity *cluster.ClusterIdentity, pid *actor.PID)
Setup(cluster *cluster.Cluster, kinds []string, isClient bool)
Shutdown()
```

`RemovePid` accepts `placementContext` for interface compatibility but does not use it. Storage removal remains keyed by `clusterIdentity` and `pid`.

### Lookup Order

`Get` keeps storage as the source of truth for existing activations:

1. Read `IdentityStorage.TryGetExistingActivation`.
2. If the activation has a PID and its `MemberID` resolves to a live member with a matching epoch, return the PID.
3. If no valid PID exists, acquire the storage activation lock.
4. If another node owns the lock, wait for activation and return the stored PID.
5. If this node owns the lock, select a target member and request activation.

### Member Selection

`selectMember` follows this order:

1. If storage contains a `memberID`, try to reuse that member. It must exist in current topology. The existing epoch validation remains part of PID reuse. For lock reuse, absence means continue to the configured placement strategy.
2. If `cluster.Config.StaticRouter` is configured, call `Route(placementContext, clusterIdentity, currentMembers)`.
3. If static routing returns no member, activation fails immediately.
4. If no static router is configured, use the existing `MemberStrategy.GetActivator`.

Static and non-static placement are mutually exclusive. A configured static router is authoritative for new activations. It never falls back to random/default member selection when it cannot route.

### Manual Kind Removal

Manual kind support is removed from `service/identitylookup`:

- no `manualKinds` field
- no `Activate` entry point
- no manual proxy actor
- no `ManualActivateRequest`
- `Get` always attempts activation when no valid PID exists

The only activator actor is `storage-activator`.

### Topology Snapshot

Static routing needs current members in cluster format. `Manager` should retain the latest topology members as `cluster.Members` alongside the existing `MemberStrategy` update. This keeps static routing independent from the private maps inside `member_strategy`.

### Failure Behavior

- Storage lookup error: log and return nil.
- Existing PID with missing/stale member: treat as invalid and continue to activation.
- Lock acquisition error other than lock contention: log and return nil.
- Static router configured but placement context is missing or route cannot be resolved: return nil.
- Static router resolves a base node that is not currently in topology: return nil.
- Default strategy has no candidate: return nil.

## Test Strategy

Use TDD/BDD with behavior-focused tests:

- `StorageIdentityLookup` satisfies `cluster.IdentityLookup`.
- `Get` passes `PlacementContext` into manager placement.
- Static router success chooses the routed member.
- Static router miss fails activation without calling default strategy.
- Static router target missing fails activation without calling default strategy.
- No static router keeps existing default member strategy behavior.
- Manual kind path is absent from the public `StorageIdentityLookup` API.

The initial implementation can test selection behavior directly at the manager level with small fakes, avoiding remote actor spawning where possible.
