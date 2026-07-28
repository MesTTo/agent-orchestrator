package controllers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apispec"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/envelope"
	goalchainersvc "github.com/aoagents/agent-orchestrator/backend/internal/service/goalchainer"
)

const maxGoalChainerBodyBytes = 1 << 20

// GoalChainerController owns the read-only project board projection routes.
type GoalChainerController struct {
	Mgr goalchainersvc.Manager
}

// Register mounts the GoalChainer projection routes.
func (c *GoalChainerController) Register(r chi.Router) {
	r.Get("/projects/{id}/goalchainer", c.get)
	r.Put("/projects/{id}/goalchainer", c.put)
	r.Delete("/projects/{id}/goalchainer", c.delete)
}

func (c *GoalChainerController) get(w http.ResponseWriter, r *http.Request) {
	if c.Mgr == nil {
		apispec.NotImplemented(w, r, "GET", "/api/v1/projects/{id}/goalchainer")
		return
	}
	projection, err := c.Mgr.Get(r.Context(), goalChainerProjectID(r))
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, projection)
}

func (c *GoalChainerController) put(w http.ResponseWriter, r *http.Request) {
	if c.Mgr == nil {
		apispec.NotImplemented(w, r, "PUT", "/api/v1/projects/{id}/goalchainer")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxGoalChainerBodyBytes)
	var projection goalchainersvc.Projection
	if err := decodeJSONStrict(r, &projection); err != nil {
		envelope.WriteAPIError(w, r, http.StatusBadRequest, "bad_request", "INVALID_JSON", "Invalid JSON body", nil)
		return
	}
	saved, err := c.Mgr.Put(r.Context(), goalChainerProjectID(r), projection)
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, saved)
}

func (c *GoalChainerController) delete(w http.ResponseWriter, r *http.Request) {
	if c.Mgr == nil {
		apispec.NotImplemented(w, r, "DELETE", "/api/v1/projects/{id}/goalchainer")
		return
	}
	result, err := c.Mgr.Delete(r.Context(), goalChainerProjectID(r))
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, result)
}

func goalChainerProjectID(r *http.Request) domain.ProjectID {
	return domain.ProjectID(chi.URLParam(r, "id"))
}
