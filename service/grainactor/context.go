package grainactor

import (
	"context"
	"time"

	"github.com/asynkron/protoactor-go/service/cluster"
)

type contextKey struct{}

// ToContextOption customizes a handler context converted from a grain actor context.
type ToContextOption func(context.Context) context.Context

// Context carries grain actor metadata for generated service handlers.
type Context interface {
	context.Context

	GrainContext() cluster.GrainContext
	Identity() string
	Kind() string
	Actor() string
	State() any
	PeerSession() (PeerSession, bool)

	// After schedules a one-shot timer handled inside the actor mailbox.
	After(name string, delay time.Duration, fn func(Context))
	// Every schedules a repeating timer handled inside the actor mailbox.
	Every(name string, interval time.Duration, fn func(Context))
	// CancelTimer cancels a named timer.
	CancelTimer(name string)

	// On registers an in-actor event handler.
	On(event string, handler func(Context, any))
	// Off removes all handlers for an event name.
	Off(event string)
	// Emit dispatches an event to registered handlers inside the actor mailbox.
	Emit(event string, payload any)
}

type actorContext struct {
	context.Context
	grainContext cluster.GrainContext
	identity     string
	kind         string
	actor        string
	state        any
	peerSession  func() PeerSession
}

func newContext(parent context.Context, grainContext cluster.GrainContext, identity, kind, actor string, state any, peerSession func() PeerSession) Context {
	ctx := &actorContext{
		Context:      parent,
		grainContext: grainContext,
		identity:     identity,
		kind:         kind,
		actor:        actor,
		state:        state,
		peerSession:  peerSession,
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

// ToContext converts a grain actor context into the context passed to service handlers.
func ToContext(ctx Context, opts ...ToContextOption) context.Context {
	var handlerCtx context.Context = ctx
	for _, opt := range opts {
		handlerCtx = opt(handlerCtx)
	}
	return handlerCtx
}

// WithValue adds a value to a converted handler context.
func WithValue(key any, value any) ToContextOption {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, key, value)
	}
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

// PeerSessionFromContext returns the active peer session stored in ctx, if any.
func PeerSessionFromContext(ctx context.Context) (PeerSession, bool) {
	actorCtx := FromContext(ctx)
	if actorCtx == nil {
		return nil, false
	}
	return actorCtx.PeerSession()
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

// PeerSession returns the active peer session for this actor instance.
func (c *actorContext) PeerSession() (PeerSession, bool) {
	if c.peerSession == nil {
		return nil, false
	}
	session := c.peerSession()
	return session, session != nil
}

func (c *actorContext) After(name string, delay time.Duration, fn func(Context)) {
	if c.grainContext == nil || fn == nil {
		return
	}
	c.grainContext.Send(c.grainContext.Self(), &scheduleAfter{
		name:  name,
		delay: delay,
		fn:    fn,
	})
}

func (c *actorContext) Every(name string, interval time.Duration, fn func(Context)) {
	if c.grainContext == nil || fn == nil {
		return
	}
	c.grainContext.Send(c.grainContext.Self(), &scheduleEvery{
		name:     name,
		interval: interval,
		fn:       fn,
	})
}

func (c *actorContext) CancelTimer(name string) {
	if c.grainContext == nil || name == "" {
		return
	}
	c.grainContext.Send(c.grainContext.Self(), &cancelTimer{name: name})
}

func (c *actorContext) On(event string, handler func(Context, any)) {
	if c.grainContext == nil || event == "" || handler == nil {
		return
	}
	c.grainContext.Send(c.grainContext.Self(), &eventRegister{
		name:    event,
		handler: handler,
	})
}

func (c *actorContext) Off(event string) {
	if c.grainContext == nil || event == "" {
		return
	}
	c.grainContext.Send(c.grainContext.Self(), &eventUnregister{name: event})
}

func (c *actorContext) Emit(event string, payload any) {
	if c.grainContext == nil || event == "" {
		return
	}
	c.grainContext.Send(c.grainContext.Self(), &eventEmit{
		name:    event,
		payload: payload,
	})
}
