package tcaplus_storage

import (
	"github.com/asynkron/protoactor-go/service/cluster"
	"google.golang.org/protobuf/proto"
)

type DataRecord struct {
	desc    *RouterTableDesc
	data    proto.Message
	version int32
}

func NewDataRecord(desc *RouterTableDesc, clusterIdentity *cluster.ClusterIdentity) *DataRecord {
	dataRecord := &DataRecord{
		desc: desc,
	}
	data := proto.Clone(desc.MessageType)
	setFieldValueStringUnsafe(data, desc.IdentityFields[0], clusterIdentity.Kind)
	setFieldValueStringUnsafe(data, desc.IdentityFields[1], clusterIdentity.Identity)
	dataRecord.data = data
	return dataRecord
}

func (d *DataRecord) GetAddress() string {
	return getFieldValueStringUnsafe(d.data, d.desc.PidFields[0])
}

func (d *DataRecord) SetAddress(address string) {
	setFieldValueStringUnsafe(d.data, d.desc.PidFields[0], address)
}

func (d *DataRecord) GetActorID() string {
	return getFieldValueStringUnsafe(d.data, d.desc.PidFields[1])
}

func (d *DataRecord) SetActorID(actorID string) {
	setFieldValueStringUnsafe(d.data, d.desc.PidFields[1], actorID)
}

func (d *DataRecord) GetMemberID() string {
	return getFieldValueStringUnsafe(d.data, d.desc.MemberIdField)
}

func (d *DataRecord) SetMemberID(memberID string) {
	setFieldValueStringUnsafe(d.data, d.desc.MemberIdField, memberID)
}

func (d *DataRecord) GetTerminateTime() uint64 {
	return getFieldValueIntUnsafe[uint64](d.data, d.desc.TerminateTimeField)
}

func (d *DataRecord) SetTerminateTime(terminateTime uint64) {
	setFieldValueIntUnsafe(d.data, d.desc.TerminateTimeField, terminateTime)
}

func (d *DataRecord) GetLockTime() uint64 {
	return getFieldValueIntUnsafe[uint64](d.data, d.desc.LockTimeField)
}

func (d *DataRecord) SetLockTime(lockTime uint64) {
	setFieldValueIntUnsafe(d.data, d.desc.LockTimeField, lockTime)
}

func (d *DataRecord) SetActivateTime(activateTime uint64) {
	setFieldValueIntUnsafe(d.data, d.desc.ActivateTimeField, activateTime)
}

func (d *DataRecord) GetVersion() int32 {
	return d.version
}
