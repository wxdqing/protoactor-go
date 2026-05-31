package tcaplus_storage

import (
	"errors"
	"fmt"

	"github.com/asynkron/protoactor-go/service/cluster"
	"github.com/asynkron/protoactor-go/service/identitylookup"
	"github.com/asynkron/protoactor-go/service/identitylookup/errorx"
	cli "github.com/tencentyun/tcaplusdb-go-sdk/pb"
	"google.golang.org/protobuf/proto"
)

type RouterTableDesc struct {
	TableName          string
	MessageType        proto.Message
	IdentityFields     []string
	PidFields          []string
	MemberIdField      string
	LockTimeField      string
	ActivateTimeField  string
	TerminateTimeField string
}

type IdentityDataAccess struct {
	desc  *RouterTableDesc
	table *TcaplusTable
}

func NewIdentityDataAccess(client *cli.PBClient, zoneID uint32, desc *RouterTableDesc) *IdentityDataAccess {
	t := &IdentityDataAccess{
		desc: desc,
	}

	checkFields := desc.IdentityFields
	checkFields = append(checkFields, desc.PidFields...)
	checkFields = append(checkFields, desc.MemberIdField, desc.LockTimeField,
		desc.ActivateTimeField, desc.TerminateTimeField)
	for _, field := range checkFields {
		if !isMessageField(desc.MessageType, field) {
			panic(fmt.Sprintf("field %s not found in message %s", field, desc.MessageType))
		}
	}

	t.table = NewTcaplusTable(client, zoneID, desc.TableName, desc.MessageType, desc.IdentityFields)
	return t
}

func (a *IdentityDataAccess) NewDataRecord(clusterIdentity *cluster.ClusterIdentity) identitylookup.IdentityDataRecord {
	d := NewDataRecord(a.desc, clusterIdentity)
	return d
}

func (a *IdentityDataAccess) LookupKey(clusterIdentity *cluster.ClusterIdentity) (identitylookup.IdentityDataRecord, error) {
	identityValues := []string{clusterIdentity.Kind, clusterIdentity.Identity}
	data, version, err := a.table.GetRecord(identityValues)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil
	}

	return &DataRecord{
		data:    data,
		desc:    a.desc,
		version: version,
	}, nil
}

func (a *IdentityDataAccess) Insert(clusterIdentity *cluster.ClusterIdentity, dataRecord identitylookup.IdentityDataRecord) error {
	dataRecordImp := dataRecord.(*DataRecord)
	identityValues := []string{clusterIdentity.Kind, clusterIdentity.Identity}
	newData, newVersion, err := a.table.InsertRecord(identityValues, dataRecordImp.data)
	if err != nil {
		if errors.Is(err, errorx.ErrDBRecordExist) {
			return errorx.ErrDBRecordLocked
		}
		return fmt.Errorf("insert record failed: %w", err)
	}

	dataRecordImp.data = newData
	dataRecordImp.version = newVersion
	return nil
}

func (a *IdentityDataAccess) Update(clusterIdentity *cluster.ClusterIdentity, dataRecord identitylookup.IdentityDataRecord) error {
	dataRecordImp := dataRecord.(*DataRecord)
	identityValues := []string{clusterIdentity.Kind, clusterIdentity.Identity}
	newData, newVersion, err := a.table.UpdateRecord(identityValues, dataRecordImp.data, dataRecordImp.version)
	if err != nil {
		if errors.Is(err, errorx.ErrDBInvalidVersion) {
			return errorx.ErrDBRecordLocked
		}
		return fmt.Errorf("update record failed: %w", err)
	}

	dataRecordImp.data = newData
	dataRecordImp.version = newVersion
	return nil
}
