// Package cluster provides service cluster membership primitives.
package cluster

import (
	"strconv"
	"strings"
)

const memberIDSeparator = "-epoch-"

// BuildMemberID builds a member instance ID from a stable member name and epoch.
func BuildMemberID(name string, epoch uint64) string {
	return name + memberIDSeparator + strconv.FormatUint(epoch, 10)
}

// MemberName extracts the stable member name from a member instance ID.
func MemberName(id string) string {
	name, _, _ := ParseMemberID(id)
	return name
}

// ParseMemberID extracts the stable member name and epoch from a member instance ID.
func ParseMemberID(id string) (name string, epoch uint64, ok bool) {
	name, epochString, found := strings.Cut(id, memberIDSeparator)
	if !found {
		return id, 0, false
	}

	epoch, err := strconv.ParseUint(epochString, 10, 64)
	if err != nil {
		return name, 0, false
	}

	return name, epoch, true
}
