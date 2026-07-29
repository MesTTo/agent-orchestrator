package controllers_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/config"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apierr"
	projectagentsvc "github.com/aoagents/agent-orchestrator/backend/internal/service/projectagent"
)

const controllerProjectAgentID domain.ProjectAgentID = "7e66da4a-2b4f-4e87-865f-14f554070faa"

type fakeProjectAgentManager struct {
	agents []projectagentsvc.Agent
	agent  projectagentsvc.Agent
	notes  projectagentsvc.Notes
	err    error

	projectID domain.ProjectID
	agentID   domain.ProjectAgentID
}

func (f *fakeProjectAgentManager) List(
	_ context.Context,
	projectID domain.ProjectID,
) ([]projectagentsvc.Agent, error) {
	f.projectID = projectID
	return f.agents, f.err
}

func (f *fakeProjectAgentManager) Get(
	_ context.Context,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
) (projectagentsvc.Agent, error) {
	f.projectID = projectID
	f.agentID = agentID
	return f.agent, f.err
}

func (f *fakeProjectAgentManager) ReadNotes(
	_ context.Context,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
) (projectagentsvc.Notes, error) {
	f.projectID = projectID
	f.agentID = agentID
	return f.notes, f.err
}

func newProjectAgentServer(
	t *testing.T,
	manager projectagentsvc.Manager,
) *httptest.Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(httpd.NewRouterWithControl(
		config.Config{},
		log,
		nil,
		httpd.APIDeps{ProjectAgents: manager},
		httpd.ControlDeps{},
	))
	t.Cleanup(server.Close)
	return server
}

func projectAgentFixtures() (projectagentsvc.Agent, projectagentsvc.Notes) {
	createdAt := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
	agent := projectagentsvc.Agent{
		ID:              controllerProjectAgentID,
		ProjectID:       "research",
		ProjectAgentKey: "evidence-worker",
		Role:            "researcher",
		Notes: projectagentsvc.NotesMetadata{
			Exists:      true,
			Initialized: true,
			SizeBytes:   180,
			ModifiedAt:  createdAt,
		},
		Attempts: []projectagentsvc.Attempt{{
			Number:    2,
			SessionID: "research-7",
			CreatedAt: createdAt,
		}},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
	notes := projectagentsvc.Notes{
		AgentID:          controllerProjectAgentID,
		Content:          "(MemorySnapshot oh-my-goals-atomspace-v1 agent \"research\" \"" + string(controllerProjectAgentID) + "\")\n",
		Initialized:      true,
		PropositionCount: 3,
		SizeBytes:        180,
		ModifiedAt:       createdAt,
	}
	return agent, notes
}

func TestProjectAgentRoutesReturnScopedEnvelopes(t *testing.T) {
	agent, notes := projectAgentFixtures()
	manager := &fakeProjectAgentManager{
		agents: []projectagentsvc.Agent{agent},
		agent:  agent,
		notes:  notes,
	}
	server := newProjectAgentServer(t, manager)

	body, status, headers := doRequest(
		t,
		server,
		http.MethodGet,
		"/api/v1/projects/research/agents",
		"",
	)
	assertJSON(t, headers)
	if status != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", status, body)
	}
	var list struct {
		Agents []projectagentsvc.Agent `json:"agents"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatal(err)
	}
	if manager.projectID != "research" || len(list.Agents) != 1 ||
		list.Agents[0].ID != controllerProjectAgentID ||
		list.Agents[0].Attempts[0].Number != 2 {
		t.Fatalf("list envelope = %#v, scope=%q", list, manager.projectID)
	}

	path := "/api/v1/projects/research/agents/" + string(controllerProjectAgentID)
	body, status, _ = doRequest(t, server, http.MethodGet, path, "")
	if status != http.StatusOK {
		t.Fatalf("detail status = %d, body=%s", status, body)
	}
	var detail struct {
		Agent projectagentsvc.Agent `json:"agent"`
	}
	if err := json.Unmarshal(body, &detail); err != nil {
		t.Fatal(err)
	}
	if manager.projectID != "research" ||
		manager.agentID != controllerProjectAgentID ||
		detail.Agent.ProjectAgentKey != "evidence-worker" {
		t.Fatalf("detail envelope = %#v, scope=%q/%q", detail, manager.projectID, manager.agentID)
	}

	body, status, _ = doRequest(t, server, http.MethodGet, path+"/notes", "")
	if status != http.StatusOK {
		t.Fatalf("notes status = %d, body=%s", status, body)
	}
	var notesEnvelope struct {
		Notes projectagentsvc.Notes `json:"notes"`
	}
	if err := json.Unmarshal(body, &notesEnvelope); err != nil {
		t.Fatal(err)
	}
	if manager.projectID != "research" ||
		manager.agentID != controllerProjectAgentID ||
		notesEnvelope.Notes.AgentID != controllerProjectAgentID ||
		notesEnvelope.Notes.PropositionCount != 3 {
		t.Fatalf("notes envelope = %#v, scope=%q/%q", notesEnvelope, manager.projectID, manager.agentID)
	}
}

func TestProjectAgentRoutesPreserveTypedStatusAndEmptyListShape(t *testing.T) {
	manager := &fakeProjectAgentManager{}
	server := newProjectAgentServer(t, manager)
	body, status, _ := doRequest(
		t,
		server,
		http.MethodGet,
		"/api/v1/projects/research/agents",
		"",
	)
	if status != http.StatusOK || strings.TrimSpace(string(body)) != `{"agents":[]}` {
		t.Fatalf("empty list = %d, %s", status, body)
	}

	manager.err = apierr.NotFound("PROJECT_AGENT_NOT_FOUND", "Unknown project agent")
	body, status, _ = doRequest(
		t,
		server,
		http.MethodGet,
		"/api/v1/projects/research/agents/"+string(controllerProjectAgentID),
		"",
	)
	assertErrorCode(
		t,
		body,
		status,
		http.StatusNotFound,
		"PROJECT_AGENT_NOT_FOUND",
	)

	manager.err = apierr.Conflict(
		"PROJECT_AGENT_NOTES_INVALID",
		"Project agent notes are not a canonical snapshot",
		nil,
	)
	body, status, _ = doRequest(
		t,
		server,
		http.MethodGet,
		"/api/v1/projects/research/agents/"+string(controllerProjectAgentID)+"/notes",
		"",
	)
	assertErrorCode(
		t,
		body,
		status,
		http.StatusConflict,
		"PROJECT_AGENT_NOTES_INVALID",
	)
}

func TestProjectAgentRoutesAreReadOnly(t *testing.T) {
	manager := &fakeProjectAgentManager{}
	server := newProjectAgentServer(t, manager)

	body, status, _ := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/projects/research/agents",
		`{"id":"caller-controlled"}`,
	)
	if status != http.StatusMethodNotAllowed {
		t.Fatalf("POST stable agent = %d, body=%s", status, body)
	}
}
