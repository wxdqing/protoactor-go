package identitylookup

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
)

type StorageIdentityLookup struct {
	storageManager *Manager
	storage        IdentityStorage
	manualKinds    []string
}

func NewStorageIdentityLookup(storage IdentityStorage) *StorageIdentityLookup {
	return &StorageIdentityLookup{
		storage: storage,
	}
}

func (s *StorageIdentityLookup) Get(clusterIdentity *cluster.ClusterIdentity) *actor.PID {
	return s.storageManager.Get(clusterIdentity)
}

func (s *StorageIdentityLookup) RemovePid(clusterIdentity *cluster.ClusterIdentity, pid *actor.PID) {
	s.storageManager.RemovePid(clusterIdentity, pid)
}

func (s *StorageIdentityLookup) Activate(clusterIdentity *cluster.ClusterIdentity) *actor.PID {
	return s.storageManager.Activate(clusterIdentity)
}

func (s *StorageIdentityLookup) Setup(cluster *cluster.Cluster, kinds []string, isClient bool) {
	s.storageManager = newStorageManager(cluster, s.storage, s.manualKinds)
	s.storageManager.Start()
}

func (s *StorageIdentityLookup) Shutdown() {
	s.storageManager.Stop()
}

func (s *StorageIdentityLookup) GetIdentityStorage() IdentityStorage {
	return s.storage
}
