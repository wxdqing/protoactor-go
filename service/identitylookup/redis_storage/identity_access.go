package redis_storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/asynkron/protoactor-go/service/cluster"
	"github.com/asynkron/protoactor-go/service/identitylookup"
	"github.com/asynkron/protoactor-go/service/identitylookup/errorx"
	"github.com/redis/go-redis/v9"
)

type IdentityDataAccess struct {
	client      redis.UniversalClient
	fieldPrefix string
}

func NewIdentityDataAccess(client redis.UniversalClient, baseKey string) *IdentityDataAccess {
	fieldPrefix := strings.Join([]string{baseKey, "actor:router"}, ":")
	return &IdentityDataAccess{
		client:      client,
		fieldPrefix: fieldPrefix,
	}
}

func (a *IdentityDataAccess) NewDataRecord(clusterIdentity *cluster.ClusterIdentity) identitylookup.IdentityDataRecord {
	d := NewDataRecord(clusterIdentity)
	return d
}

func (a *IdentityDataAccess) IDKey(clusterIdentity *cluster.ClusterIdentity) string {
	return strings.Join([]string{a.fieldPrefix, clusterIdentity.AsKey()}, ":")
}

func (a *IdentityDataAccess) LookupKey(clusterIdentity *cluster.ClusterIdentity) (identitylookup.IdentityDataRecord, error) {
	key := a.IDKey(clusterIdentity)
	ctx := context.Background()

	jsonData, err := a.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get router from Redis: %w", err)
	}

	data := &DataRecord{}
	if err := json.Unmarshal([]byte(jsonData), data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal router data: %w", err)
	}

	return data, nil
}

func (a *IdentityDataAccess) Insert(clusterIdentity *cluster.ClusterIdentity, dataRecord identitylookup.IdentityDataRecord) error {
	dataRecordImp := dataRecord.(*DataRecord)

	key := a.IDKey(clusterIdentity)
	ctx := context.Background()

	luaScript := `local actor_key = KEYS[1]
	local new_data = ARGV[1]
	
	local exists = redis.call('EXISTS', actor_key)
	if exists == 1 then
		return 0
	end
	
	redis.call('SET', actor_key, new_data)
	return 1
	`

	jsonData, err := json.Marshal(dataRecordImp)
	if err != nil {
		return fmt.Errorf("TryLockRouter Fail. failed to marshal router: %w", err)
	}

	result, err := a.client.Eval(ctx, luaScript, []string{key}, jsonData).Result()
	if err != nil {
		return fmt.Errorf("TryLockRouter Fail. failed to bind member: %w", err)
	}

	switch v := result.(type) {
	case int64:
		if v == 1 {
			return nil
		} else {
			return errorx.ErrDBRecordExist
		}
	default:
		return fmt.Errorf("TryLockRouter Fail. failed to bind member: %w", err)
	}
}

func (a *IdentityDataAccess) Update(clusterIdentity *cluster.ClusterIdentity, dataRecord identitylookup.IdentityDataRecord) error {
	dataRecordImp := dataRecord.(*DataRecord)

	key := a.IDKey(clusterIdentity)
	ctx := context.Background()

	luaScript := `local actor_key = KEYS[1]
local new_data = ARGV[1]
local new_version = tonumber(ARGV[2])

local exists = redis.call('EXISTS', actor_key)
if exists == 0 then
    return 0
end

local old_data = redis.call('GET', actor_key)
local old_version = tonumber(cjson.decode(old_data).version)

if old_version + 1 ~= new_version then
    return -1
end

redis.call('SET', actor_key, new_data)
return 1`

	dataRecordImp.Version = dataRecordImp.Version + 1
	dataJson, err := json.Marshal(dataRecordImp)
	if err != nil {
		return fmt.Errorf("failed to marshal router: %w", err)
	}

	res, err := a.client.Eval(ctx, luaScript, []string{key}, string(dataJson), dataRecordImp.Version).Result()
	if err != nil {
		return fmt.Errorf("failed to eval lua script: %w", err)
	}

	switch res.(int64) {
	case 0:
		return fmt.Errorf("record not exist, update failed")
	case -1:
		return errorx.ErrDBInvalidVersion
	case 1:
		return nil
	}

	return fmt.Errorf("unknown error")
}
