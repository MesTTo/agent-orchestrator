package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/cdc"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

func TestGoalChainerProjectionPersistsIdempotentlyAndEmitsCDC(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedProject(t, store, "goalchainer")
	first := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
	record := domain.GoalChainerProjectionRecord{
		ProjectID:      "goalchainer",
		Protocol:       "goalchainer-ui-v1",
		SourceRevision: "a3c0c8a6e6a6c9308f32af7fd614159fd64a35f9ce90f9b443f350b3887275fa",
		Projection:     `{"protocol":"goalchainer-ui-v1"}`,
		UpdatedAt:      first,
	}

	saved, err := store.UpsertGoalChainerProjection(ctx, record)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if saved.ProjectID != record.ProjectID || saved.Projection != record.Projection {
		t.Fatalf("saved = %#v", saved)
	}
	events, err := store.EventsAfter(ctx, 0, 100)
	if err != nil {
		t.Fatalf("read CDC: %v", err)
	}
	assertGoalChainerEventCount(t, events, 1)

	record.UpdatedAt = first.Add(time.Hour)
	same, err := store.UpsertGoalChainerProjection(ctx, record)
	if err != nil {
		t.Fatalf("idempotent upsert: %v", err)
	}
	if !same.UpdatedAt.Equal(first) {
		t.Fatalf("idempotent upsert changed timestamp to %s", same.UpdatedAt)
	}
	events, _ = store.EventsAfter(ctx, 0, 100)
	assertGoalChainerEventCount(t, events, 1)

	record.SourceRevision = "b3c0c8a6e6a6c9308f32af7fd614159fd64a35f9ce90f9b443f350b3887275fb"
	record.UpdatedAt = first.Add(2 * time.Hour)
	if _, err := store.UpsertGoalChainerProjection(ctx, record); err != nil {
		t.Fatalf("changed upsert: %v", err)
	}
	events, _ = store.EventsAfter(ctx, 0, 100)
	assertGoalChainerEventCount(t, events, 2)

	deleted, err := store.DeleteGoalChainerProjection(ctx, "goalchainer")
	if err != nil || !deleted {
		t.Fatalf("delete = %v, %v", deleted, err)
	}
	events, _ = store.EventsAfter(ctx, 0, 100)
	assertGoalChainerEventCount(t, events, 3)
}

func TestGoalChainerV2ProjectionPersistsWithoutChangingTheCDCContract(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedProject(t, store, "goalchainer")
	record := domain.GoalChainerProjectionRecord{
		ProjectID:      "goalchainer",
		Protocol:       "goalchainer-ui-v2",
		SourceRevision: "a3c0c8a6e6a6c9308f32af7fd614159fd64a35f9ce90f9b443f350b3887275fa",
		Projection:     `{"protocol":"goalchainer-ui-v2","projectId":"goalchainer","agents":[{"agentKey":"worker","role":"worker"}]}`,
		UpdatedAt:      time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC),
	}

	saved, err := store.UpsertGoalChainerProjection(ctx, record)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Protocol != "goalchainer-ui-v2" ||
		saved.Projection != record.Projection {
		t.Fatalf("saved v2 record = %#v", saved)
	}
	loaded, err := store.GetGoalChainerProjection(ctx, "goalchainer")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Protocol != record.Protocol || loaded.Projection != record.Projection {
		t.Fatalf("loaded v2 record = %#v", loaded)
	}
	events, err := store.EventsAfter(ctx, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	assertGoalChainerEventCount(t, events, 1)
}

func assertGoalChainerEventCount(t *testing.T, events []cdc.Event, want int) {
	t.Helper()
	got := 0
	for _, event := range events {
		if event.Type == cdc.EventGoalChainerProjectionChanged {
			got++
			if event.ProjectID != "goalchainer" || event.SessionID != "" {
				t.Fatalf("GoalChainer CDC scope = %#v", event)
			}
		}
	}
	if got != want {
		t.Fatalf("GoalChainer CDC events = %d, want %d; all=%#v", got, want, events)
	}
}
