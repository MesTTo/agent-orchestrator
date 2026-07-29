package projectagent

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/agentnotes"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apierr"
)

const serviceTestAgentID domain.ProjectAgentID = "7e66da4a-2b4f-4e87-865f-14f554070faa"

type fakeStore struct {
	project       domain.ProjectRecord
	projectOK     bool
	projectErr    error
	agents        []domain.ProjectAgentRecord
	listAgentsErr error
	agent         domain.ProjectAgentRecord
	agentOK       bool
	getAgentErr   error
	attempts      []domain.ProjectAgentAttemptRecord
	attemptsErr   error

	gotProjectID domain.ProjectID
	gotAgentID   domain.ProjectAgentID
}

func (f *fakeStore) GetProject(
	_ context.Context,
	id string,
) (domain.ProjectRecord, bool, error) {
	f.gotProjectID = domain.ProjectID(id)
	return f.project, f.projectOK, f.projectErr
}

func (f *fakeStore) ListProjectAgents(
	_ context.Context,
	projectID domain.ProjectID,
) ([]domain.ProjectAgentRecord, error) {
	f.gotProjectID = projectID
	return f.agents, f.listAgentsErr
}

func (f *fakeStore) GetProjectAgentByID(
	_ context.Context,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
) (domain.ProjectAgentRecord, bool, error) {
	f.gotProjectID = projectID
	f.gotAgentID = agentID
	return f.agent, f.agentOK, f.getAgentErr
}

func (f *fakeStore) ListProjectAgentAttempts(
	_ context.Context,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
) ([]domain.ProjectAgentAttemptRecord, error) {
	f.gotProjectID = projectID
	f.gotAgentID = agentID
	return f.attempts, f.attemptsErr
}

