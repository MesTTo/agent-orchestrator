package sessionmanager

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/agentnotes"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

const (
	firstProjectAgentID  = "7e66da4a-2b4f-4e87-865f-14f554070faa"
	secondProjectAgentID = "272a4c63-64b9-47a0-9624-7fc0fff46aa1"
	thirdProjectAgentID  = "76dd7750-15bb-4a96-ad0d-de776444818a"
)

func projectAgentManager(
	t *testing.T,
	store Store,
	baseStore *fakeStore,
	dataDir string,
	ids ...string,
) (*Manager, *fakeRuntime) {
	t.Helper()
	index := 0
	newProjectAgentID := func() string {
		if index >= len(ids) {
			t.Fatalf("unexpected stable project-agent identity generation")
		}
		id := ids[index]
		index++
		return id
	}
	runtimeController := &fakeRuntime{}
	manager := New(Deps{
		Runtime:           runtimeController,
		Agents:            fakeAgents{},
		Workspace:         &fakeWorkspace{},
		Store:             store,
		Messenger:         &fakeMessenger{},
		Lifecycle:         &fakeLCM{store: baseStore},
		DataDir:           dataDir,
		LookPath:          func(string) (string, error) { return "/bin/true", nil },
		NewProjectAgentID: newProjectAgentID,
	})
	return manager, runtimeController
}

