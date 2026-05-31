package redis_storage

import (
	"github.com/asynkron/protoactor-go/cluster"
)

type DataRecord struct {
	Kind          string `json:"kind"`
	Identity      string `json:"identity"`
	Address       string `json:"address"`
	ActorID       string `json:"actor_id"`
	MemberID      string `json:"member_id"`
	LockTime      int64  `json:"lock_time"`
	Version       int32  `json:"version"`
	ActivateTime  int64  `json:"activate_time"`
	TerminateTime int64  `json:"terminate_time"`
}

func NewDataRecord(clusterIdentity *cluster.ClusterIdentity) *DataRecord {
	dataRecord := &DataRecord{
		Kind:     clusterIdentity.Kind,
		Identity: clusterIdentity.Identity,
	}
	return dataRecord
}

func (d *DataRecord) GetAddress() string {
	return d.Address
}

func (d *DataRecord) SetAddress(address string) {
	d.Address = address
}

func (d *DataRecord) GetActorID() string {
	return d.ActorID
}

func (d *DataRecord) SetActorID(actorID string) {
	d.ActorID = actorID
}

func (d *DataRecord) GetMemberID() string {
	return d.MemberID
}

func (d *DataRecord) SetMemberID(memberID string) {
	d.MemberID = memberID
}

func (d *DataRecord) GetTerminateTime() uint64 {
	return uint64(d.TerminateTime)
}

func (d *DataRecord) SetTerminateTime(terminateTime uint64) {
	d.TerminateTime = int64(terminateTime)
}

func (d *DataRecord) GetLockTime() uint64 {
	return uint64(d.LockTime)
}

func (d *DataRecord) SetLockTime(lockTime uint64) {
	d.LockTime = int64(lockTime)
}

func (d *DataRecord) SetActivateTime(activateTime uint64) {
	d.ActivateTime = int64(activateTime)
}

func (d *DataRecord) GetVersion() int32 {
	return d.Version
}