func testRecord(projectID domain.ProjectID) domain.ProjectAgentRecord {
	now := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
	return domain.ProjectAgentRecord{
		ID:                serviceTestAgentID,
		ProjectID:         projectID,
		ProjectAgentKey:   "evidence-worker",
		Role:              "researcher",
		NotesRelativePath: agentnotes.AgentRelativePath(projectID, serviceTestAgentID),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func readyService(t *testing.T, projectID domain.ProjectID) (*Service, *fakeStore, string) {
	t.Helper()
	dataDir := t.TempDir()
	record := testRecord(projectID)
	if _, err := agentnotes.Ensure(
		dataDir,
		projectID,
		record.ID,
		record.NotesRelativePath,
	); err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{
		project:   domain.ProjectRecord{ID: string(projectID)},
		projectOK: true,
		agents:    []domain.ProjectAgentRecord{record},
		agent:     record,
		agentOK:   true,
	}
	return New(store, dataDir), store, dataDir
}

func apiCodeAndKind(t *testing.T, err error) (string, apierr.Kind) {
	t.Helper()
	var typed *apierr.Error
	if !errors.As(err, &typed) {
		t.Fatalf("error = %v, want *apierr.Error", err)
	}
	return typed.Code, typed.Kind
}

func TestListScopesAgentsAndProjectsAttempts(t *testing.T) {
	service, store, _ := readyService(t, "research")
	store.attempts = []domain.ProjectAgentAttemptRecord{
		{
			ProjectID: "research",
			AgentID:   serviceTestAgentID,
			Attempt:   2,
			SessionID: "research-7",
			CreatedAt: time.Date(2026, 7, 29, 5, 0, 0, 0, time.UTC),
		},
		{
			ProjectID: "research",
			AgentID:   serviceTestAgentID,
			Attempt:   1,
			SessionID: "research-3",
			CreatedAt: time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC),
		},
	}

	agents, err := service.List(context.Background(), "research")
	if err != nil {
		t.Fatal(err)
	}
	if store.gotProjectID != "research" || store.gotAgentID != serviceTestAgentID {
		t.Fatalf("store scope = %q/%q", store.gotProjectID, store.gotAgentID)
	}
	if len(agents) != 1 || len(agents[0].Attempts) != 2 {
		t.Fatalf("agents = %#v", agents)
	}
	if agents[0].Attempts[0].Number != 2 ||
		agents[0].Attempts[0].SessionID != "research-7" ||
		agents[0].Attempts[1].Number != 1 {
		t.Fatalf("attempt projection = %#v", agents[0].Attempts)
	}
	if !agents[0].Notes.Exists || agents[0].Notes.Initialized {
		t.Fatalf("empty notes metadata = %#v", agents[0].Notes)
	}
}

func TestArchivedProjectRemainsReadable(t *testing.T) {
	service, store, _ := readyService(t, "research")
	store.project.ArchivedAt = time.Date(2026, 7, 30, 4, 0, 0, 0, time.UTC)

	agents, err := service.List(context.Background(), "research")
	if err != nil || len(agents) != 1 {
		t.Fatalf("List archived = %#v, %v", agents, err)
	}
	got, err := service.Get(context.Background(), "research", serviceTestAgentID)
	if err != nil || got.ID != serviceTestAgentID {
		t.Fatalf("Get archived = %#v, %v", got, err)
	}
	if _, err := service.ReadNotes(
		context.Background(),
		"research",
		serviceTestAgentID,
	); err != nil {
		t.Fatalf("ReadNotes archived: %v", err)
	}
}

func TestProjectAndAgentLookupErrorsAreTyped(t *testing.T) {
	tests := []struct {
		name string
		run  func(*Service) error
		edit func(*fakeStore)
		code string
		kind apierr.Kind
	}{
		{
			name: "project id required",
			run: func(service *Service) error {
				_, err := service.List(context.Background(), "")
				return err
			},
			code: "PROJECT_ID_REQUIRED",
			kind: apierr.KindInvalid,
		},
		{
			name: "project missing",
			run: func(service *Service) error {
				_, err := service.List(context.Background(), "missing")
				return err
			},
			edit: func(store *fakeStore) { store.projectOK = false },
			code: "PROJECT_NOT_FOUND",
			kind: apierr.KindNotFound,
		},
		{
			name: "project store failure",
			run: func(service *Service) error {
				_, err := service.List(context.Background(), "research")
				return err
			},
			edit: func(store *fakeStore) { store.projectErr = errors.New("database unavailable") },
			code: "PROJECT_LOAD_FAILED",
			kind: apierr.KindInternal,
		},
		{
			name: "agent id invalid",
			run: func(service *Service) error {
				_, err := service.Get(context.Background(), "research", "caller-key")
				return err
			},
			code: "PROJECT_AGENT_ID_INVALID",
			kind: apierr.KindInvalid,
		},
		{
			name: "agent missing",
			run: func(service *Service) error {
				_, err := service.Get(context.Background(), "research", serviceTestAgentID)
				return err
			},
			edit: func(store *fakeStore) { store.agentOK = false },
			code: "PROJECT_AGENT_NOT_FOUND",
			kind: apierr.KindNotFound,
		},
		{
			name: "agent store failure",
			run: func(service *Service) error {
				_, err := service.Get(context.Background(), "research", serviceTestAgentID)
				return err
			},
			edit: func(store *fakeStore) { store.getAgentErr = errors.New("database unavailable") },
			code: "PROJECT_AGENT_LOAD_FAILED",
			kind: apierr.KindInternal,
		},
		{
			name: "agent list failure",
			run: func(service *Service) error {
				_, err := service.List(context.Background(), "research")
				return err
			},
			edit: func(store *fakeStore) { store.listAgentsErr = errors.New("database unavailable") },
			code: "PROJECT_AGENTS_LOAD_FAILED",
			kind: apierr.KindInternal,
		},
		{
			name: "attempt list failure",
			run: func(service *Service) error {
				_, err := service.Get(context.Background(), "research", serviceTestAgentID)
				return err
			},
			edit: func(store *fakeStore) { store.attemptsErr = errors.New("database unavailable") },
			code: "PROJECT_AGENT_ATTEMPTS_LOAD_FAILED",
			kind: apierr.KindInternal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, store, _ := readyService(t, "research")
			if test.edit != nil {
				test.edit(store)
			}
			code, kind := apiCodeAndKind(t, test.run(service))
			if code != test.code || kind != test.kind {
				t.Fatalf("error = %s/%v, want %s/%v", code, kind, test.code, test.kind)
			}
		})
	}
}

