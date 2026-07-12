package consul

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
)

type consulAPIFixture struct {
	mu              sync.Mutex
	registration    api.AgentServiceRegistration
	registerCode    int
	ttlCode         int
	passingQuery    string
	deregisterCalls int
}

func (f *consulAPIFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPut && r.URL.Path == "/v1/agent/service/register":
		if f.registerCode != 0 && f.registerCode != http.StatusOK {
			http.Error(w, "register", f.registerCode)
			return
		}
		var registration api.AgentServiceRegistration
		if err := json.NewDecoder(r.Body).Decode(&registration); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.registration = registration
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPut && len(r.URL.Path) >= len("/v1/agent/check/update/service:") && r.URL.Path[:len("/v1/agent/check/update/service:")] == "/v1/agent/check/update/service:":
		if f.ttlCode != 0 && f.ttlCode != http.StatusOK {
			http.Error(w, "ttl", f.ttlCode)
			return
		}
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodGet && len(r.URL.Path) >= len("/v1/health/service/") && r.URL.Path[:len("/v1/health/service/")] == "/v1/health/service/":
		f.mu.Lock()
		f.passingQuery = r.URL.Query().Get("passing")
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Consul-Index", "1")
		_, _ = w.Write([]byte("[]"))
	case r.Method == http.MethodPut && len(r.URL.Path) >= len("/v1/agent/service/deregister/") && r.URL.Path[:len("/v1/agent/service/deregister/")] == "/v1/agent/service/deregister/":
		f.mu.Lock()
		f.deregisterCalls++
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, r.Method+" "+r.URL.EscapedPath(), http.StatusNotFound)
	}
}

func TestDeregisterMemberAndShutdownDeregisterOnce(t *testing.T) {
	fixture := &consulAPIFixture{}
	server := httptest.NewServer(fixture)
	t.Cleanup(server.Close)
	p, err := NewWithConfig(testConsulConfig(t, server.URL), WithRefreshTTL(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	cluster := newClusterForTest("deregister-once", "127.0.0.1:8010", p)
	if err := p.StartMember(cluster); err != nil {
		t.Fatal(err)
	}
	if err := p.DeregisterMember(); err != nil {
		t.Fatal(err)
	}
	if err := p.Shutdown(true); err != nil {
		t.Fatal(err)
	}
	fixture.mu.Lock()
	calls := fixture.deregisterCalls
	fixture.mu.Unlock()
	if calls != 1 {
		t.Fatalf("deregister calls=%d want 1", calls)
	}
}

func TestStartMemberWaitsForRegistrationAndFirstPassingTTL(t *testing.T) {
	fixture := &consulAPIFixture{}
	server := httptest.NewServer(fixture)
	t.Cleanup(server.Close)
	p, err := NewWithConfig(testConsulConfig(t, server.URL),
		WithTTL(time.Second), WithRefreshTTL(time.Hour), WithStartTimeout(time.Second),
		WithServiceMetadata(map[string]string{
			"node_identity": "game/server-1/game-1", "node_session_id": "session-a",
			"node_type": "game", "node_group": "server-1", "node_name": "game-1",
			"grpc_port": "9001", "actor_address": "127.0.0.1:8000", "kinds": "player",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	cluster := newClusterForTest("contract", "127.0.0.1:8000", p)
	if err := p.StartMember(cluster); err != nil {
		t.Fatalf("StartMember: %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(true) })

	fixture.mu.Lock()
	registration := fixture.registration
	fixture.mu.Unlock()
	for key, want := range map[string]string{
		"node_identity": "game/server-1/game-1", "node_session_id": "session-a",
		"node_type": "game", "node_group": "server-1", "node_name": "game-1",
		"grpc_port": "9001", "actor_address": "127.0.0.1:8000", "kinds": "player",
	} {
		if registration.Meta[key] != want {
			t.Fatalf("metadata[%q]=%q, want %q", key, registration.Meta[key], want)
		}
	}
	grant, ok := p.LeaseGrant()
	if !ok || grant.ValidUntil.IsZero() || !grant.ValidUntil.After(time.Now()) {
		t.Fatalf("grant=%+v ok=%v", grant, ok)
	}
	health := p.Health()
	if !health.Healthy || health.LastSuccess.IsZero() || health.Err != nil {
		t.Fatalf("health=%+v", health)
	}
}

func TestStartMemberReturnsRegistrationAndTTLFailures(t *testing.T) {
	for _, test := range []struct {
		name         string
		registerCode int
		ttlCode      int
	}{
		{name: "register", registerCode: http.StatusInternalServerError},
		{name: "ttl", ttlCode: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := consulAPIFixture{registerCode: test.registerCode, ttlCode: test.ttlCode}
			server := httptest.NewServer(&fixture)
			t.Cleanup(server.Close)
			p, err := NewWithConfig(testConsulConfig(t, server.URL), WithStartTimeout(500*time.Millisecond))
			if err != nil {
				t.Fatal(err)
			}
			cluster := newClusterForTest("contract-fail-"+test.name, "127.0.0.1:8001", p)
			if err := p.StartMember(cluster); err == nil {
				_ = p.Shutdown(true)
				t.Fatal("StartMember succeeded")
			}
		})
	}
}

func TestPassingMembersIncludesEmptyTopology(t *testing.T) {
	if members := passingMembers("cluster", nil); members == nil || len(members) != 0 {
		t.Fatalf("members=%#v, want non-nil empty topology", members)
	}
}

func TestStartClientUsesPassingOnlyAndAcceptsEmptyTopology(t *testing.T) {
	fixture := &consulAPIFixture{}
	server := httptest.NewServer(fixture)
	t.Cleanup(server.Close)
	p, err := NewWithConfig(testConsulConfig(t, server.URL), WithRefreshTTL(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	cluster := newClusterForTest("client-contract", "127.0.0.1:8002", p)
	if err := p.StartClient(cluster); err != nil {
		t.Fatalf("StartClient: %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(true) })
	fixture.mu.Lock()
	passing := fixture.passingQuery
	fixture.mu.Unlock()
	if passing != "true" && passing != "1" {
		t.Fatalf("passing query = %q", passing)
	}
	if health := p.Health(); !health.Healthy || health.Err != nil {
		t.Fatalf("health = %+v", health)
	}
}

func testConsulConfig(t *testing.T, rawURL string) *api.Config {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	config := api.DefaultConfig()
	config.Address = parsed.Host
	config.Scheme = parsed.Scheme
	return config
}
