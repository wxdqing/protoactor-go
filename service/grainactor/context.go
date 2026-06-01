package grainactor

import (
	"context"

	"github.com/asynkron/protoactor-go/service/cluster"
)

type contextKey struct{}

// Context carries grain actor metadata for generated service handlers.
type Context interface {
	context.Context

	GrainContext() cluster.GrainContext
	Identity() string
	Kind() string
	Actor() string
	State() any
}

type actorContext struct {
	context.Context
	grainContext cluster.GrainContext
	identity     string
	kind         string
	actor        string
	state        any
}

func newContext(parent context.Context, grainContext cluster.GrainContext, identity, kind, actor string, state any) Context {
	ctx := &actorContext{
		Context:      parent,
		grainContext: grainContext,
		identity:     identity,
		kind:         kind,
		actor:        actor,
		state:        state,
	}
	ctx.Context = context.WithValue(parent, contextKey{}, ctx)
	return ctx
}

// FromContext returns the grain actor context stored in ctx, if present.
func FromContext(ctx context.Context) Context {
	if actorCtx, ok := ctx.(Context); ok {
		return actorCtx
	}
	if actorCtx, ok := ctx.Value(contextKey{}).(Context); ok {
		return actorCtx
	}
	return nil
}

// State returns the typed actor state stored in ctx.
func State[T any](ctx context.Context) (T, bool) {
	var zero T
	actorCtx := FromContext(ctx)
	if actorCtx == nil {
		return zero, false
	}
	state, ok := actorCtx.State().(T)
	return state, ok
}

// GrainContext returns the service cluster grain context.
func (c *actorContext) GrainContext() cluster.GrainContext {
	return c.grainContext
}

// Identity returns the cluster identity for this actor instance.
func (c *actorContext) Identity() string {
	return c.identity
}

// Kind returns the cluster kind for this actor instance.
func (c *actorContext) Kind() string {
	return c.kind
}

// Actor returns the logical actor group name.
func (c *actorContext) Actor() string {
	return c.actor
}

// State returns the shared actor state.
func (c *actorContext) State() any {
	return c.state
}
