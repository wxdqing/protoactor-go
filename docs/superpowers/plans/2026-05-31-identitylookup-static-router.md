# Identitylookup Static Router Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Adapt `service/identitylookup` to the current cluster identity lookup interface and add authoritative static-router placement for new activations.

**Architecture:** Existing storage lookup remains the source of truth for already-active identities. New activations use either `cluster.Config.StaticRouter` or the existing default `MemberStrategy`, never both. Manual kind and manual proxy behavior are removed.

**Tech Stack:** Go, `service/cluster`, `service/identitylookup`, behavior tests with the standard `testing` package.

---

## File Map

- Modify `service/identitylookup/identity_lookup.go`: update interface methods, remove manual kind/Activate API.
- Modify `service/identitylookup/manager.go`: thread `PlacementContext`, remove manual proxy, store topology members, add static-router member selection.
- Modify `service/identitylookup/types/types.go`: remove `ManualActivateRequest`.
- Modify `service/identitylookup/manual_proxy_actor.go`: delete after manual flow removal.
- Add or modify `service/identitylookup/manager_test.go`: TDD coverage for static router behavior.
- Add or modify `service/identitylookup/identity_lookup_test.go`: interface conformance and API behavior.

## Task 1: Interface Adaptation And Manual Removal

**Files:**
- Modify: `service/identitylookup/identity_lookup.go`
- Modify: `service/identitylookup/manager.go`
- Modify: `service/identitylookup/types/types.go`
- Delete: `service/identitylookup/manual_proxy_actor.go`
- Test: `service/identitylookup/identity_lookup_test.go`

- [ ] **Step 1: Write failing interface test**

Create `service/identitylookup/identity_lookup_test.go`:

```go
package identitylookup

import (
	"testing"

	"github.com/asynkron/protoactor-go/cluster"
)

func TestStorageIdentityLookupImplementsClusterIdentityLookup(t *testing.T) {
	var _ cluster.IdentityLookup = (*StorageIdentityLookup)(nil)
}
```

- [ ] **Step 2: Run test and verify RED**

Run:

```bash
go test ./service/identitylookup -run TestStorageIdentityLookupImplementsClusterIdentityLookup -count=1
```

Expected: compile failure because `StorageIdentityLookup.Get` and `RemovePid` have the old signatures.

- [ ] **Step 3: Implement minimal interface changes**

Change `StorageIdentityLookup` methods to:

```go
func (s *StorageIdentityLookup) Get(placementContext *cluster.PlacementContext, clusterIdentity *cluster.ClusterIdentity) *actor.PID {
	return s.storageManager.Get(placementContext, clusterIdentity)
}

func (s *StorageIdentityLookup) RemovePid(_ *cluster.PlacementContext, clusterIdentity *cluster.ClusterIdentity, pid *actor.PID) {
	s.storageManager.RemovePid(clusterIdentity, pid)
}
```

Remove the `manualKinds` field and `Activate` method from `StorageIdentityLookup`.

- [ ] **Step 4: Remove manual manager flow**

In `manager.go`:

- remove `StorageManualProxyActorName`
- remove `manualProxyActor`
- remove `manualKinds`
- change `newStorageManager(c, storage, manualKinds)` to `newStorageManager(c, storage)`
- stop spawning and stopping the manual proxy actor
- remove `Manager.Activate`
- remove `Manager.IsManualKind`
- change `Manager.Get` to always call `doActivate` after no valid PID exists
- change `doActivate` to use `StorageActivatorActorName` directly

In `types/types.go`, remove `ManualActivateRequest`.

Delete `service/identitylookup/manual_proxy_actor.go`.

- [ ] **Step 5: Run interface test and verify GREEN**

Run:

```bash
go test ./service/identitylookup -run TestStorageIdentityLookupImplementsClusterIdentityLookup -count=1
```

Expected: test passes or reaches unrelated dependency/package failures that must be recorded before continuing.

## Task 2: Static Router Member Selection

**Files:**
- Modify: `service/identitylookup/manager.go`
- Test: `service/identitylookup/manager_test.go`

