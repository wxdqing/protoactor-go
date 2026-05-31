package identitylookup

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"gitee.com/wxdqing/identitylookup/errorx"
	"gitee.com/wxdqing/identitylookup/types"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
)

type IdentityDataAccess interface {
	NewDataRecord(clusterIdentity *cluster.ClusterIdentity) IdentityDataRecord
	LookupKey(clusterIdentity *cluster.ClusterIdentity) (IdentityDataRecord, error)
	Insert(clusterIdentity *cluster.ClusterIdentity, data IdentityDataRecord) error
	Update(clusterIdentity *cluster.ClusterIdentity, data IdentityDataRecord) error
}

type IdentityDataRecord interface {
	GetAddress() string
	SetAddress(address string)

	GetActorID() string
	SetActorID(actorID string)

	GetMemberID() string
	SetMemberID(memberID string)

	GetTerminateTime() uint64
	SetTerminateTime(terminateTime uint64)

	GetLockTime() uint64
	SetLockTime(lockTime uint64)

	SetActivateTime(activateTime uint64)

	GetVersion() int32
}

type IdentityStorageImp struct {
	dataAccess     IdentityDataAccess
	maxLockTime    uint64
	keepMemberTime uint64
}

func NewIdentityStorage(dataAccess IdentityDataAccess) IdentityStorage {
	identityStorage := &IdentityStorageImp{
		dataAccess:     dataAccess,
		maxLockTime:    15,
		keepMemberTime: 3,
	}
	return identityStorage
}

func (s *IdentityStorageImp) TryGetExistingActivation(clusterIdentity *cluster.ClusterIdentity) (*types.StoredActivation, error) {
	data, err := s.dataAccess.LookupKey(clusterIdentity)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return &types.StoredActivation{}, nil
	}

	terminateTime := data.GetTerminateTime()
	if terminateTime > 0 {
		return &types.StoredActivation{}, nil
	}

	address := data.GetAddress()
	actorID := data.GetActorID()
	memberID := data.GetMemberID()
	if address == "" || actorID == "" || memberID == "" {
		return &types.StoredActivation{}, nil
	}

	return &types.StoredActivation{
		Pid:      actor.NewPID(address, actorID),
		MemberID: memberID,
	}, nil
}

func (s *IdentityStorageImp) TryAcquireLock(clusterIdentity *cluster.ClusterIdentity) (*types.SpawnLock, string, error) {
	data, err := s.dataAccess.LookupKey(clusterIdentity)
	if err != nil {
		return nil, "", fmt.Errorf("TryAcquireLock Fail. failed to lookup key: %w", err)
	}

	if data == nil {
		data = s.dataAccess.NewDataRecord(clusterIdentity)
		data.SetLockTime(uint64(time.Now().Unix()))
		if err := s.dataAccess.Insert(clusterIdentity, data); err != nil {
			if errors.Is(err, errorx.ErrDBRecordExist) {
				return nil, "", fmt.Errorf("failed to insert router: %w", errorx.ErrDBRecordLocked)
			}
			return nil, "", fmt.Errorf("TryAcquireLock Fail. failed to insert router: %w", err)
		}
	} else {
		lockTime := data.GetLockTime()
		if time.Now().Unix() < int64(lockTime)+int64(s.maxLockTime) {
			return nil, "", errorx.ErrDBRecordLocked
		}

		data.SetLockTime(uint64(time.Now().Unix()))
		if err := s.dataAccess.Update(clusterIdentity, data); err != nil {
			if errors.Is(err, errorx.ErrDBInvalidVersion) {
				return nil, "", fmt.Errorf("failed to update router: %w", errorx.ErrDBRecordLocked)
			}
			return nil, "", fmt.Errorf("TryAcquireLock Fail. failed to update router: %w", err)
		}
	}

	memberID := data.GetMemberID()
	if data.GetTerminateTime()+s.keepMemberTime < uint64(time.Now().Unix()) {
		// Actor 终止时间超过保留窗口后，不再强制复用原 member。
		fmt.Printf("Actor Terminated Longer Than %d Seconds %d %d\n", s.keepMemberTime, data.GetTerminateTime(), time.Now().Unix())
		memberID = ""
	}

	return &types.SpawnLock{
		ClusterIdentity: clusterIdentity,
		LockID:          strconv.Itoa(int(data.GetVersion())),
	}, memberID, nil
}

func (s *IdentityStorageImp) WaitForActivation(clusterIdentity *cluster.ClusterIdentity) (*types.StoredActivation, error) {
	data, err := s.dataAccess.LookupKey(clusterIdentity)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return &types.StoredActivation{}, nil
	}

	var BackoffTimes = []time.Duration{
		50 * time.Millisecond,
		100 * time.Millisecond,
		500 * time.Millisecond,
		1000 * time.Millisecond,
	}

	lockTime := data.GetLockTime()
	if lockTime > 0 && time.Now().Unix() < int64(lockTime)+int64(s.maxLockTime) {
		for _, backOff := range BackoffTimes {
			time.Sleep(time.Duration(backOff))
			if data, err = s.dataAccess.LookupKey(clusterIdentity); err != nil {
				return nil, err
			}

			if data == nil {
				return &types.StoredActivation{}, nil
			}

			if data.GetLockTime() == 0 {
				break
			}
		}
	}

	if data.GetTerminateTime() > 0 {
		return &types.StoredActivation{}, nil
	}

	address := data.GetAddress()
	actorID := data.GetActorID()
	memberID := data.GetMemberID()
	if address == "" || actorID == "" || memberID == "" {
		return &types.StoredActivation{}, nil
	}

	return &types.StoredActivation{
		Pid:      actor.NewPID(address, actorID),
		MemberID: memberID,
	}, nil
}

// RemoveLock(spawnLock SpawnLock)

func (s *IdentityStorageImp) StoreActivation(memberID string, spawnLock *types.SpawnLock, pid *actor.PID) error {
	data, err := s.dataAccess.LookupKey(spawnLock.ClusterIdentity)
	if err != nil {
		return fmt.Errorf("StoreActivation Fail. failed to lookup key: %w", err)
	}

	requestVersion, _ := strconv.Atoi(spawnLock.LockID)
	dataVersion := data.GetVersion()
	if int32(dataVersion) != int32(requestVersion) {
		return fmt.Errorf("version change %d %d", dataVersion, requestVersion)
	}

	data.SetActorID(pid.Id)
	data.SetAddress(pid.Address)
	data.SetMemberID(memberID)
	data.SetLockTime(0)
	data.SetActivateTime(uint64(time.Now().Unix()))
	data.SetTerminateTime(0)
	return s.dataAccess.Update(spawnLock.ClusterIdentity, data)
}

func (s *IdentityStorageImp) RemoveActivation(clusterIdentity *cluster.ClusterIdentity, pid *actor.PID) error {
	data, err := s.dataAccess.LookupKey(clusterIdentity)
	if err != nil {
		return fmt.Errorf("RemoveActivation Fail. failed to lookup key: %w", err)
	}

	if data == nil {
		return errorx.ErrActorRouterNotFound
	}

	address := data.GetAddress()
	actorID := data.GetActorID()
	if address != pid.Address || actorID != pid.Id {
		return errorx.ErrActorRouterPidNotMatch
	}

	data.SetTerminateTime(uint64(time.Now().Unix()))
	if err := s.dataAccess.Update(clusterIdentity, data); err != nil {
		return fmt.Errorf("RemoveActivation Fail. failed to delete router: %w", err)
	}

	return nil
}
