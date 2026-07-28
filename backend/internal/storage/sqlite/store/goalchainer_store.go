package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/storage/sqlite/gen"
)

// GoalChainerProjectExists reports whether an active project can accept a projection.
func (s *Store) GoalChainerProjectExists(ctx context.Context, projectID domain.ProjectID) (bool, error) {
	present, err := s.qr.GoalChainerProjectExists(ctx, projectID)
	if err != nil {
		return false, fmt.Errorf("check GoalChainer project %s: %w", projectID, err)
	}
	return present, nil
}

// UpsertGoalChainerProjection atomically replaces one project projection.
func (s *Store) UpsertGoalChainerProjection(
	ctx context.Context,
	record domain.GoalChainerProjectionRecord,
) (domain.GoalChainerProjectionRecord, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	row, err := s.qw.UpsertGoalChainerProjection(
		ctx,
		gen.UpsertGoalChainerProjectionParams{
			ProjectID:      record.ProjectID,
			Protocol:       record.Protocol,
			SourceRevision: record.SourceRevision,
			Projection:     record.Projection,
			UpdatedAt:      record.UpdatedAt,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return s.GetGoalChainerProjection(ctx, record.ProjectID)
	}
	if err != nil {
		return domain.GoalChainerProjectionRecord{}, fmt.Errorf(
			"upsert GoalChainer projection for %s: %w",
			record.ProjectID,
			err,
		)
	}
	return goalChainerProjectionFromGen(row), nil
}

// GetGoalChainerProjection returns a project projection, if one is connected.
func (s *Store) GetGoalChainerProjection(
	ctx context.Context,
	projectID domain.ProjectID,
) (domain.GoalChainerProjectionRecord, error) {
	row, err := s.qr.GetGoalChainerProjection(ctx, projectID)
	if err != nil {
		return domain.GoalChainerProjectionRecord{}, err
	}
	return goalChainerProjectionFromGen(row), nil
}

// DeleteGoalChainerProjection disconnects a project's GoalChainer projection.
func (s *Store) DeleteGoalChainerProjection(
	ctx context.Context,
	projectID domain.ProjectID,
) (bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	rows, err := s.qw.DeleteGoalChainerProjection(ctx, projectID)
	if err != nil {
		return false, fmt.Errorf("delete GoalChainer projection for %s: %w", projectID, err)
	}
	return rows > 0, nil
}

func goalChainerProjectionFromGen(row gen.GoalchainerProjection) domain.GoalChainerProjectionRecord {
	return domain.GoalChainerProjectionRecord{
		ProjectID:      row.ProjectID,
		Protocol:       row.Protocol,
		SourceRevision: row.SourceRevision,
		Projection:     row.Projection,
		UpdatedAt:      row.UpdatedAt,
	}
}
