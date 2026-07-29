package controllers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apispec"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/envelope"
	projectagentsvc "github.com/aoagents/agent-orchestrator/backend/internal/service/projectagent"
)

// ProjectAgentsController owns stable project-agent metadata and bounded notes
// reads. Identity creation happens only through session spawn.
type ProjectAgentsController struct {
	Mgr projectagentsvc.Manager
}

// Register mounts the read-only project-agent routes.
func (c *ProjectAgentsController) Register(r chi.Router) {
	r.Get("/projects/{id}/agents", c.list)
	r.Get("/projects/{id}/agents/{agentId}", c.get)
	r.Get("/projects/{id}/agents/{agentId}/notes", c.notes)
}

func (c *ProjectAgentsController) list(w http.ResponseWriter, r *http.Request) {
	if c.Mgr == nil {
		apispec.NotImplemented(w, r, "GET", "/api/v1/projects/{id}/agents")
		return
	}
	agents, err := c.Mgr.List(r.Context(), projectAgentProjectID(r))
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	if agents == nil {
		agents = []projectagentsvc.Agent{}
	}
	envelope.WriteJSON(w, http.StatusOK, ListProjectAgentsResponse{Agents: agents})
}

func (c *ProjectAgentsController) get(w http.ResponseWriter, r *http.Request) {
	if c.Mgr == nil {
		apispec.NotImplemented(w, r, "GET", "/api/v1/projects/{id}/agents/{agentId}")
		return
	}
	agent, err := c.Mgr.Get(
		r.Context(),
		projectAgentProjectID(r),
		projectAgentID(r),
	)
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, ProjectAgentResponse{Agent: agent})
}

func (c *ProjectAgentsController) notes(w http.ResponseWriter, r *http.Request) {
	if c.Mgr == nil {
		apispec.NotImplemented(w, r, "GET", "/api/v1/projects/{id}/agents/{agentId}/notes")
		return
	}
	notes, err := c.Mgr.ReadNotes(
		r.Context(),
		projectAgentProjectID(r),
		projectAgentID(r),
	)
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, ProjectAgentNotesResponse{Notes: notes})
}

func projectAgentProjectID(r *http.Request) domain.ProjectID {
	return domain.ProjectID(chi.URLParam(r, "id"))
}

func projectAgentID(r *http.Request) domain.ProjectAgentID {
	return domain.ProjectAgentID(chi.URLParam(r, "agentId"))
}
