package grainactor

import (
	"context"
	"testing"
)

func TestContextCarriesActorMetadataAndState(t *testing.T) {
	state := &struct{ Count int }{}
	ctx := newContext(context.Background(), nil, "player-1", "player_equip", "player", state, nil)

	got := FromContext(ctx)
	if got == nil {
		t.Fatal("FromContext() = nil")
	}
	if got.Identity() != "player-1" {
		t.Fatalf("Identity() = %q", got.Identity())
	}
	if got.Kind() != "player_equip" {
		t.Fatalf("Kind() = %q", got.Kind())
	}
	if got.Actor() != "player" {
		t.Fatalf("Actor() = %q", got.Actor())
	}

	gotState, ok := State[*struct{ Count int }](ctx)
	if !ok || gotState != state {
		t.Fatalf("State() = %#v, %v", gotState, ok)
	}
}

func TestToContextCarriesActorMetadataAndValues(t *testing.T) {
	state := &struct{ Count int }{}
	actorCtx := newContext(context.Background(), nil, "player-1", "player_equip", "player", state, nil)
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
