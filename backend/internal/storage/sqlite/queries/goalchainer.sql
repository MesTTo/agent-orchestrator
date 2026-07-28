-- name: UpsertGoalChainerProjection :one
INSERT INTO goalchainer_projections (
    project_id,
    protocol,
    source_revision,
    projection,
    updated_at
) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(project_id) DO UPDATE SET
    protocol = excluded.protocol,
    source_revision = excluded.source_revision,
    projection = excluded.projection,
    updated_at = excluded.updated_at
WHERE goalchainer_projections.source_revision <> excluded.source_revision
RETURNING project_id, protocol, source_revision, projection, updated_at;

-- name: GetGoalChainerProjection :one
SELECT project_id, protocol, source_revision, projection, updated_at
FROM goalchainer_projections
WHERE project_id = ?;

-- name: DeleteGoalChainerProjection :execrows
DELETE FROM goalchainer_projections WHERE project_id = ?;

-- name: GoalChainerProjectExists :one
SELECT EXISTS (
    SELECT 1 FROM projects WHERE id = ? AND archived_at IS NULL
) AS present;
