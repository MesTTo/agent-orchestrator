package store_test

import (
	"context"
	"testing"
)

func TestCreateSessionForExternalSourceIsDurablyIdempotent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedProject(t, store, "goalchainer")

	first, created, err := store.CreateSessionForExternalSource(
		ctx,
		sampleRecord("goalchainer"),
		"gc:incident:collect:1",
	)
	if err != nil || !created {
		t.Fatalf("first create = %#v, created=%v, err=%v", first, created, err)
	}
	second, created, err := store.CreateSessionForExternalSource(
		ctx,
		sampleRecord("goalchainer"),
		"gc:incident:collect:1",
	)
	if err != nil || created {
		t.Fatalf("retry = %#v, created=%v, err=%v", second, created, err)
	}
	if second.ID != first.ID {
		t.Fatalf("retry session = %s, want %s", second.ID, first.ID)
	}

	third, created, err := store.CreateSessionForExternalSource(
		ctx,
		sampleRecord("goalchainer"),
		"gc:incident:publish:1",
	)
	if err != nil || !created {
		t.Fatalf("second task = %#v, created=%v, err=%v", third, created, err)
	}
	if third.ID == first.ID {
		t.Fatalf("distinct task reused session %s", third.ID)
	}
}
