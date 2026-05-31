package identitylookup

import "testing"

func SystemIDTest(t *testing.T) {
	systemID := "test-epoch-1"
	name, epoch := ExtractSystemID(systemID)
	if name != "test" || epoch != 1 {
		t.Errorf("extract system id:%s fail", systemID)
	}

	systemID = "test-epoch--1"
	name, _ = ExtractSystemID(systemID)
	if name != systemID {
		t.Errorf("extract system id:%s fail", systemID)
	}

}
