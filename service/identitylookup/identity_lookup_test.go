package identitylookup

import (
	"errors"
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/service/cluster"
	"github.com/asynkron/protoactor-go/service/identitylookup/errorx"
	"github.com/asynkron/protoactor-go/service/identitylookup/types"
)

func TestStorageIdentityLookupImplementsClusterIdentityLookup(t *testing.T) {
	var _ cluster.IdentityLookup = (*StorageIdentityLookup)(nil)
}

type fakeRouteCleanerStorage struct {
	clusterIdentity *cluster.ClusterIdentity
	err             error
}

func (s *fakeRouteCleanerStorage) TryGetExistingActivation(clusterIdentity *cluster.ClusterIdentity) (*types.StoredActivation, error) {
	return nil, nil
}

func (s *fakeRouteCleanerStorage) TryAcquireLock(clusterIdentity *cluster.ClusterIdentity) (*types.SpawnLock, string, error) {
	return nil, "", nil
}

func (s *fakeRouteCleanerStorage) WaitForActivation(clusterIdentity *cluster.ClusterIdentity) (*types.StoredActivation, error) {
	return nil, nil
}

func (s *fakeRouteCleanerStorage) StoreActivation(memberID string, spawnLock *types.SpawnLock, pid *actor.PID) error {
	return nil
}

func (s *fakeRouteCleanerStorage) RemoveActivation(clusterIdentity *cluster.ClusterIdentity, pid *actor.PID) error {
	return nil
}

func (s *fakeRouteCleanerStorage) ClearRoute(clusterIdentity *cluster.ClusterIdentity) error {
	s.clusterIdentity = clusterIdentity
	return s.err
}

type fakeStorageWithoutRouteCleaner struct{}

func (s *fakeStorageWithoutRouteCleaner) TryGetExistingActivation(clusterIdentity *cluster.ClusterIdentity) (*types.StoredActivation, error) {
	return nil, nil
}

func (s *fakeStorageWithoutRouteCleaner) TryAcquireLock(clusterIdentity *cluster.ClusterIdentity) (*types.SpawnLock, string, error) {
	return nil, "", nil
}

func (s *fakeStorageWithoutRouteCleaner) WaitForActivation(clusterIdentity *cluster.ClusterIdentity) (*types.StoredActivation, error) {
	return nil, nil
}

func (s *fakeStorageWithoutRouteCleaner) StoreActivation(memberID string, spawnLock *types.SpawnLock, pid *actor.PID) error {
	return nil
}

func (s *fakeStorageWithoutRouteCleaner) RemoveActivation(clusterIdentity *cluster.ClusterIdentity, pid *actor.PID) error {
	return nil
}

func TestStorageIdentityLookupClearRouteUsesStorageCleaner(t *testing.T) {
	storage := &fakeRouteCleanerStorage{}
	lookup := NewStorageIdentityLookup(storage)
	clusterIdentity := cluster.NewClusterIdentity("user-1", "player")

	if err := lookup.ClearRoute(clusterIdentity); err != nil {
		t.Fatalf("ClearRoute() error = %v", err)
	}
	if storage.clusterIdentity != clusterIdentity {
		t.Fatalf("ClearRoute() clusterIdentity = %v, want %v", storage.clusterIdentity, clusterIdentity)
	}
}

func TestStorageIdentityLookupClearRouteReturnsUnsupportedWhenStorageCannotClean(t *testing.T) {
	lookup := NewStorageIdentityLookup(&fakeStorageWithoutRouteCleaner{})

	err := lookup.ClearRoute(cluster.NewClusterIdentity("user-1", "player"))

	if !errors.Is(err, errorx.ErrRouteCleanupUnsupported) {
		t.Fatalf("ClearRoute() error = %v, want %v", err, errorx.ErrRouteCleanupUnsupported)
	}
}
