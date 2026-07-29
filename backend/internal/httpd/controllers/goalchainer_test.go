package controllers_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/config"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd"
	goalchainersvc "github.com/aoagents/agent-orchestrator/backend/internal/service/goalchainer"
	"github.com/aoagents/agent-orchestrator/backend/internal/storage/sqlite"
)

const goalChainerDigest = "a3c0c8a6e6a6c9308f32af7fd614159fd64a35f9ce90f9b443f350b3887275fa"

func newGoalChainerServer(t *testing.T) (*httptest.Server, *sqlite.Store) {
	t.Helper()
	store, err := sqlite.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.UpsertProject(t.Context(), domain.ProjectRecord{
		ID:           "goalchainer",
		Path:         "/repos/goalchainer",
		RegisteredAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := store.UpsertProject(t.Context(), domain.ProjectRecord{
		ID:           "project-incident-42",
		Path:         "/repos/project-incident-42",
		RegisteredAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed v2 project: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(
		httpd.NewRouterWithControl(
			config.Config{},
			log,
			nil,
			httpd.APIDeps{GoalChainer: goalchainersvc.New(store)},
			httpd.ControlDeps{},
		),
	)
	t.Cleanup(server.Close)
	return server, store
}

func validGoalChainerBody() string {
	return `{
		"protocol":"goalchainer-ui-v1",
		"sourceRevision":"` + goalChainerDigest + `",
		"threadId":"incident-42",
		"title":"Incident response",
		"owner":"responder",
		"semanticDigest":"` + goalChainerDigest + `",
		"planDigest":"` + goalChainerDigest + `",
		"usageTokens":0,
		"summary":{"total":1,"scheduled":0,"backlog":0,"ready":1,"inProgress":0,"blocked":0,"done":0,"progressPct":0},
		"tasks":[{"id":"collect-evidence","capability":"local","acceptance":"evidence-collected","stakes":"normal","dependencies":[],"state":"ready","upstreamBlocked":false,"attempts":0}],
		"audit":{"events":1,"head":"` + goalChainerDigest + `"},
		"updatedAt":"0001-01-01T00:00:00Z"
	}`
}

func TestGoalChainerProjectionRoutesRoundTripWithoutCreatingSession(t *testing.T) {
	server, store := newGoalChainerServer(t)

	body, status, headers := doRequest(
		t,
		server,
		http.MethodPut,
		"/api/v1/projects/goalchainer/goalchainer",
		validGoalChainerBody(),
	)
	assertJSON(t, headers)
	if status != http.StatusOK {
		t.Fatalf("PUT GoalChainer = %d, body=%s", status, body)
	}

	body, status, _ = doRequest(
		t,
		server,
		http.MethodGet,
		"/api/v1/projects/goalchainer/goalchainer",
		"",
	)
	if status != http.StatusOK || !strings.Contains(string(body), `"state":"ready"`) {
		t.Fatalf("GET GoalChainer = %d, body=%s", status, body)
	}
	sessions, err := store.ListAllSessions(t.Context())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("projection created %d sessions", len(sessions))
	}

	body, status, _ = doRequest(
		t,
		server,
		http.MethodDelete,
		"/api/v1/projects/goalchainer/goalchainer",
		"",
	)
	if status != http.StatusOK || !strings.Contains(string(body), `"deleted":true`) {
		t.Fatalf("DELETE GoalChainer = %d, body=%s", status, body)
	}
}

func TestGoalChainerProjectionRejectsUnknownFields(t *testing.T) {
	server, _ := newGoalChainerServer(t)
	body := validGoalChainerBody()
	body = body[:len(body)-1] + `,"state":"done"}`

	response, status, _ := doRequest(
		t,
		server,
		http.MethodPut,
		"/api/v1/projects/goalchainer/goalchainer",
		body,
	)
	assertErrorCode(t, response, status, http.StatusBadRequest, "INVALID_JSON")
}

func TestGoalChainerProjectionRejectsOversizedBody(t *testing.T) {
	server, _ := newGoalChainerServer(t)
	body := validGoalChainerBody()
	body = body[:len(body)-1] + strings.Repeat(" ", (1<<20)+1) + "}"

	response, status, _ := doRequest(
		t,
		server,
		http.MethodPut,
		"/api/v1/projects/goalchainer/goalchainer",
		body,
	)

	assertErrorCode(t, response, status, http.StatusBadRequest, "INVALID_JSON")
}

func TestGoalChainerV2SyncFixtureRoundTripsStableAgentData(t *testing.T) {
	server, _ := newGoalChainerServer(t)
	fixture, err := os.ReadFile("testdata/goalchainer-ui-v2.json")
	if err != nil {
		t.Fatal(err)
	}

	body, status, _ := doRequest(
		t,
		server,
		http.MethodPut,
		"/api/v1/projects/project-incident-42/goalchainer",
		string(fixture),
	)
	if status != http.StatusOK {
		t.Fatalf("PUT v2 GoalChainer = %d, body=%s", status, body)
	}
	for _, field := range []string{
		`"protocol":"goalchainer-ui-v2"`,
		`"agentKey":"local-worker"`,
		`"agentAttempt":2`,
		`"linkKey":"worker-private"`,
	} {
		if !strings.Contains(string(body), field) {
			t.Fatalf("PUT v2 response missing %s: %s", field, body)
		}
	}

	body, status, _ = doRequest(
		t,
		server,
		http.MethodGet,
		"/api/v1/projects/project-incident-42/goalchainer",
		"",
	)
	if status != http.StatusOK ||
		!strings.Contains(string(body), `"sessionId":"ao-session-2"`) {
		t.Fatalf("GET v2 GoalChainer = %d, body=%s", status, body)
	}
}

func TestGoalChainerV2RejectsRouteMismatchAndPhysicalAtomspacePath(t *testing.T) {
	server, _ := newGoalChainerServer(t)
	fixture, err := os.ReadFile("testdata/goalchainer-ui-v2.json")
	if err != nil {
		t.Fatal(err)
	}

	body, status, _ := doRequest(
		t,
		server,
		http.MethodPut,
		"/api/v1/projects/goalchainer/goalchainer",
		string(fixture),
	)
	assertErrorCode(
		t,
		body,
		status,
		http.StatusBadRequest,
		"INVALID_GOALCHAINER_PROJECTION",
	)

	withPath := strings.Replace(
		string(fixture),
		`"owner": "project"`,
		`"owner": "project", "path": "/private/atomspace.metta"`,
		1,
	)
	body, status, _ = doRequest(
		t,
		server,
		http.MethodPut,
		"/api/v1/projects/project-incident-42/goalchainer",
		withPath,
	)
	assertErrorCode(t, body, status, http.StatusBadRequest, "INVALID_JSON")
}
