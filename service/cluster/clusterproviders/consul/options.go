package consul

import "time"

// Option configures a Consul cluster provider.
type Option func(p *Provider)

// WithTTL sets the Consul service TTL for the provider.
func WithTTL(ttl time.Duration) Option {
	return func(p *Provider) {
		p.ttl = ttl
	}
}

// WithRefreshTTL sets how often the provider refreshes its TTL in Consul.
func WithRefreshTTL(refreshTTL time.Duration) Option {
	return func(p *Provider) {
		p.refreshTTL = refreshTTL
	}
}

// WithStartTimeout limits how long StartMember waits for Consul readiness.
func WithStartTimeout(timeout time.Duration) Option {
	return func(p *Provider) {
		p.startTimeout = timeout
	}
}

// WithServiceMetadata adds application metadata to the Consul registration.
func WithServiceMetadata(metadata map[string]string) Option {
	return func(p *Provider) {
		p.serviceMetadata = cloneMetadata(metadata)
	}
}
