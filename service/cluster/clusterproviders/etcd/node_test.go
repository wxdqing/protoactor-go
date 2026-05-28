package etcd

import (
	"context"
	"log/slog"
	"reflect"
	"sync"
	"testing"

	"github.com/asynkron/protoactor-go/service/cluster"
)

type recordHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *recordHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *recordHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *recordHandler) WithGroup(_ string) slog.Handler      { return h }

func TestStrToIntLogsOnError(t *testing.T) {
	h := &recordHandler{}
	logger := slog.New(h)
	orig := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(orig)

	if v := strToInt("bad"); v != 0 {
		t.Fatalf("expected 0, got %d", v)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.records) == 0 {
		t.Fatalf("expected log entry for invalid input")
	}
}

func TestNodeUsesBaseNameFromMemberID(t *testing.T) {
	t.Parallel()

	node := NewNode("pod1@42", "192.168.0.1", 7788, []string{"kind1"})

	if node.ID != "pod1@42" {
		t.Fatalf("NewNode() ID = %q, want %q", node.ID, "pod1@42")
	}
	if node.Name != "pod1" {
		t.Fatalf("NewNode() Name = %q, want %q", node.Name, "pod1")
	}

	expected := &cluster.Member{
		Id:    "pod1@42",
		Name:  "pod1",
		Host:  "192.168.0.1",
		Port:  int32(7788),
		Kinds: []string{"kind1"},
	}
	if got := node.MemberStatus(); !reflect.DeepEqual(got, expected) {
		t.Fatalf("MemberStatus() = %+v, want %+v", got, expected)
	}
}
