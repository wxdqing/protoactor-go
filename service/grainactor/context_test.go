package grainactor

import (
	"context"
	"testing"
)

func TestContextCarriesActorMetadataAndState(t *testing.T) {
	state := &struct{ Count int }{}
	ctx := newContext(context.Background(), nil, "player-1", "player_equip", "player", state)

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
