package redis_storage

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/service/cluster"
	"github.com/redis/go-redis/v9"
)

func newRedisClientForTest(t *testing.T) redis.UniversalClient {
	t.Helper()

	addr := os.Getenv("IDENTITYLOOKUP_REDIS_ADDR")
	if addr == "" {
		t.Skip("set IDENTITYLOOKUP_REDIS_ADDR to run Redis integration tests")
	}

	password := os.Getenv("IDENTITYLOOKUP_REDIS_PASSWORD")
	addrs := strings.Split(addr, ",")
	if len(addrs) > 1 {
		return redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    addrs,
			Password: password,
		})
	}

	db := 0
	if rawDB := os.Getenv("IDENTITYLOOKUP_REDIS_DB"); rawDB != "" {
		parsedDB, err := strconv.Atoi(rawDB)
		if err != nil {
			t.Fatalf("parse IDENTITYLOOKUP_REDIS_DB: %v", err)
		}
		db = parsedDB
	}

	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

func TestClearRouteDeletesExistingRouteKey(t *testing.T) {
	client := newRedisClientForTest(t)
	defer client.Close()

	access := NewIdentityDataAccess(client, "protoactor-go-test")
	clusterIdentity := cluster.NewClusterIdentity("clear-route-"+time.Now().Format("150405.000000000"), "player")
	t.Cleanup(func() {
		_ = client.Del(t.Context(), access.IDKey(clusterIdentity)).Err()
	})

	record := access.NewDataRecord(clusterIdentity)
	if err := access.Insert(clusterIdentity, record); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	if err := access.ClearRoute(clusterIdentity); err != nil {
		t.Fatalf("ClearRoute() error = %v", err)
	}

	data, err := access.LookupKey(clusterIdentity)
	if err != nil {
		t.Fatalf("LookupKey() error = %v", err)
	}
	if data != nil {
		t.Fatalf("LookupKey() = %#v, want nil after ClearRoute", data)
	}
}
