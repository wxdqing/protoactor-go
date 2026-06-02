package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrainOnewaySendWithEtcd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping etcd integration test in short mode")
	}

	endpoints := os.Getenv("PROTOACTOR_ETCD_ENDPOINTS")
	if endpoints == "" {
		t.Skip("set PROTOACTOR_ETCD_ENDPOINTS to run etcd integration tests")
	}

	got, err := runGrainOnewayDemo(DemoConfig{
		EtcdEndpoints: strings.Split(endpoints, ","),
		BaseKey:       demoBaseKey(t.Name()),
		ClusterName:   "grain-oneway-" + strings.ReplaceAll(t.Name(), "/", "-"),
		RouteKey:      defaultRouteKey,
	})
	require.NoError(t, err)
	assert.Equal(t, defaultRouteKey, got)
}
