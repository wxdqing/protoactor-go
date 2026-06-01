# Grainactor Method Index Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route generated grainactor requests by actor-scoped `method_index` instead of protobuf message type name.

**Architecture:** The generator groups services by `option (options.actor)` and assigns each RPC an actor-scoped method index. Generated clients send that actor-scoped index, generated actor dispatchers switch on `cluster.GrainRequest.MethodIndex`, and `service/grainactor.BaseActor` delegates to one generated dispatcher without knowing about service bindings.

**Tech Stack:** Go, protobuf `protogen`, `service/protobuf/protoc-gen-go-grain`, `service/grainactor`, `service/cluster`, Go unit tests, generated-code compile tests.

---

### Task 1: Runtime Dispatcher API

**Files:**
- Modify: `service/grainactor/base_actor_test.go`
- Modify: `service/grainactor/base_actor.go`
- Modify: `service/grainactor/binding.go`

- [ ] **Step 1: Write failing runtime tests**

Replace binding-based tests with a dispatcher that receives the raw `GrainRequest`. The success test sends `MethodIndex: 7`; the not-found test is handled by the fake dispatcher; the panic test verifies `BaseActor` still recovers panics.

- [ ] **Step 2: Run runtime tests and verify failure**

Run: `go test ./service/grainactor -count=1`
Expected: FAIL because `NewBaseActor` has not yet accepted a single generated handler that receives method-index requests.

- [ ] **Step 3: Implement minimal runtime API**

Change `Binding` to `Handler`:

```go
type Handler interface {
	Receive(ctx Context, req *cluster.GrainRequest) (proto.Message, error)
}
```

Change `BaseActor` to store one `handler Handler`, remove the binding map, and call `handler.Receive(a.ctx, msg)`.

- [ ] **Step 4: Run runtime tests and verify pass**

Run: `go test ./service/grainactor -count=1`
Expected: PASS.

### Task 2: Generator Method Index Model

**Files:**
- Modify: `service/protobuf/protoc-gen-go-grain/generate_test.go`
- Modify: `service/protobuf/protoc-gen-go-grain/template.go`
- Modify: `service/protobuf/protoc-gen-go-grain/generate.go`

- [ ] **Step 1: Write failing generator tests**

Update template tests to require:
- `grainactor "github.com/asynkron/protoactor-go/service/grainactor"`
- no generated `MessageTypeName`
- `LoadMail` in the second service uses `MethodIndex: 2`
- generated `Player` dispatcher switches on `req.MethodIndex`
- same actor rejects mismatched `node_type`

- [ ] **Step 2: Run generator tests and verify failure**

Run: `go test ./service/protobuf/protoc-gen-go-grain -run 'Grainactor|Actor|BuildActor' -count=1`
Expected: FAIL due old binding template and message-type dispatch.

- [ ] **Step 3: Implement actor-scoped indexes**

Add actor method descriptors that record service, method, and actor-scoped index. Build them in `buildActorDescs` in stable service/method order. Copy the actor-scoped index back to the service method used by the client template.

- [ ] **Step 4: Run generator tests and verify pass**

Run: `go test ./service/protobuf/protoc-gen-go-grain -run 'Grainactor|Actor|BuildActor' -count=1`
Expected: PASS.

### Task 3: Generated Dispatcher Templates

**Files:**
- Modify: `service/protobuf/protoc-gen-go-grain/templates/grain.tmpl`
- Delete: `service/protobuf/protoc-gen-go-grain/templates/binding.tmpl`
- Modify: `service/protobuf/protoc-gen-go-grain/templates/actor.tmpl`
- Modify: `service/protobuf/protoc-gen-go-grain/template.go`

- [ ] **Step 1: Write failing template checks**

The tests from Task 2 should fail until templates generate a single actor dispatcher instead of per-service bindings.

- [ ] **Step 2: Implement templates**

Make `actor.tmpl` generate:

```go
type playerHandler struct {
	handler PlayerActor
}

func (h *playerHandler) Receive(ctx grainactor.Context, req *cluster.GrainRequest) (proto.Message, error) {
	switch req.MethodIndex {
	case 0:
		// unmarshal and call CrossSns.RoleSimple
	case 1:
		// unmarshal and call CrossSns.Keepalive
	case 2:
		// unmarshal and call CrossMail.LoadMail
	default:
		return nil, cluster.NewGrainErrorResponse(cluster.ErrorReason_NOT_FOUND, "unknown grain method index")
	}
}
```

Remove service binding generation.

- [ ] **Step 3: Run generator tests**

Run: `go test ./service/protobuf/protoc-gen-go-grain -count=1`
Expected: PASS.

### Task 4: Protocol Compatibility And Testdata

**Files:**
- Modify: `service/cluster/grain.proto`
- Modify: `service/cluster/grain.pb.go`
- Modify: `service/protobuf/protoc-gen-go-grain/Makefile`
- Regenerate: `service/protobuf/protoc-gen-go-grain/test/grainactor/*.pb.go`

- [ ] **Step 1: Preserve request message type name**

Keep `string message_type_name = 3;` on `GrainRequest` for protocol compatibility. Grainactor-generated clients and dispatchers route by actor-scoped `method_index` and do not populate or read `MessageTypeName`.

- [ ] **Step 2: Regenerate protobuf and grain testdata**

Run protoc for `service/cluster/grain.proto` and `make testdata` under `service/protobuf/protoc-gen-go-grain`.

- [ ] **Step 3: Run compile tests**

Run: `go test ./service/cluster ./service/grainactor ./service/protobuf/protoc-gen-go-grain/... -count=1`
Expected: PASS.

### Task 5: Documentation And Verification

**Files:**
- Modify: `docs/superpowers/specs/2026-05-31-grainactor-grain-design.md`
- Modify: `docs/superpowers/checklists/2026-05-31-grainactor-grain-checklist.md`
- Modify: `docs/superpowers/test-cases/2026-05-31-grainactor-grain-test-cases.md`

- [ ] **Step 1: Update docs**

Document that grainactor dispatch uses actor-scoped `method_index`; multiple services in one actor must share `kind` and `node_type`; `service/grainactor` is the runtime package.

- [ ] **Step 2: Run focused verification**

Run:

```bash
go test ./service/grainactor -count=1
go test ./service/protobuf/protoc-gen-go-grain/... -count=1
go test ./service/cluster -count=1
make lint
```

Expected: tests PASS; lint exits successfully. Fix relevant issues.

- [ ] **Step 3: Run all tests**

Run: `go test ./... -count=1`
Expected: PASS except environment-dependent providers if local Zookeeper or other services are unavailable; report exact failures.
