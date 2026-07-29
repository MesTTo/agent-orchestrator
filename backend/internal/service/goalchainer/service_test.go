package goalchainer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
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

func validV2Projection() Projection {
	projection := validProjection()
	projection.Protocol = ProtocolV2
	projection.ProjectID = "goalchainer"
	projection.Agents = []Agent{{
		AgentKey: "local-worker",
		Role:     "worker",
		AgentID:  "7e66da4a-2b4f-4e87-865f-14f554070faa",
	}}
	for index := range projection.Tasks {
		projection.Tasks[index].AssignedAgent = "local-worker"
		projection.Tasks[index].AssignedRole = "worker"
		projection.Tasks[index].AgentKey = "local-worker"
		projection.Tasks[index].AgentID = projection.Agents[0].AgentID
		projection.Tasks[index].AtomspaceLinks = []AtomspaceLink{
			{LinkKey: "project-shared", Scope: "shared", Owner: "project"},
			{LinkKey: "worker-private", Scope: "private", Owner: "local-worker"},
		}
	}
	projection.Tasks[0].Attempts = 1
	projection.Tasks[0].SessionID = "goalchainer-7"
	projection.Tasks[0].SessionAttempts = []SessionAttempt{{
		Attempt:      1,
		AgentID:      projection.Agents[0].AgentID,
		AgentAttempt: 1,
		SessionID:    "goalchainer-7",
		Status:       "working",
		Activity:     "active",
	}}
	return projection
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

func TestV1ProjectionKeepsLegacyStoredShape(t *testing.T) {
	store := &fakeStore{project: true}
	service := New(store)
	projection := validProjection()

	if _, err := service.Put(context.Background(), "goalchainer", projection); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(store.record.Projection, `"agents"`) ||
		strings.Contains(store.record.Projection, `"sessionAttempts"`) ||
		strings.Contains(store.record.Projection, `"projectId"`) {
		t.Fatalf("v1 stored projection gained v2 fields: %s", store.record.Projection)
	}
	var roundTrip Projection
	if err := json.Unmarshal([]byte(store.record.Projection), &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip.Protocol != ProtocolV1 ||
		roundTrip.Tasks[0].AssignedRole != "" ||
		len(roundTrip.Agents) != 0 {
		t.Fatalf("v1 round trip = %#v", roundTrip)
	}
}

func TestV2ProjectionRoundTripsStableAgentsAttemptsAndLinks(t *testing.T) {
	store := &fakeStore{project: true}
	service := New(store)
	projection := validV2Projection()

	saved, err := service.Put(context.Background(), "goalchainer", projection)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ProjectID != "goalchainer" ||
		saved.Agents[0].AgentID != projection.Agents[0].AgentID ||
		saved.Tasks[0].SessionAttempts[0].AgentAttempt != 1 ||
		saved.Tasks[0].AtomspaceLinks[1].Scope != "private" {
		t.Fatalf("saved v2 projection = %#v", saved)
	}
	if !strings.Contains(store.record.Projection, `"projectId":"goalchainer"`) ||
		!strings.Contains(store.record.Projection, `"agentAttempt":1`) {
		t.Fatalf("stored v2 projection = %s", store.record.Projection)
	}
	got, err := service.Get(context.Background(), "goalchainer")
	if err != nil {
		t.Fatal(err)
	}
	if got.Protocol != ProtocolV2 ||
		got.Tasks[0].AgentKey != "local-worker" ||
		got.Tasks[0].SessionAttempts[0].SessionID != "goalchainer-7" {
		t.Fatalf("loaded v2 projection = %#v", got)
	}
}

func TestV2ProjectionAcceptsUnassignedEitherTaskAndHistoricalReroute(t *testing.T) {
	service := New(&fakeStore{project: true})
	unassigned := validV2Projection()
	task := &unassigned.Tasks[1]
	task.Capability = "either"
	task.AssignedAgent = ""
	task.AssignedRole = ""
	task.AgentKey = ""
	task.AgentID = ""
	task.AtomspaceLinks = []AtomspaceLink{{
		LinkKey: "project-shared",
		Scope:   "shared",
		Owner:   "project",
	}}
	if _, err := service.Put(
		context.Background(),
		"goalchainer",
		unassigned,
	); err != nil {
		t.Fatalf("unassigned either task: %v", err)
	}

	rerouted := validV2Projection()
	oldAgentID := domain.ProjectAgentID("272a4c63-64b9-47a0-9624-7fc0fff46aa1")
	rerouted.Agents = append(rerouted.Agents, Agent{
		AgentKey: "old-worker",
		Role:     "reviewer",
		AgentID:  oldAgentID,
	})
	current := rerouted.Tasks[0].SessionAttempts[0]
	current.Attempt = 2
	rerouted.Tasks[0].Attempts = 2
	rerouted.Tasks[0].SessionAttempts = []SessionAttempt{
		{
			Attempt:      1,
			AgentID:      oldAgentID,
			AgentAttempt: 1,
			SessionID:    "goalchainer-3",
			Status:       "terminated",
			Activity:     "exited",
			IsTerminated: true,
		},
		current,
	}
	if _, err := service.Put(
		context.Background(),
		"goalchainer",
		rerouted,
	); err != nil {
		t.Fatalf("historical stable-agent reroute: %v", err)
	}
}

func TestV2ProjectionRejectsCrossProjectAndInconsistentAgentData(t *testing.T) {
	tests := map[string]func(*Projection){
		"route project mismatch": func(p *Projection) {
			p.ProjectID = "other-project"
		},
		"duplicate agent key": func(p *Projection) {
			p.Agents = append(p.Agents, Agent{
				AgentKey: "local-worker",
				Role:     "reviewer",
			})
		},
		"duplicate role": func(p *Projection) {
			p.Agents = append(p.Agents, Agent{
				AgentKey: "review-worker",
				Role:     "worker",
			})
		},
		"duplicate stable UUID": func(p *Projection) {
			p.Agents = append(p.Agents, Agent{
				AgentKey: "review-worker",
				Role:     "reviewer",
				AgentID:  p.Agents[0].AgentID,
			})
		},
		"undeclared agent key": func(p *Projection) {
			p.Tasks[0].AgentKey = "missing-worker"
		},
		"role mismatch": func(p *Projection) {
			p.Tasks[0].AssignedRole = "reviewer"
		},
		"legacy assignment mismatch": func(p *Projection) {
			p.Tasks[0].AssignedAgent = "other-worker"
		},
		"agent UUID mismatch": func(p *Projection) {
			p.Tasks[0].AgentID = "272a4c63-64b9-47a0-9624-7fc0fff46aa1"
		},
		"Goal attempt is zero": func(p *Projection) {
			p.Tasks[0].SessionAttempts[0].Attempt = 0
		},
		"AO attempt is zero": func(p *Projection) {
			p.Tasks[0].SessionAttempts[0].AgentAttempt = 0
		},
		"Goal attempt does not start at one": func(p *Projection) {
			p.Tasks[0].SessionAttempts[0].Attempt = 2
		},
		"unknown session status": func(p *Projection) {
			p.Tasks[0].SessionAttempts[0].Status = "running"
		},
		"unknown activity state": func(p *Projection) {
			p.Tasks[0].SessionAttempts[0].Activity = "sleeping"
		},
		"attempt count mismatch": func(p *Projection) {
			p.Tasks[0].Attempts = 2
		},
		"latest session mismatch": func(p *Projection) {
			p.Tasks[0].SessionID = "goalchainer-8"
		},
		"attempt agent mismatch": func(p *Projection) {
			p.Tasks[0].SessionAttempts[0].AgentID =
				"272a4c63-64b9-47a0-9624-7fc0fff46aa1"
		},
		"no logical links": func(p *Projection) {
			p.Tasks[0].AtomspaceLinks = nil
		},
		"duplicate logical link": func(p *Projection) {
			p.Tasks[0].AtomspaceLinks[1].LinkKey = "project-shared"
		},
		"unknown logical scope": func(p *Projection) {
			p.Tasks[0].AtomspaceLinks[0].Scope = "filesystem"
		},
		"shared link has agent owner": func(p *Projection) {
			p.Tasks[0].AtomspaceLinks[0].Owner = "local-worker"
		},
		"private link has unknown owner": func(p *Projection) {
			p.Tasks[0].AtomspaceLinks[1].Owner = "missing-worker"
		},
		"assigned task lacks shared link": func(p *Projection) {
			p.Tasks[0].AtomspaceLinks = p.Tasks[0].AtomspaceLinks[1:]
		},
		"assigned task lacks current private link": func(p *Projection) {
			p.Agents = append(p.Agents, Agent{
				AgentKey: "review-worker",
				Role:     "reviewer",
			})
			p.Tasks[0].AtomspaceLinks[1].Owner = "review-worker"
		},
		"unassigned local task": func(p *Projection) {
			p.Tasks[1].AssignedAgent = ""
			p.Tasks[1].AssignedRole = ""
			p.Tasks[1].AgentKey = ""
			p.Tasks[1].AgentID = ""
			p.Tasks[1].AtomspaceLinks = p.Tasks[1].AtomspaceLinks[:1]
		},
		"unassigned task has private link": func(p *Projection) {
			p.Tasks[1].Capability = "either"
			p.Tasks[1].AssignedAgent = ""
			p.Tasks[1].AssignedRole = ""
			p.Tasks[1].AgentKey = ""
			p.Tasks[1].AgentID = ""
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			service := New(&fakeStore{project: true})
			projection := validV2Projection()
			mutate(&projection)
			_, err := service.Put(context.Background(), "goalchainer", projection)
			if apiErrorCode(err) != "INVALID_GOALCHAINER_PROJECTION" {
				t.Fatalf("Put err = %v", err)
			}
		})
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
		"unknown protocol": func(p *Projection) { p.Protocol = "goalchainer-ui-v3" },
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
