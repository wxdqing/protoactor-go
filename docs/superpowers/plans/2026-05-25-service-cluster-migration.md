# Service Cluster Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the existing `cluster` functionality into `service/cluster`, add explicit `PlacementContext` propagation for identity lookup, reserve static routing hooks, and add service-scoped grain generation for the new package.

**Architecture:** Create `service/cluster` as an independent package copied from the current `cluster` runtime, then change the service package API around placement-aware lookup without changing the old `cluster` package behavior. Keep static routing behind a small `StaticRouter` interface and migrate the grain generator into `service/protobuf/protoc-gen-go-grain` so the root generator remains unchanged.

**Tech Stack:** Go 1.26, Proto.Actor actor/remote packages, protobuf generation, service-scoped `protoc-gen-go-grain`, Go tests with `testing` and `testify`.

---

## Migration Strategy

The migration is split into phases that can be verified independently:

1. Copy the current `cluster` package into `service/cluster` and rewrite import paths.
2. Add `PlacementContext`, `StaticRouter`, config hooks, and placement-aware `IdentityLookup`.
3. Thread `PlacementContext` through `Cluster`, `Context`, `DefaultContext`, pub-sub direct lookups, and `disthash`.
4. Adapt protobuf generation and add the service grain generator for `service/cluster`.
5. Add behavior tests proving `PlacementContext` reaches identity lookup and generated grain clients call placement-aware APIs.
6. Run focused tests, lint, and then broader test suites.

## Checklist

- [x] `service/cluster` exists as an independent package.
- [x] All internal service cluster imports point to `github.com/asynkron/protoactor-go/service/cluster`.
- [x] `PlacementContext` exists and is documented.
- [x] `IdentityLookup.Get` and `RemovePid` accept `*PlacementContext`.
- [x] `Cluster.Get`, `Request`, and `RequestFuture` accept `*PlacementContext`.
- [x] Convenience wrappers either do not exist or call placement-aware methods with `nil`.
- [x] Existing timeout/retry behavior remains separate from placement context.
- [x] `StaticRouter` and `WithStaticRouter` exist without importing `staticrouter`.
- [x] `disthash` compiles and receives `PlacementContext`.
- [x] `service/cluster` protobuf files use `service/cluster` go package.
- [x] Service grain generator exists under `service/protobuf/protoc-gen-go-grain`.
- [x] Service grain generator supports the service cluster import path.
- [x] Generated service cluster grain client methods accept `*cluster.PlacementContext`.
- [x] Tests verify placement context propagation to custom identity lookup.
- [x] Tests verify static router config hook is stored.
- [x] Tests verify grain generator can emit service cluster import and placement-aware calls.
- [x] `make lint` has been run and actionable issues in touched files are fixed.
- [x] Relevant focused tests pass.
- [ ] Final broad tests have been run or documented if blocked by local services.

## TDD And BDD Plan

TDD behavior tests:

- `service/cluster` custom identity lookup records the received `PlacementContext` from `Cluster.Get`.
- `service/cluster` custom identity lookup records the received `PlacementContext` from `Cluster.Request`.
- `WithStaticRouter` stores the router on `Config`.
- `service/protobuf/protoc-gen-go-grain` emits service cluster import when `cluster_import` is supplied.
- `service/protobuf/protoc-gen-go-grain` emits placement-aware service grain client method signatures and calls.

BDD scenarios:

- Given a caller resolves a PID with a placement context, when `Cluster.Get` runs, then `IdentityLookup.Get` receives the same placement context.
- Given a caller sends a grain request with a placement context, when the PID is not cached, then the request path resolves through identity lookup with the same placement context.
- Given no placement context is supplied, when convenience APIs are used, then lookup still works with `nil` placement context and current timeout behavior is unchanged.
- Given a static router is configured, when identity lookup chooses an owner, then the configured router can be consulted without `service/cluster` depending on the concrete staticrouter repository.
- Given grain code is generated for `service/cluster`, when a client method is called, then it passes placement context into `Cluster.Request` or `Cluster.RequestFuture`.

## Task 1: Create Independent Service Cluster Package

**Files:**
- Create: `service/cluster/**`
- Modify generated import paths under `service/cluster`
- Keep: `cluster/**` and root `protobuf/protoc-gen-go-grain/**` unchanged.

- [x] Copy the current `cluster` directory into `service/cluster`, excluding existing `service/cluster/docs`.
- [x] Rewrite imports from `github.com/asynkron/protoactor-go/cluster` to `github.com/asynkron/protoactor-go/service/cluster` inside `service/cluster`.
- [x] Rewrite `github.com/asynkron/protoactor-go/cluster/metrics` to `github.com/asynkron/protoactor-go/service/cluster/metrics`.
- [x] Rewrite `github.com/asynkron/protoactor-go/cluster/identitylookup/disthash` to `github.com/asynkron/protoactor-go/service/cluster/identitylookup/disthash`.
- [x] Rewrite `github.com/asynkron/protoactor-go/cluster/clusterproviders/...` imports to `github.com/asynkron/protoactor-go/service/cluster/clusterproviders/...`.
- [x] Run `go test ./service/cluster/...` and capture compile failures for the next tasks.

## Task 2: Add PlacementContext And StaticRouter Contract

**Files:**
- Create: `service/cluster/placement_context.go`
- Create: `service/cluster/static_router.go`
- Modify: `service/cluster/config.go`
- Modify: `service/cluster/config_opts.go`
- Test: `service/cluster/placement_context_test.go`

- [x] Write failing tests for `PlacementContext` availability and `WithStaticRouter`.
- [x] Add exported comments for `PlacementContext` and `StaticRouter`.
- [x] Add `StaticRouter StaticRouter` to `Config`.
- [x] Add `WithStaticRouter(router StaticRouter) ConfigOption`.
- [x] Run `go test ./service/cluster -run 'TestPlacementContext|TestWithStaticRouter' -count=1`.

