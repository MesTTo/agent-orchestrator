package projectagent

import (
	"context"
	"errors"
	"io/fs"

	"github.com/aoagents/agent-orchestrator/backend/internal/agentnotes"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apierr"
)

// Store is the durable surface needed by the project-agent read service.
type Store interface {
	GetProject(ctx context.Context, id string) (domain.ProjectRecord, bool, error)
	ListProjectAgents(ctx context.Context, projectID domain.ProjectID) ([]domain.ProjectAgentRecord, error)
	GetProjectAgentByID(ctx context.Context, projectID domain.ProjectID, agentID domain.ProjectAgentID) (domain.ProjectAgentRecord, bool, error)
	ListProjectAgentAttempts(ctx context.Context, projectID domain.ProjectID, agentID domain.ProjectAgentID) ([]domain.ProjectAgentAttemptRecord, error)
}

// Manager is the controller-facing stable project-agent contract.
type Manager interface {
	List(context.Context, domain.ProjectID) ([]Agent, error)
	Get(context.Context, domain.ProjectID, domain.ProjectAgentID) (Agent, error)
	ReadNotes(context.Context, domain.ProjectID, domain.ProjectAgentID) (Notes, error)
}

// Service assembles project-agent read models from SQLite and confined files.
type Service struct {
	store   Store
	dataDir string
}

var _ Manager = (*Service)(nil)

// New constructs a project-agent read service.
func New(store Store, dataDir string) *Service {
	return &Service{store: store, dataDir: dataDir}
}

// List returns every stable agent owned by one active project.
func (s *Service) List(
	ctx context.Context,
	projectID domain.ProjectID,
) ([]Agent, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	records, err := s.store.ListProjectAgents(ctx, projectID)
	if err != nil {
		return nil, apierr.Internal("PROJECT_AGENTS_LOAD_FAILED", "Failed to load project agents")
	}
	out := make([]Agent, 0, len(records))
	for _, record := range records {
		agent, err := s.readModel(ctx, record)
		if err != nil {
			return nil, err
		}
		out = append(out, agent)
	}
	return out, nil
}

// Get returns one AO-generated identity scoped to one active project.
func (s *Service) Get(
	ctx context.Context,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
) (Agent, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return Agent{}, err
	}
	if !domain.ValidProjectAgentID(agentID) {
		return Agent{}, apierr.Invalid("PROJECT_AGENT_ID_INVALID", "Project agent id must be a canonical UUID", nil)
	}
	record, ok, err := s.store.GetProjectAgentByID(ctx, projectID, agentID)
	if err != nil {
		return Agent{}, apierr.Internal("PROJECT_AGENT_LOAD_FAILED", "Failed to load project agent")
	}
	if !ok {
		return Agent{}, apierr.NotFound("PROJECT_AGENT_NOT_FOUND", "Unknown project agent")
	}
	return s.readModel(ctx, record)
}

// ReadNotes returns validated canonical private MeTTa without evaluating it.
func (s *Service) ReadNotes(
	ctx context.Context,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
) (Notes, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return Notes{}, err
	}
	if !domain.ValidProjectAgentID(agentID) {
		return Notes{}, apierr.Invalid("PROJECT_AGENT_ID_INVALID", "Project agent id must be a canonical UUID", nil)
	}
	record, ok, err := s.store.GetProjectAgentByID(ctx, projectID, agentID)
	if err != nil {
		return Notes{}, apierr.Internal("PROJECT_AGENT_LOAD_FAILED", "Failed to load project agent")
	}
	if !ok {
		return Notes{}, apierr.NotFound("PROJECT_AGENT_NOT_FOUND", "Unknown project agent")
	}
	snapshot, err := agentnotes.Read(
		s.dataDir,
		record.ProjectID,
		record.ID,
		record.NotesRelativePath,
	)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Notes{}, apierr.NotFound("PROJECT_AGENT_NOTES_NOT_FOUND", "Project agent notes are missing")
	case errors.Is(err, agentnotes.ErrSnapshotVersion):
		return Notes{}, apierr.Conflict("PROJECT_AGENT_NOTES_VERSION_UNSUPPORTED", "Project agent notes use an unsupported snapshot schema", nil)
	case errors.Is(err, agentnotes.ErrSnapshotMalformed):
		return Notes{}, apierr.Conflict("PROJECT_AGENT_NOTES_INVALID", "Project agent notes are not a canonical snapshot", nil)
	case errors.Is(err, agentnotes.ErrSnapshotTooLarge):
		return Notes{}, apierr.Conflict("PROJECT_AGENT_NOTES_TOO_LARGE", "Project agent notes exceed the validation limit", nil)
	case err != nil:
		return Notes{}, apierr.Internal("PROJECT_AGENT_NOTES_LOAD_FAILED", "Failed to load project agent notes")
	}
	return Notes{
		AgentID:          record.ID,
		Content:          snapshot.Content,
		Truncated:        snapshot.Truncated,
		Initialized:      snapshot.Initialized,
		PropositionCount: snapshot.PropositionCount,
		SizeBytes:        snapshot.SizeBytes,
		ModifiedAt:       snapshot.ModifiedAt,
	}, nil
}

func (s *Service) readModel(
	ctx context.Context,
	record domain.ProjectAgentRecord,
) (Agent, error) {
	attempts, err := s.store.ListProjectAgentAttempts(ctx, record.ProjectID, record.ID)
	if err != nil {
		return Agent{}, apierr.Internal("PROJECT_AGENT_ATTEMPTS_LOAD_FAILED", "Failed to load project agent attempts")
	}
	out := Agent{
		ID:              record.ID,
		ProjectID:       record.ProjectID,
		ProjectAgentKey: record.ProjectAgentKey,
		Role:            record.Role,
		Attempts:        make([]Attempt, 0, len(attempts)),
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
	}
	for _, attempt := range attempts {
		out.Attempts = append(out.Attempts, Attempt{
			Number:    attempt.Attempt,
			SessionID: attempt.SessionID,
			CreatedAt: attempt.CreatedAt,
		})
	}
	metadata, err := agentnotes.Stat(
		s.dataDir,
		record.ProjectID,
		record.ID,
		record.NotesRelativePath,
	)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return Agent{}, apierr.Internal("PROJECT_AGENT_NOTES_STAT_FAILED", "Failed to inspect project agent notes")
	}
	out.Notes = NotesMetadata{
		Exists:      true,
		Initialized: metadata.SizeBytes > 0,
		SizeBytes:   metadata.SizeBytes,
		ModifiedAt:  metadata.ModifiedAt,
	}
	return out, nil
}

func (s *Service) requireProject(
	ctx context.Context,
	projectID domain.ProjectID,
) error {
	if projectID == "" {
		return apierr.Invalid("PROJECT_ID_REQUIRED", "Project id is required", nil)
	}
	_, ok, err := s.store.GetProject(ctx, string(projectID))
	if err != nil {
		return apierr.Internal("PROJECT_LOAD_FAILED", "Failed to load project")
	}
	if !ok {
		return apierr.NotFound("PROJECT_NOT_FOUND", "Unknown project")
	}
	return nil
}
