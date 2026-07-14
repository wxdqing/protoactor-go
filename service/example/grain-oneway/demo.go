package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/asynkron/protoactor-go/service/cluster"
	"github.com/asynkron/protoactor-go/service/cluster/clusterproviders/etcd"
	"github.com/asynkron/protoactor-go/service/cluster/identitylookup/disthash"
	grainoneway "github.com/asynkron/protoactor-go/service/example/grain-oneway/proto"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	defaultIdentity = "player-1"
	defaultRouteKey = uint64(42)
)

// DemoConfig configures the grain one-way Send example against etcd.
type DemoConfig struct {
	EtcdEndpoints []string
	BaseKey       string
	ClusterName   string
	RouteKey      uint64
}

func runGrainOnewayDemo(cfg DemoConfig) (receivedRouteKey uint64, err error) {
	if len(cfg.EtcdEndpoints) == 0 {
		return 0, fmt.Errorf("etcd endpoints are required")
	}
	if cfg.BaseKey == "" {
		cfg.BaseKey = "/protoactor-example/grain-oneway"
	}
	if cfg.ClusterName == "" {
		cfg.ClusterName = "grain-oneway-demo"
	}
	if cfg.RouteKey == 0 {
		cfg.RouteKey = defaultRouteKey
	}

	received := make(chan uint64, 1)
	handler := &pingHandler{received: received}

	provider, err := etcd.NewWithConfig(cfg.BaseKey, clientv3.Config{
		Endpoints:   cfg.EtcdEndpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return 0, fmt.Errorf("create etcd provider: %w", err)
	}
	defer func() { _ = provider.Shutdown(true) }()

	playerKind := grainoneway.NewPlayerKind(handler, nil)

	remoteConfig := remote.Configure("127.0.0.1", 0)
	system := actor.NewActorSystem()
	config := cluster.Configure(cfg.ClusterName, provider, disthash.New(), remoteConfig, cluster.WithKinds(playerKind))
	c := cluster.New(system, config)
	defer c.Shutdown(true)

	c.StartMember()

	client := grainoneway.GetPlayerGrainClient(c, defaultIdentity)
	req := &grainoneway.PingRequest{ServerId: cfg.RouteKey}

	deadline := time.Now().Add(15 * time.Second)
	var sendErr error
	for time.Now().Before(deadline) {
		sendErr = client.PingSend(
			&cluster.PlacementContext{Labels: map[string]string{
				"server_id": strconv.FormatUint(cfg.RouteKey, 10),
			}},
			req,
		)
		if sendErr == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if sendErr != nil {
		return 0, fmt.Errorf("PingSend: %w", sendErr)
	}

	select {
	case got := <-received:
		return got, nil
	case <-time.After(10 * time.Second):
		return 0, fmt.Errorf("timed out waiting for one-way grain message")
	}
}

func demoBaseKey(suffix string) string {
	key := "/protoactor-example/grain-oneway"
	if suffix != "" {
		key += "/" + strings.ReplaceAll(suffix, "/", "-")
	}
	return key
}
