// Copyright (C) 2017 - 2024 Asynkron AB All rights reserved

package cluster

// MemberStateDelta describes a member state update that should be applied to gossip state.
type MemberStateDelta struct {
	TargetMemberID string
	HasState       bool
	State          *GossipState
	CommitOffsets  func()
}