func TestSpawnReusesStableProjectAgentAndAppendsAttempts(t *testing.T) {
	store := newFakeStore()
	store.projects["mer"] = domain.ProjectRecord{ID: "mer", Config: testRoleAgents()}
	dataDir := t.TempDir()
	manager, runtimeController := projectAgentManager(
		t,
		store,
		store,
		dataDir,
		firstProjectAgentID,
		secondProjectAgentID,
	)
	cfg := ports.SpawnConfig{
		ProjectID:        "mer",
		Kind:             domain.KindWorker,
		ProjectAgentKey:  "evidence-worker",
		ProjectAgentRole: "researcher",
	}

	first, _, _, err := manager.Spawn(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	firstPrivate := filepath.Join(
		dataDir,
		filepath.FromSlash(agentnotes.AgentRelativePath("mer", firstProjectAgentID)),
	)
	noteBody := "(MemorySnapshot oh-my-goals-atomspace-v1 agent \"mer\" \"" +
		firstProjectAgentID + "\")\n" +
		"(MemoryContent note-1 \"survives termination\")\n" +
		"(MemoryProposition note-1 session observation)\n"
	if err := os.WriteFile(firstPrivate, []byte(noteBody), 0o600); err != nil {
		t.Fatal(err)
	}
	if freed, err := manager.Kill(t.Context(), first.ID); err != nil || !freed {
		t.Fatalf("terminate first attempt = freed:%v err:%v", freed, err)
	}
	second, _, _, err := manager.Spawn(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first.ProjectAgentID != firstProjectAgentID ||
		second.ProjectAgentID != firstProjectAgentID {
		t.Fatalf("stable ids = %q, %q", first.ProjectAgentID, second.ProjectAgentID)
	}
	if first.ProjectAgentAttempt != 1 || second.ProjectAgentAttempt != 2 {
		t.Fatalf("attempts = %d, %d", first.ProjectAgentAttempt, second.ProjectAgentAttempt)
	}
	if got := len(store.projectAttempts[firstProjectAgentID]); got != 2 {
		t.Fatalf("persisted attempts = %d, want 2", got)
	}
	env := runtimeController.lastCfg.Env
	if env[EnvProjectAgentID] != firstProjectAgentID ||
		env[EnvProjectAgentRole] != "researcher" ||
		env[EnvProjectAgentAttempt] != "2" {
		t.Fatalf("stable project-agent environment = %#v", env)
	}

	if _, err := os.Stat(firstPrivate); err != nil {
		t.Fatalf("stable private notes: %v", err)
	}
	snapshot, err := agentnotes.Read(
		dataDir,
		"mer",
		firstProjectAgentID,
		agentnotes.AgentRelativePath("mer", firstProjectAgentID),
	)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Content != noteBody {
		t.Fatalf("private notes changed across terminate and respawn: %q", snapshot.Content)
	}
	discardedPrivate := filepath.Join(
		dataDir,
		filepath.FromSlash(agentnotes.AgentRelativePath("mer", secondProjectAgentID)),
	)
	if _, err := os.Stat(discardedPrivate); !os.IsNotExist(err) {
		t.Fatalf("discarded candidate private notes error = %v, want not exist", err)
	}
	shared := filepath.Join(
		dataDir,
		filepath.FromSlash(agentnotes.ProjectRelativePath("mer")),
	)
	if _, err := os.Stat(shared); err != nil {
		t.Fatalf("shared project Atomspace: %v", err)
	}
}

func TestSpawnProjectAgentPersistenceFailureRemovesOnlyNewPrivateNotes(t *testing.T) {
	store := newFakeStore()
	store.projects["mer"] = domain.ProjectRecord{ID: "mer", Config: testRoleAgents()}
	store.resolveAgentErr = errors.New("injected persistence failure")
	dataDir := t.TempDir()
	manager, _ := projectAgentManager(
		t,
		store,
		store,
		dataDir,
		firstProjectAgentID,
	)

	_, _, _, err := manager.Spawn(t.Context(), ports.SpawnConfig{
		ProjectID:        "mer",
		Kind:             domain.KindWorker,
		ProjectAgentKey:  "evidence-worker",
		ProjectAgentRole: "researcher",
	})
	if err == nil || !errors.Is(err, store.resolveAgentErr) {
		t.Fatalf("spawn error = %v", err)
	}
	if len(store.sessions) != 0 {
		t.Fatalf("sessions created after identity persistence failure: %#v", store.sessions)
	}
	private := filepath.Join(
		dataDir,
		filepath.FromSlash(agentnotes.AgentRelativePath("mer", firstProjectAgentID)),
	)
	if _, err := os.Stat(private); !os.IsNotExist(err) {
		t.Fatalf("new private notes error = %v, want not exist", err)
	}
	shared := filepath.Join(
		dataDir,
		filepath.FromSlash(agentnotes.ProjectRelativePath("mer")),
	)
	if _, err := os.Stat(shared); err != nil {
		t.Fatalf("shared project Atomspace was removed: %v", err)
	}
}

func TestSpawnRejectsProjectAgentRoleChange(t *testing.T) {
	store := newFakeStore()
	store.projects["mer"] = domain.ProjectRecord{ID: "mer", Config: testRoleAgents()}
	manager, _ := projectAgentManager(
		t,
		store,
		store,
		t.TempDir(),
		firstProjectAgentID,
		secondProjectAgentID,
	)
	base := ports.SpawnConfig{
		ProjectID:        "mer",
		Kind:             domain.KindWorker,
		ProjectAgentKey:  "evidence-worker",
		ProjectAgentRole: "researcher",
	}
	if _, _, _, err := manager.Spawn(t.Context(), base); err != nil {
		t.Fatal(err)
	}
	base.ProjectAgentRole = "reviewer"

	_, _, _, err := manager.Spawn(t.Context(), base)
	if !errors.Is(err, domain.ErrProjectAgentRoleMismatch) {
		t.Fatalf("spawn error = %v, want immutable role conflict", err)
	}
	if len(store.sessions) != 1 {
		t.Fatalf("sessions = %d, want original only", len(store.sessions))
	}
}

func TestExternalSourceReplayUsesSemanticProjectAgentIdentity(t *testing.T) {
	base := newFakeStore()
	base.projects["mer"] = domain.ProjectRecord{ID: "mer", Config: testRoleAgents()}
	store := &blockingExternalStore{
		fakeStore: base,
		sources:   make(map[string]domain.SessionID),
		entered:   make(chan struct{}, 1),
		release:   make(chan struct{}),
	}
	close(store.release)
	generated := 0
	manager := New(Deps{
		Runtime:   &fakeRuntime{},
		Agents:    fakeAgents{},
		Workspace: &fakeWorkspace{},
		Store:     store,
		Messenger: &fakeMessenger{},
		Lifecycle: &fakeLCM{store: base},
		DataDir:   t.TempDir(),
		LookPath:  func(string) (string, error) { return "/bin/true", nil },
		NewProjectAgentID: func() string {
			generated++
			if generated == 1 {
				return firstProjectAgentID
			}
			return secondProjectAgentID
		},
	})
	cfg := ports.SpawnConfig{
		ProjectID:        "mer",
		Kind:             domain.KindWorker,
		SourceTaskID:     "goalchainer:thread:task:1",
		ProjectAgentKey:  "evidence-worker",
		ProjectAgentRole: "researcher",
	}
	first, _, _, created, err := manager.SpawnFromExternalSource(t.Context(), cfg)
	if err != nil || !created {
		t.Fatalf("first spawn = %#v, created=%v, err=%v", first, created, err)
	}
	replayed, _, _, created, err := manager.SpawnFromExternalSource(t.Context(), cfg)
	if err != nil || created || replayed.ID != first.ID {
		t.Fatalf("replay = %#v, created=%v, err=%v", replayed, created, err)
	}
	if generated != 1 {
		t.Fatalf("stable identities generated = %d, want 1", generated)
	}

	cfg.ProjectAgentKey = "different-worker"
	if _, _, _, _, err := manager.SpawnFromExternalSource(
		t.Context(),
		cfg,
	); !errors.Is(err, domain.ErrExternalProjectAgentMismatch) {
		t.Fatalf("different semantic key error = %v", err)
	}
	cfg.ProjectAgentKey = "evidence-worker"
	cfg.ProjectAgentRole = "reviewer"
	if _, _, _, _, err := manager.SpawnFromExternalSource(
		t.Context(),
		cfg,
	); !errors.Is(err, domain.ErrProjectAgentRoleMismatch) {
		t.Fatalf("different role error = %v", err)
	}
	if generated != 1 {
		t.Fatalf("conflicting replays generated a new identity: %d", generated)
	}
}
