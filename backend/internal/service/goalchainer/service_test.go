package goalchainer

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apierr"
)

type fakeStore struct {
	project bool
	record  domain.GoalChainerProjectionRecord
}

func (f *fakeStore) GoalChainerProjectExists(context.Context, domain.ProjectID) (bool, error) {
	return f.project, nil
}

func (f *fakeStore) UpsertGoalChainerProjection(
	_ context.Context,
	record domain.GoalChainerProjectionRecord,
) (domain.GoalChainerProjectionRecord, error) {
	if f.record.SourceRevision == record.SourceRevision {
		return f.record, nil
	}
	f.record = record
	return record, nil
}

func (f *fakeStore) GetGoalChainerProjection(
	context.Context,
	domain.ProjectID,
) (domain.GoalChainerProjectionRecord, error) {
	if f.record.ProjectID == "" {
		return domain.GoalChainerProjectionRecord{}, sql.ErrNoRows
	}
	return f.record, nil
}

func (f *fakeStore) DeleteGoalChainerProjection(
	context.Context,
	domain.ProjectID,
) (bool, error) {
	deleted := f.record.ProjectID != ""
	f.record = domain.GoalChainerProjectionRecord{}
	return deleted, nil
}

func validProjection() Projection {
	digest := "a3c0c8a6e6a6c9308f32af7fd614159fd64a35f9ce90f9b443f350b3887275fa"
	return Projection{
		Protocol:       Protocol,
		SourceRevision: digest,
		ThreadID:       "incident-42",
		Title:          "Incident response",
		Owner:          "responder",
		SemanticDigest: digest,
		PlanDigest:     digest,
		Summary: Summary{
			Total: 2, Ready: 1, Backlog: 1,
		},
		Tasks: []Task{
			{
				ID: "collect-evidence", Capability: "local",
				Acceptance: "evidence-collected", Stakes: "normal",
				Dependencies: []string{}, State: "ready",
			},
			{
				ID: "publish", Capability: "specialist",
				Acceptance: "update-published", Stakes: "high",
				Dependencies: []string{"collect-evidence"}, State: "backlog",
			},
		},
		Audit: Audit{Events: 1, Head: digest},
	}
}

func TestPutGetDeleteProjectionWithoutTaskMutation(t *testing.T) {
	store := &fakeStore{project: true}
	service := New(store)
	now := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
	service.clock = func() time.Time { return now }
	projection := validProjection()

	saved, err := service.Put(context.Background(), "goalchainer", projection)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if saved.ProjectID != "goalchainer" || !saved.UpdatedAt.Equal(now) {
		t.Fatalf("saved projection = %#v", saved)
	}
	if saved.Tasks[0].State != "ready" {
		t.Fatalf("AO mutated task state to %q", saved.Tasks[0].State)
	}

	got, err := service.Get(context.Background(), "goalchainer")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SourceRevision != projection.SourceRevision || got.Tasks[1].Dependencies[0] != "collect-evidence" {
		t.Fatalf("projection did not round-trip: %#v", got)
	}

	deleted, err := service.Delete(context.Background(), "goalchainer")
	if err != nil || !deleted.Deleted {
		t.Fatalf("Delete = %#v, %v", deleted, err)
	}
	if _, err := service.Get(context.Background(), "goalchainer"); apiErrorCode(err) != "GOALCHAINER_NOT_CONNECTED" {
		t.Fatalf("Get after delete err = %v", err)
	}
}

func TestPutIsIdempotentForSameRevision(t *testing.T) {
	store := &fakeStore{project: true}
	service := New(store)
	first := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
	service.clock = func() time.Time { return first }
	projection := validProjection()
	if _, err := service.Put(context.Background(), "goalchainer", projection); err != nil {
		t.Fatalf("first Put: %v", err)
	}
	service.clock = func() time.Time { return first.Add(time.Hour) }
	second, err := service.Put(context.Background(), "goalchainer", projection)
	if err != nil {
		t.Fatalf("second Put: %v", err)
	}
	if !second.UpdatedAt.Equal(first) {
		t.Fatalf("idempotent Put changed updatedAt to %s", second.UpdatedAt)
	}
}

func TestPutAcceptsAMettaScheduledTask(t *testing.T) {
	service := New(&fakeStore{project: true})
	projection := validProjection()
	scheduledAt := time.Date(2033, 5, 18, 3, 33, 20, 0, time.UTC)
	projection.Tasks[0].State = "scheduled"
	projection.Tasks[0].ScheduledAt = &scheduledAt
	projection.Tasks[0].ReleaseReason = "not-before constraint has not elapsed"
	projection.Summary.Ready = 0
	projection.Summary.Scheduled = 1

	saved, err := service.Put(context.Background(), "goalchainer", projection)

	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if saved.Tasks[0].ScheduledAt == nil ||
		!saved.Tasks[0].ScheduledAt.Equal(scheduledAt) {
		t.Fatalf("scheduled task did not round-trip: %#v", saved.Tasks[0])
	}
}

func TestPutRejectsInvalidProjectionAndUnknownProject(t *testing.T) {
	tests := map[string]func(*Projection){
		"unknown state":      func(p *Projection) { p.Tasks[0].State = "complete" },
		"duplicate task":     func(p *Projection) { p.Tasks[1].ID = p.Tasks[0].ID },
		"missing dependency": func(p *Projection) { p.Tasks[1].Dependencies = []string{"missing"} },
		"summary drift":      func(p *Projection) { p.Summary.Ready = 0 },
		"progress drift":     func(p *Projection) { p.Summary.Progress = 50 },
		"unsafe agent":       func(p *Projection) { p.Tasks[0].AssignedAgent = "agent/path" },
		"stray schedule": func(p *Projection) {
			now := time.Now().UTC()
			p.Tasks[0].ScheduledAt = &now
		},
		"unknown protocol": func(p *Projection) { p.Protocol = "goalchainer-ui-v2" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			service := New(&fakeStore{project: true})
			projection := validProjection()
			mutate(&projection)
			_, err := service.Put(context.Background(), "goalchainer", projection)
			if apiErrorCode(err) != "INVALID_GOALCHAINER_PROJECTION" {
				t.Fatalf("Put err = %v", err)
			}
		})
	}

	service := New(&fakeStore{})
	if _, err := service.Put(context.Background(), "missing", validProjection()); apiErrorCode(err) != "PROJECT_NOT_FOUND" {
		t.Fatalf("unknown project err = %v", err)
	}
}

func FuzzValidateProjectionRejectsUnknownState(f *testing.F) {
	f.Add("complete")
	f.Add("READY")
	f.Fuzz(func(t *testing.T, state string) {
		switch state {
		case "scheduled", "backlog", "ready", "in-progress", "blocked", "done":
			t.Skip()
		}
		projection := validProjection()
		projection.Tasks[0].State = state
		if validateProjection(projection) == nil {
			t.Fatalf("accepted unknown task state %q", state)
		}
	})
}

func apiErrorCode(err error) string {
	var typed *apierr.Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}
