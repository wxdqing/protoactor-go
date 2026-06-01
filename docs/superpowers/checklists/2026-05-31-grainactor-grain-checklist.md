# Grainactor Grain Checklist

## Documentation Delivery

- [x] Requirements document created: `docs/superpowers/specs/2026-05-31-grainactor-grain-design.md`
- [x] Implementation plan created: `docs/superpowers/plans/2026-05-31-grainactor-grain.md`
- [x] Checklist document created: `docs/superpowers/checklists/2026-05-31-grainactor-grain-checklist.md`
- [x] Test case document created: `docs/superpowers/test-cases/2026-05-31-grainactor-grain-test-cases.md`
- [x] Directory recommendation recorded as `service/grainactor`
- [x] RouteKey handling recorded as call-time data
- [x] Use-case-driven development flow recorded

## Future Implementation Checklist

### Options

- [x] Add `options.kind`
- [x] Add `options.node_type`
- [x] Add `options.actor`
- [x] Regenerate `options.pb.go`
- [x] Add proto fixture using all three options

### Generator

- [x] Parse service options into `serviceDesc`
- [x] Group services by actor name
- [x] Validate duplicate method names inside one actor group
- [x] Validate actor group kind consistency
- [x] Generate actor aggregate interfaces
- [x] Generate actor-level dispatcher
- [x] Generate one base actor constructor per actor group
- [x] Generate actor-scoped method indexes for grouped services
- [x] Generate client `ByRouteKey` methods
- [x] Generate client `WithPlacement` methods
- [x] Preserve current output for services without new options

### Runtime

- [x] Create `service/grainactor`
- [x] Add actor-aware context wrapper
- [x] Add typed state helper
- [x] Add handler interface
- [x] Add base actor dispatch by generated handler
- [x] Add lifecycle handling
- [x] Add panic recovery around handler dispatch
- [x] Add runtime options for state and receive timeout

### Routing

- [x] Ensure generated clients pass `NodeType`
- [x] Ensure generated clients pass call-time `RouteKey`
- [x] Verify `PlacementContext` reaches identitylookup
- [x] Verify staticrouter receives `NodeType` and `RouteKey`
- [x] Verify no generator dependency on staticrouter

### Tests

- [x] Generator fixture compiles
- [x] Actor aggregate output is asserted
- [x] Actor dispatcher output is asserted
- [x] Actor-scoped method index output is asserted
- [x] Placement helper output is asserted
- [x] Runtime context tests pass
- [x] Runtime dispatch tests pass
- [x] Identitylookup/staticrouter placement tests pass
- [x] Compatibility tests pass for proto services without new options

## Completion Definition For Future Implementation

The feature is complete when:

- generated code for one actor group with two services compiles
- generated code uses one base actor constructor for that actor group
- generated clients and dispatchers agree on actor-scoped `method_index`
- one shared state object is visible to both service methods
- generated client can call plain, `ByRouteKey`, and `WithPlacement` variants
- `NodeType` and `RouteKey` reach staticrouter through existing identitylookup flow
- existing generated grain tests still pass
- lint exits with code 0