- [ ] **Step 1: Write failing static-router success test**

Add a fake static router and test that a configured router selects the routed member:

```go
func TestSelectMemberUsesStaticRouterWhenConfigured(t *testing.T) {
	placementContext := &cluster.PlacementContext{NodeType: "game", RouteKey: 42}
	identity := cluster.NewClusterIdentity("player-1", "player")
	routedMember := &cluster.Member{Name: "node-a", Host: "127.0.0.1", Port: 12001, Kinds: []string{"player"}, Id: "node-a-epoch-2"}

	router := &recordingStaticRouter{member: routedMember, ok: true}
	manager := &Manager{
		cluster: &cluster.Cluster{Config: &cluster.Config{StaticRouter: router}},
		members: cluster.Members{
			routedMember,
		},
		memberStrategy: &recordingMemberStrategy{},
	}

	got := manager.selectMember(placementContext, identity, "")

	if got == nil || got.Name != "node-a" || got.Port != 12001 {
		t.Fatalf("selectMember() = %#v, want node-a member", got)
	}
	if router.placementContext != placementContext {
		t.Fatalf("static router did not receive placement context")
	}
}
```

- [ ] **Step 2: Run test and verify RED**

Run:

```bash
go test ./service/identitylookup -run TestSelectMemberUsesStaticRouterWhenConfigured -count=1
```

Expected: compile failure or test failure because `selectMember` does not accept placement context and does not use `StaticRouter`.

- [ ] **Step 3: Implement topology member snapshot**

Add to `Manager`:

```go
members cluster.Members
```

In `onClusterTopology`, populate both `newMembers []*types.Member` and `clusterMembers cluster.Members`, then assign:

```go
pm.members = clusterMembers
pm.memberStrategy.UpdateMember(newMembers)
```

- [ ] **Step 4: Implement static-router selection**

Change:

```go
func (pm *Manager) selectMember(placementContext *cluster.PlacementContext, clusterIdentity *cluster.ClusterIdentity, memberID string) *types.Member
```

If `pm.cluster.Config.StaticRouter != nil`, call:

```go
member, ok := pm.cluster.Config.StaticRouter.Route(placementContext, clusterIdentity, pm.members)
if !ok || member == nil {
	return nil
}
return memberToStoredMember(member)
```

Use a helper:

```go
func memberToStoredMember(member *cluster.Member) *types.Member {
	name, epoch := ExtractSystemID(member.Id)
	return &types.Member{
		Member: *member,
		Name:   name,
		Epoch:  epoch,
	}
}
```

- [ ] **Step 5: Run static-router success test and verify GREEN**

Run:

```bash
go test ./service/identitylookup -run TestSelectMemberUsesStaticRouterWhenConfigured -count=1
```

Expected: pass.

## Task 3: Static Router Failure Is Authoritative

**Files:**
- Modify: `service/identitylookup/manager.go`
- Test: `service/identitylookup/manager_test.go`

- [ ] **Step 1: Write failing BDD tests**

Add tests:

```go
func TestSelectMemberReturnsNilWhenStaticRouterMisses(t *testing.T) {
	identity := cluster.NewClusterIdentity("player-1", "player")
	strategy := &recordingMemberStrategy{activator: &types.Member{Name: "fallback"}}
	manager := &Manager{
		cluster:        &cluster.Cluster{Config: &cluster.Config{StaticRouter: &recordingStaticRouter{ok: false}}},
		memberStrategy: strategy,
	}

	got := manager.selectMember(&cluster.PlacementContext{NodeType: "game", RouteKey: 42}, identity, "")

	if got != nil {
		t.Fatalf("selectMember() = %#v, want nil", got)
	}
	if strategy.getActivatorCalls != 0 {
		t.Fatalf("GetActivator called %d times, want 0", strategy.getActivatorCalls)
	}
}

func TestSelectMemberReturnsNilWhenStaticRouterTargetIsMissing(t *testing.T) {
	identity := cluster.NewClusterIdentity("player-1", "player")
	router := &recordingStaticRouter{ok: false}
	strategy := &recordingMemberStrategy{activator: &types.Member{Name: "fallback"}}
	manager := &Manager{
		cluster:        &cluster.Cluster{Config: &cluster.Config{StaticRouter: router}},
		memberStrategy: strategy,
		members:        cluster.Members{{Name: "node-b", Id: "node-b-epoch-1", Host: "127.0.0.1", Port: 12002, Kinds: []string{"player"}}},
	}

	got := manager.selectMember(&cluster.PlacementContext{NodeType: "game", RouteKey: 42}, identity, "")

	if got != nil {
		t.Fatalf("selectMember() = %#v, want nil", got)
	}
	if strategy.getActivatorCalls != 0 {
		t.Fatalf("GetActivator called %d times, want 0", strategy.getActivatorCalls)
	}
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
go test ./service/identitylookup -run 'TestSelectMemberReturnsNilWhenStaticRouter' -count=1
```

