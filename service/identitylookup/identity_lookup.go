package identitylookup

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/service/cluster"
)

type StorageIdentityLookup struct {
	storageManager *Manager
	storage        IdentityStorage
}

func NewStorageIdentityLookup(storage IdentityStorage) *StorageIdentityLookup {
	return &StorageIdentityLookup{
		storage: storage,
	}
}

func (s *StorageIdentityLookup) Get(placementContext *cluster.PlacementContext, clusterIdentity *cluster.ClusterIdentity) *actor.PID {
	return s.storageManager.Get(placementContext, clusterIdentity)
}

func (s *StorageIdentityLookup) RemovePid(_ *cluster.PlacementContext, clusterIdentity *cluster.ClusterIdentity, pid *actor.PID) {
	s.storageManager.RemovePid(clusterIdentity, pid)
}

func (s *StorageIdentityLookup) Setup(cluster *cluster.Cluster, kinds []string, isClient bool) {
	s.storageManager = newStorageManager(cluster, s.storage)
	s.storageManager.Start()
}

func (s *StorageIdentityLookup) Shutdown() {
	s.storageManager.Stop()
}

func (s *StorageIdentityLookup) GetIdentityStorage() IdentityStorage {
	return s.storage
}
