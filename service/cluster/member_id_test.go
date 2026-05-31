package cluster

import (
	"strings"
	"testing"
)

func TestBuildMemberID(t *testing.T) {
	t.Parallel()

	memberID := BuildMemberID("node-a", 42)

	if memberID != "node-a-epoch-42" {
		t.Fatalf("BuildMemberID() = %q, want %q", memberID, "node-a-epoch-42")
	}
}

func TestParseMemberID(t *testing.T) {
	t.Parallel()

	name, epoch, ok := ParseMemberID(BuildMemberID("node-a", 42))

	if !ok {
		t.Fatalf("ParseMemberID() ok = false, want true")
	}
	if name != "node-a" {
		t.Fatalf("ParseMemberID() name = %q, want %q", name, "node-a")
	}
	if epoch != 42 {
		t.Fatalf("ParseMemberID() epoch = %d, want %d", epoch, int64(42))
	}
}

func TestParseMemberIDReturnsNameWithoutEpoch(t *testing.T) {
	t.Parallel()

	name, epoch, ok := ParseMemberID("node-a")

	if ok {
		t.Fatalf("ParseMemberID() ok = true, want false")
	}
	if name != "node-a" {
		t.Fatalf("ParseMemberID() name = %q, want %q", name, "node-a")
	}
	if epoch != 0 {
		t.Fatalf("ParseMemberID() epoch = %d, want %d", epoch, int64(0))
	}
}

func TestParseMemberIDReturnsNameWhenEpochIsInvalid(t *testing.T) {
	t.Parallel()

	invalidMemberID := strings.Replace(BuildMemberID("node-a", 42), "42", "not-an-epoch", 1)
	name, epoch, ok := ParseMemberID(invalidMemberID)

	if ok {
		t.Fatalf("ParseMemberID() ok = true, want false")
	}
	if name != "node-a" {
		t.Fatalf("ParseMemberID() name = %q, want %q", name, "node-a")
	}
	if epoch != 0 {
		t.Fatalf("ParseMemberID() epoch = %d, want %d", epoch, int64(0))
	}
}