Expected: failure if implementation falls back to default strategy.

- [ ] **Step 3: Ensure static router miss returns nil**

Keep `selectMember` returning nil whenever static router exists and `Route` returns `ok=false` or `member=nil`.

- [ ] **Step 4: Run tests and verify GREEN**

Run:

```bash
go test ./service/identitylookup -run 'TestSelectMemberReturnsNilWhenStaticRouter' -count=1
```

Expected: pass.

## Task 4: Default Strategy Still Works Without Static Router

**Files:**
- Modify: `service/identitylookup/manager.go`
- Test: `service/identitylookup/manager_test.go`

- [ ] **Step 1: Write failing default strategy test**

```go
func TestSelectMemberUsesDefaultStrategyWhenStaticRouterIsNotConfigured(t *testing.T) {
	identity := cluster.NewClusterIdentity("player-1", "player")
	expected := &types.Member{Name: "default-node"}
	strategy := &recordingMemberStrategy{activator: expected}
	manager := &Manager{
		cluster:        &cluster.Cluster{Config: &cluster.Config{}},
		memberStrategy: strategy,
	}

	got := manager.selectMember(&cluster.PlacementContext{NodeType: "game", RouteKey: 42}, identity, "")

	if got != expected {
		t.Fatalf("selectMember() = %#v, want default strategy member", got)
	}
	if strategy.getActivatorCalls != 1 {
		t.Fatalf("GetActivator called %d times, want 1", strategy.getActivatorCalls)
	}
}
```

- [ ] **Step 2: Run test and verify RED if behavior is broken**

Run:

```bash
go test ./service/identitylookup -run TestSelectMemberUsesDefaultStrategyWhenStaticRouterIsNotConfigured -count=1
```

Expected: pass if behavior survived Task 2, otherwise fail and drive the minimal fix.

- [ ] **Step 3: Run focused identitylookup tests**

Run:

```bash
go test ./service/identitylookup -run 'TestStorageIdentityLookupImplementsClusterIdentityLookup|TestSelectMember' -count=1
```

Expected: all focused tests pass.

## Task 5: Final Verification

**Files:**
- All modified files.

- [ ] **Step 1: Format**

Run:

```bash
gofmt -w service/identitylookup/identity_lookup.go service/identitylookup/manager.go service/identitylookup/types/types.go service/identitylookup/*_test.go
```

- [ ] **Step 2: Run package tests**

Run:

```bash
go test ./service/identitylookup/... -count=1
```

Expected: pass, or report any unrelated module-path/dependency failures.

- [ ] **Step 3: Run lint**

Run:

```bash
make lint
```

Expected: pass, or report environment/network/module-cache failures.

- [ ] **Step 4: Review diff**

Run:

```bash
git diff -- service/identitylookup docs/superpowers/specs/2026-05-31-identitylookup-static-router-design.md docs/superpowers/plans/2026-05-31-identitylookup-static-router.md
```

Confirm:

- manual flow is removed
- `StorageIdentityLookup` implements the current cluster interface
- static router does not fall back
- default strategy still works when static router is absent