func TestReadNotesReturnsCanonicalBoundedProjection(t *testing.T) {
	service, store, dataDir := readyService(t, "research")
	body := "(MemorySnapshot oh-my-goals-atomspace-v1 agent \"research\" \"" +
		string(serviceTestAgentID) + "\")\n" +
		"(MemoryContent note-1 \"" + strings.Repeat("x", int(agentnotes.MaxReadBytes)) + "\")\n" +
		"(MemoryProposition note-1 session observation)\n"
	privatePath := agentnotes.AgentRelativePath("research", serviceTestAgentID)
	if err := os.WriteFile(store.agentNotesPath(dataDir), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	notes, err := service.ReadNotes(context.Background(), "research", serviceTestAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if !notes.Truncated || !notes.Initialized || notes.PropositionCount != 1 {
		t.Fatalf("notes = %#v", notes)
	}
	if notes.SizeBytes != int64(len(body)) || len(notes.Content) > int(agentnotes.MaxReadBytes) {
		t.Fatalf("bounded notes size = %d/%d", notes.SizeBytes, len(notes.Content))
	}
	if !strings.HasPrefix(notes.Content, "(MemorySnapshot ") ||
		strings.Contains(notes.Content, strings.Repeat("x", 1024)) {
		t.Fatalf("bounded content did not preserve complete lines: %q", notes.Content)
	}
	if privatePath != store.agent.NotesRelativePath {
		t.Fatalf("test fixture path drift: %q != %q", privatePath, store.agent.NotesRelativePath)
	}
}

func (f *fakeStore) agentNotesPath(dataDir string) string {
	return dataDir + string(os.PathSeparator) +
		strings.ReplaceAll(f.agent.NotesRelativePath, "/", string(os.PathSeparator))
}

func TestReadNotesMapsFileAndSnapshotFailures(t *testing.T) {
	tests := []struct {
		name string
		body string
		edit func(*fakeStore, string) error
		code string
		kind apierr.Kind
	}{
		{
			name: "missing notes",
			edit: func(store *fakeStore, dataDir string) error {
				return os.Remove(store.agentNotesPath(dataDir))
			},
			code: "PROJECT_AGENT_NOTES_NOT_FOUND",
			kind: apierr.KindNotFound,
		},
		{
			name: "unsupported version",
			body: "(MemorySnapshot oh-my-goals-atomspace-v2 agent \"research\" \"" +
				string(serviceTestAgentID) + "\")\n",
			code: "PROJECT_AGENT_NOTES_VERSION_UNSUPPORTED",
			kind: apierr.KindConflict,
		},
		{
			name: "malformed snapshot",
			body: "(MemorySnapshot oh-my-goals-atomspace-v1 agent \"other\" \"" +
				string(serviceTestAgentID) + "\")\n",
			code: "PROJECT_AGENT_NOTES_INVALID",
			kind: apierr.KindConflict,
		},
		{
			name: "snapshot too large",
			body: strings.Repeat("x", int(agentnotes.MaxSnapshotBytes)+1),
			code: "PROJECT_AGENT_NOTES_TOO_LARGE",
			kind: apierr.KindConflict,
		},
		{
			name: "internal read failure",
			edit: func(store *fakeStore, _ string) error {
				store.agent.NotesRelativePath = "caller/controlled/path.metta"
				return nil
			},
			code: "PROJECT_AGENT_NOTES_LOAD_FAILED",
			kind: apierr.KindInternal,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, store, dataDir := readyService(t, "research")
			if test.body != "" {
				if err := os.WriteFile(store.agentNotesPath(dataDir), []byte(test.body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if test.edit != nil {
				if err := test.edit(store, dataDir); err != nil {
					t.Fatal(err)
				}
			}
			_, err := service.ReadNotes(context.Background(), "research", serviceTestAgentID)
			code, kind := apiCodeAndKind(t, err)
			if code != test.code || kind != test.kind {
				t.Fatalf("error = %s/%v, want %s/%v", code, kind, test.code, test.kind)
			}
		})
	}
}

func TestListMapsNotesStatFailure(t *testing.T) {
	service, store, _ := readyService(t, "research")
	store.agents[0].NotesRelativePath = "caller/controlled/path.metta"

	_, err := service.List(context.Background(), "research")
	code, kind := apiCodeAndKind(t, err)
	if code != "PROJECT_AGENT_NOTES_STAT_FAILED" || kind != apierr.KindInternal {
		t.Fatalf("error = %s/%v", code, kind)
	}
}
