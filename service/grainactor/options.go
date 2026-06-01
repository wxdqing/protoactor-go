package grainactor

import "time"

type config struct {
	state          any
	stateFactory   func(identity string) any
	receiveTimeout time.Duration
}

// Option configures a BaseActor.
type Option func(*config)

// WithState sets a shared state object for the base actor instance.
func WithState(state any) Option {
	return func(config *config) {
		config.state = state
	}
}

// WithStateFactory creates state for each actor identity during initialization.
func WithStateFactory(fn func(identity string) any) Option {
	return func(config *config) {
		config.stateFactory = fn
	}
}

// WithReceiveTimeout sets the actor receive timeout.
func WithReceiveTimeout(timeout time.Duration) Option {
	return func(config *config) {
		config.receiveTimeout = timeout
	}
}
