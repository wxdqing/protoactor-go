package grainactor

import (
	"context"
	"testing"
)

func TestContextCarriesActorMetadataAndState(t *testing.T) {
	state := &struct{ Count int }{}
	ctx := newContext(context.Background(), nil, "player-1", "player", state, nil)

	got := FromContext(ctx)
	if got == nil {
		t.Fatal("FromContext() = nil")
	}
	if got.Identity() != "player-1" {
		t.Fatalf("Identity() = %q", got.Identity())
	}
	if got.Kind() != "player" {
		t.Fatalf("Kind() = %q", got.Kind())
	}
	if got.Module() != "" {
		t.Fatalf("Module() = %q", got.Module())
	}

	gotState, ok := State[*struct{ Count int }](ctx)
	if !ok || gotState != state {
		t.Fatalf("State() = %#v, %v", gotState, ok)
	}
}

func TestContextPeerSessionReflectsBoundSession(t *testing.T) {
	session := &fakePeerSession{}
	ctx := newContext(context.Background(), nil, "player-1", "player", nil, func() PeerSession {
		return session
	})

	got, ok := ctx.PeerSession()
	if !ok || got != session {
		t.Fatalf("PeerSession() = %#v, %v; want session, true", got, ok)
	}

	handlerCtx := ToContext(ctx)
	fromHandler, ok := PeerSessionFromContext(handlerCtx)
	if !ok || fromHandler != session {
		t.Fatalf("PeerSessionFromContext() = %#v, %v; want session, true", fromHandler, ok)
	}
}

func TestToContextCarriesActorMetadataAndValues(t *testing.T) {
	state := &struct{ Count int }{}
	actorCtx := newContext(context.Background(), nil, "player-1", "player", state, nil)
	key := struct{}{}

	ctx := ToContext(actorCtx, WithValue(key, uint64(42)))

	if FromContext(ctx) != actorCtx {
		t.Fatal("FromContext() did not return original actor context")
	}
	gotState, ok := State[*struct{ Count int }](ctx)
	if !ok || gotState != state {
		t.Fatalf("State() = %#v, %v", gotState, ok)
	}
	if got := ctx.Value(key); got != uint64(42) {
		t.Fatalf("Value() = %#v, want 42", got)
	}
}

func TestToContextCarriesModuleWithoutMutatingBaseContext(t *testing.T) {
	actorCtx := newContext(context.Background(), nil, "player-1", "player", nil, nil)
	ctx := ToContext(actorCtx, WithValue("key", "value"), WithModule("equip"))

	got := FromContext(ctx)
	if got == nil {
		t.Fatal("FromContext() = nil")
	}
	if got.Module() != "equip" {
		t.Fatalf("Module() = %q", got.Module())
	}
	if actorCtx.Module() != "" {
		t.Fatalf("base Module() = %q", actorCtx.Module())
	}
	if got := ctx.Value("key"); got != "value" {
		t.Fatalf("Value(key) = %#v", got)
	}
}