## Task 3: Make IdentityLookup Placement-Aware

**Files:**
- Modify: `service/cluster/identity_lookup.go`
- Modify: `service/cluster/cluster_test.go`
- Modify: `service/cluster/identitylookup/disthash/identity_lookup.go`
- Modify: `service/cluster/identitylookup/disthash/manager.go`
- Test: `service/cluster/placement_context_test.go`

- [x] Write a failing test where a fake `IdentityLookup.Get` records the exact `*PlacementContext` passed to `Cluster.Get`.
- [x] Change `IdentityLookup` interface to accept `*PlacementContext`.
- [x] Update fake identity lookup in tests.
- [x] Update `disthash.IdentityLookup.Get` and `RemovePid`.
- [x] Update `Manager.Get`.
- [x] Run `go test ./service/cluster -run TestClusterGetPassesPlacementContext -count=1`.

## Task 4: Thread PlacementContext Through Public Cluster Calls

**Files:**
- Modify: `service/cluster/context.go`
- Modify: `service/cluster/cluster.go`
- Modify: `service/cluster/default_context.go`
- Modify: service cluster call sites under `service/cluster/*.go`
- Test: `service/cluster/placement_context_test.go`

- [x] Write a failing test where `Cluster.Request(placementContext, ...)` records the placement context in fake identity lookup when PID cache misses.
- [x] Change `Context.Request` and `Context.RequestFuture` signatures.
- [x] Change `Cluster.Get`, `Request`, and `RequestFuture` signatures.
- [x] Update `DefaultContext.Request`, `RequestFuture`, and `getPid`.
- [x] Update pub-sub and topic call sites to pass `nil` where no placement context is available.
- [x] Run `go test ./service/cluster -run TestClusterRequestPassesPlacementContext -count=1`.

## Task 5: Preserve Existing Service Cluster Behavior With Nil PlacementContext

**Files:**
- Modify: service cluster tests as needed.
- Test: existing copied service cluster tests.

- [x] Update copied tests to call placement-aware APIs with `nil` where placement context is irrelevant.
- [ ] Run `go test ./service/cluster -count=1`.
- [ ] Run `go test ./service/cluster/identitylookup/disthash -count=1`.

## Task 6: Adapt Service Cluster Protobuf Package Paths

**Files:**
- Modify: `service/cluster/*.proto`
- Modify: `service/cluster/cluster_test_tool/*.proto`
- Modify/Create: `service/cluster/build.sh`
- Regenerate: `service/cluster/*.pb.go`, `service/cluster/cluster_test_tool/*.pb.go`

- [x] Change service cluster proto `go_package` values to `/github.com/asynkron/protoactor-go/service/cluster`.
- [x] Change service cluster test tool proto `go_package` values to `/github.com/asynkron/protoactor-go/service/cluster/cluster_test_tool`.
- [x] Update `service/cluster/build.sh` include paths while preserving old `cluster/build.sh`.
- [ ] Run service cluster protobuf generation.
- [x] Run `go test ./service/cluster/... -run TestNonExistent -count=0` to verify compile.

## Task 7: Add Service Grain Generator

**Files:**
- Create: `service/protobuf/protoc-gen-go-grain/**`
- Modify: `service/protobuf/protoc-gen-go-grain/main.go`
- Modify: `service/protobuf/protoc-gen-go-grain/generate.go`
- Modify: `service/protobuf/protoc-gen-go-grain/templates/grain.tmpl`
- Test: `service/protobuf/protoc-gen-go-grain/generate_test.go`

- [x] Write failing generator test for `cluster_import=github.com/asynkron/protoactor-go/service/cluster`.
- [x] Add or preserve a generator option parser for `cluster_import` in the service copy.
- [x] Default to `github.com/asynkron/protoactor-go/service/cluster` in the service generator.
- [x] Ensure root `protobuf/protoc-gen-go-grain` output remains unchanged by not modifying it.
- [x] Run `go test ./service/protobuf/protoc-gen-go-grain -count=1`.

## Task 8: Generate Placement-Aware Service Grain Clients

**Files:**
- Modify: `service/protobuf/protoc-gen-go-grain/templates/grain.tmpl`
- Test: `service/protobuf/protoc-gen-go-grain/generate_test.go`

- [x] Write failing generator test proving service cluster output includes `placementContext *cluster.PlacementContext`.
- [x] Gate placement-aware client signatures on service cluster mode.
- [x] Emit `g.cluster.Request(placementContext, ...)` and `RequestFuture(placementContext, ...)` in service cluster mode.
- [x] Keep old cluster generated client signatures unchanged.
- [x] Run `go test ./service/protobuf/protoc-gen-go-grain -count=1`.

## Task 9: Compile All Service Cluster Packages

**Files:**
- Modify any remaining service cluster import or signature errors.

- [ ] Run `go test ./service/cluster/... -count=1`.
- [x] Fix compile errors with minimal changes.
- [x] Re-run until service cluster packages pass or document external-service blockers.

## Task 10: Lint And Final Verification

**Files:**
- Any touched files with lint issues.

- [ ] Run `make lint`.
- [ ] Fix naming/comment issues in touched service cluster and generator files.
- [ ] Run focused tests:
  - `go test ./service/cluster/... -count=1`
  - `go test ./service/protobuf/protoc-gen-go-grain -count=1`
- [ ] Run broader tests as feasible:
  - `go test ./actor/... -count=1`
  - `go test ./remote/... -count=1`
  - `go test ./cluster/... -count=1`
- [ ] If Consul-dependent tests fail because Consul is unavailable, document the blocker and run non-Consul focused packages.
