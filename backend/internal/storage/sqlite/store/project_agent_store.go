package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/storage/sqlite/gen"
)

// ResolveProjectAgent creates the stable identity for a project/key pair or
// returns the existing identity after enforcing its immutable role.
func (s *Store) ResolveProjectAgent(
	ctx context.Context,
	record domain.ProjectAgentRecord,
) (domain.ProjectAgentRecord, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	row, err := s.qw.ResolveProjectAgent(ctx, gen.ResolveProjectAgentParams{
		ID:                record.ID,
		ProjectID:         record.ProjectID,
		ProjectAgentKey:   record.ProjectAgentKey,
		Role:              record.Role,
		NotesRelativePath: record.NotesRelativePath,
		CreatedAt:         record.CreatedAt,
		UpdatedAt:         record.UpdatedAt,
	})
	if err != nil {
		return domain.ProjectAgentRecord{}, fmt.Errorf(
			"resolve project agent %s/%s: %w",
			record.ProjectID,
			record.ProjectAgentKey,
			err,
		)
	}
	resolved := projectAgentFromGen(row)
	if resolved.Role != record.Role {
		return domain.ProjectAgentRecord{}, &domain.ProjectAgentRoleMismatchError{
			AgentID:       resolved.ID,
			ExistingRole:  resolved.Role,
			RequestedRole: record.Role,
		}
	}
	return resolved, nil
}

// GetProjectAgentByID returns one project-owned stable agent.
func (s *Store) GetProjectAgentByID(
	ctx context.Context,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
) (domain.ProjectAgentRecord, bool, error) {
	row, err := s.qr.GetProjectAgentByID(ctx, gen.GetProjectAgentByIDParams{
		ProjectID: projectID,
		ID:        agentID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProjectAgentRecord{}, false, nil
	}
	if err != nil {
		return domain.ProjectAgentRecord{}, false, fmt.Errorf(
			"get project agent %s/%s: %w",
			projectID,
			agentID,
			err,
		)
	}
	return projectAgentFromGen(row), true, nil
}

// ListProjectAgents returns all stable agents owned by a project.
func (s *Store) ListProjectAgents(
	ctx context.Context,
	projectID domain.ProjectID,
) ([]domain.ProjectAgentRecord, error) {
	rows, err := s.qr.ListProjectAgents(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project agents for %s: %w", projectID, err)
	}
	out := make([]domain.ProjectAgentRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, projectAgentFromGen(row))
	}
	return out, nil
}

// GetProjectAgentAttemptBySession returns the immutable stable-agent link for a
// session, if the session was created as a project-agent attempt.
func (s *Store) GetProjectAgentAttemptBySession(
	ctx context.Context,
	sessionID domain.SessionID,
) (domain.ProjectAgentAttemptRecord, bool, error) {
	row, err := s.qr.GetProjectAgentAttemptBySession(ctx, sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProjectAgentAttemptRecord{}, false, nil
	}
	if err != nil {
		return domain.ProjectAgentAttemptRecord{}, false, fmt.Errorf(
			"get project agent attempt for session %s: %w",
			sessionID,
			err,
		)
	}
	return projectAgentAttemptFromGen(row), true, nil
}

// ListProjectAgentAttempts returns one stable agent's immutable attempt history
// newest-first.
func (s *Store) ListProjectAgentAttempts(
	ctx context.Context,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
) ([]domain.ProjectAgentAttemptRecord, error) {
	rows, err := s.qr.ListProjectAgentAttempts(ctx, gen.ListProjectAgentAttemptsParams{
		ProjectID:      projectID,
		ProjectAgentID: agentID,
	})
	if err != nil {
		return nil, fmt.Errorf(
			"list project agent attempts for %s/%s: %w",
			projectID,
			agentID,
			err,
		)
	}
	return projectAgentAttemptsFromGen(rows), nil
}

// ListProjectAgentAttemptsByProject returns all immutable stable-agent attempts
// for one project in one query.
func (s *Store) ListProjectAgentAttemptsByProject(
	ctx context.Context,
	projectID domain.ProjectID,
) ([]domain.ProjectAgentAttemptRecord, error) {
	rows, err := s.qr.ListProjectAgentAttemptsByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf(
			"list project agent attempts for project %s: %w",
			projectID,
			err,
		)
	}
	return projectAgentAttemptsFromGen(rows), nil
}

func projectAgentFromGen(row gen.ProjectAgent) domain.ProjectAgentRecord {
	return domain.ProjectAgentRecord{
		ID:                row.ID,
		ProjectID:         row.ProjectID,
		ProjectAgentKey:   row.ProjectAgentKey,
		Role:              row.Role,
		NotesRelativePath: row.NotesRelativePath,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func projectAgentAttemptFromGen(
	row gen.ProjectAgentSessionAttempt,
) domain.ProjectAgentAttemptRecord {
	return domain.ProjectAgentAttemptRecord{
		ProjectID: row.ProjectID,
		AgentID:   row.ProjectAgentID,
		Attempt:   row.AttemptNumber,
		SessionID: row.SessionID,
		CreatedAt: row.CreatedAt,
	}
}

func projectAgentAttemptsFromGen(
	rows []gen.ProjectAgentSessionAttempt,
) []domain.ProjectAgentAttemptRecord {
	out := make([]domain.ProjectAgentAttemptRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, projectAgentAttemptFromGen(row))
	}
	return out
}
