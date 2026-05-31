package member_strategy

import (
	"math/rand"
	"sync"

	"gitee.com/wxdqing/identitylookup/types"
	"github.com/asynkron/protoactor-go/cluster"
)

type defaultMemberStrategy struct {
	cluster     *cluster.Cluster
	memberMap   map[string]*types.Member
	memberKinds map[string][]*types.Member
	mutex       sync.RWMutex
}

func (d *defaultMemberStrategy) GetMemberByName(memberName string) *types.Member {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.memberMap[memberName]
}
func (d *defaultMemberStrategy) GetActivator(clusterIdentity *cluster.ClusterIdentity) *types.Member {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	members := d.memberKinds[clusterIdentity.Kind]
	if len(members) == 0 {
		return nil
	}
	return members[rand.Intn(len(members))]
}

func (d *defaultMemberStrategy) UpdateMember(members []*types.Member) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.memberMap = make(map[string]*types.Member)
	d.memberKinds = make(map[string][]*types.Member)
	for _, member := range members {
		d.memberMap[member.Name] = member
		for _, kind := range member.Kinds {
			d.memberKinds[kind] = append(d.memberKinds[kind], member)
		}
	}
}

func NewDefaultMemberStrategy(c *cluster.Cluster) *defaultMemberStrategy {
	return &defaultMemberStrategy{
		cluster:     c,
		memberKinds: make(map[string][]*types.Member),
		memberMap:   make(map[string]*types.Member),
		mutex:       sync.RWMutex{},
	}
}
