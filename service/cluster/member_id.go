// Package cluster provides service cluster membership primitives.
package cluster

import (
	"strconv"
	"strings"
)

const memberIDSeparator = "@"

// BuildMemberID builds a member instance ID from a stable member name and epoch.
func BuildMemberID(name string, epoch int64) string {
	return name + memberIDSeparator + strconv.FormatInt(epoch, 10)
}

// ParseMemberID extracts the stable member name and epoch from a member instance ID.
func ParseMemberID(id string) (name string, epoch int64, ok bool) {
	name, epochString, found := strings.Cut(id, memberIDSeparator)
	if !found {
		return id, 0, false
	}

	epoch, err := strconv.ParseInt(epochString, 10, 64)
	if err != nil {
		return name, 0, false
	}

	return name, epoch, true
}
