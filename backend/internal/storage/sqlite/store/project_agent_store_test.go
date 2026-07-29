package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

const (
	agentOne domain.ProjectAgentID = "7e66da4a-2b4f-4e87-865f-14f554070faa"
	agentTwo domain.ProjectAgentID = "272a4c63-64b9-47a0-9624-7fc0fff46aa1"
)

func TestResolveProjectAgentKeepsGeneratedIdentityAndRoleImmutable(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedProject(t, store, "research")
	record := projectAgentRecord("research", agentOne, "evidence-worker", "researcher")

	first, err := store.ResolveProjectAgent(ctx, record)
	if err != nil {
		t.Fatal(err)
	}
	retry := projectAgentRecord("research", agentTwo, "evidence-worker", "researcher")
	second, err := store.ResolveProjectAgent(ctx, retry)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.ID != agentOne {
		t.Fatalf("retry identity = %s, want %s", second.ID, first.ID)
	}

	retry.Role = "reviewer"
	if _, err := store.ResolveProjectAgent(ctx, retry); !errors.Is(err, domain.ErrProjectAgentRoleMismatch) {
		t.Fatalf("role conflict = %v", err)
	}
}

func TestProjectAgentAttemptsAreImmutableMonotonicAndProjectScoped(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedProject(t, store, "research")
	seedProject(t, store, "other")
	agent, err := store.ResolveProjectAgent(
		ctx,
		projectAgentRecord("research", agentOne, "evidence-worker", "researcher"),
	)
	if err != nil {
		t.Fatal(err)
	}

	const count = 8
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := sampleRecord("research")
			rec.ProjectAgentID = agent.ID
			_, err := store.CreateSession(ctx, rec)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	attempts, err := store.ListProjectAgentAttempts(ctx, "research", agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != count {
		t.Fatalf("attempts = %d, want %d", len(attempts), count)
	}
	projectAttempts, err := store.ListProjectAgentAttemptsByProject(ctx, "research")
	if err != nil || len(projectAttempts) != count {
		t.Fatalf("project attempts = %#v, %v", projectAttempts, err)
	}
	seen := map[int64]bool{}
	for _, attempt := range attempts {
		if attempt.ProjectID != "research" || attempt.AgentID != agent.ID {
			t.Fatalf("cross-project attempt = %#v", attempt)
		}
		seen[attempt.Attempt] = true
		session, ok, err := store.GetSession(ctx, attempt.SessionID)
		if err != nil || !ok {
			t.Fatalf("get session %s = %#v, %v, %v", attempt.SessionID, session, ok, err)
		}
		if session.ProjectAgentID != agent.ID ||
			session.ProjectAgentAttempt != attempt.Attempt {
			t.Fatalf("session link = %#v, attempt = %#v", session, attempt)
		}
	}
	for number := int64(1); number <= count; number++ {
		if !seen[number] {
			t.Fatalf("missing attempt %d in %#v", number, seen)
		}
	}

	crossProject := sampleRecord("other")
	crossProject.ProjectAgentID = agent.ID
	if _, err := store.CreateSession(ctx, crossProject); err == nil {
		t.Fatal("expected cross-project agent link to fail")
	}
}

func TestExternalSourceRejectsAProjectAgentMismatch(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedProject(t, store, "research")
	for _, record := range []domain.ProjectAgentRecord{
		projectAgentRecord("research", agentOne, "evidence-worker", "researcher"),
		projectAgentRecord("research", agentTwo, "writer", "publisher"),
	} {
		if _, err := store.ResolveProjectAgent(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	first := sampleRecord("research")
	first.ProjectAgentID = agentOne
	created, wasCreated, err := store.CreateSessionForExternalSource(
		ctx,
		first,
		"goalchainer:incident:collect:1",
	)
	if err != nil || !wasCreated {
		t.Fatalf("create = %#v, %v, %v", created, wasCreated, err)
	}
	retry := sampleRecord("research")
	retry.ProjectAgentID = agentOne
	same, wasCreated, err := store.CreateSessionForExternalSource(
		ctx,
		retry,
		"goalchainer:incident:collect:1",
	)
	if err != nil || wasCreated || same.ID != created.ID ||
		same.ProjectAgentAttempt != created.ProjectAgentAttempt {
		t.Fatalf("retry = %#v, %v, %v", same, wasCreated, err)
	}

	retry.ProjectAgentID = agentTwo
	if _, _, err := store.CreateSessionForExternalSource(
		ctx,
		retry,
		"goalchainer:incident:collect:1",
	); !errors.Is(err, domain.ErrExternalProjectAgentMismatch) {
		t.Fatalf("mismatch = %v", err)
	}
}

func TestArchivePreservesProjectAgentsAndAttempts(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedProject(t, store, "research")
	record, err := store.ResolveProjectAgent(
		ctx,
		projectAgentRecord("research", agentOne, "evidence-worker", "researcher"),
	)
	if err != nil {
		t.Fatal(err)
	}
	session := sampleRecord("research")
	session.ProjectAgentID = record.ID
	if _, err := store.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.ArchiveProject(ctx, "research", time.Now().UTC()); err != nil || !ok {
		t.Fatalf("archive = %v, %v", ok, err)
	}
	agents, err := store.ListProjectAgents(ctx, "research")
	if err != nil || len(agents) != 1 {
		t.Fatalf("agents after archive = %#v, %v", agents, err)
	}
	attempts, err := store.ListProjectAgentAttempts(ctx, "research", record.ID)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("attempts after archive = %#v, %v", attempts, err)
	}
}

func projectAgentRecord(
	projectID domain.ProjectID,
	id domain.ProjectAgentID,
	key domain.ProjectAgentKey,
	role domain.ProjectAgentRole,
) domain.ProjectAgentRecord {
	now := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
	return domain.ProjectAgentRecord{
		ID:                id,
		ProjectID:         projectID,
		ProjectAgentKey:   key,
		Role:              role,
		NotesRelativePath: "mcp/oh-my-goals/projects/project/atomspaces/agents/" + string(id) + "/notes.metta",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}
