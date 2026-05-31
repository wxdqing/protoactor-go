package identitylookup

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/service/cluster"
)

func NewSystemIDOption(inst string, epoch uint64) actor.ConfigOption {
	if inst != "" && epoch != 0 {
		systemID := cluster.BuildMemberID(inst, epoch)
		return func(conf *actor.Config) {
			conf.ActorSystemID = systemID
		}
	}

	return nil
}

func ExtractSystemID(systemID string) (string, uint64) {
	inst, epoch, ok := cluster.ParseMemberID(systemID)
	if !ok {
		return systemID, 0
	}
	return inst, epoch
}

func IsValidMemberID(memberStrategy MemberStrategy, memberID string) bool {
	memberName, epoch := ExtractSystemID(memberID)
	member := memberStrategy.GetMemberByName(memberName)
	if member == nil {
		return false
	}

	if member.Epoch != epoch {
		return false
	}

	return true
}
