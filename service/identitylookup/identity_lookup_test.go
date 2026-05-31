package identitylookup

import (
	"testing"

	"github.com/asynkron/protoactor-go/service/cluster"
)

func TestStorageIdentityLookupImplementsClusterIdentityLookup(t *testing.T) {
	var _ cluster.IdentityLookup = (*StorageIdentityLookup)(nil)
}
