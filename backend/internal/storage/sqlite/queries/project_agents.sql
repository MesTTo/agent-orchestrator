-- name: ResolveProjectAgent :one
INSERT INTO project_agents (
    id, project_id, project_agent_key, role, notes_relative_path,
    created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (project_id, project_agent_key) DO UPDATE SET
    project_agent_key = excluded.project_agent_key
RETURNING id, project_id, project_agent_key, role, notes_relative_path,
    created_at, updated_at;

-- name: GetProjectAgentByID :one
SELECT id, project_id, project_agent_key, role, notes_relative_path,
    created_at, updated_at
FROM project_agents
WHERE project_id = ? AND id = ?;

-- name: ListProjectAgents :many
SELECT id, project_id, project_agent_key, role, notes_relative_path,
    created_at, updated_at
FROM project_agents
WHERE project_id = ?
ORDER BY created_at, id;

-- name: NextProjectAgentAttemptNumber :one
SELECT COALESCE(MAX(attempt_number), 0) + 1 AS next
FROM project_agent_session_attempts
WHERE project_id = ? AND project_agent_id = ?;

-- name: InsertProjectAgentAttempt :exec
INSERT INTO project_agent_session_attempts (
    project_id, project_agent_id, attempt_number, session_id, created_at
) VALUES (?, ?, ?, ?, ?);

-- name: GetProjectAgentAttemptBySession :one
SELECT project_id, project_agent_id, attempt_number, session_id, created_at
FROM project_agent_session_attempts
WHERE session_id = ?;

-- name: ListProjectAgentAttempts :many
SELECT project_id, project_agent_id, attempt_number, session_id, created_at
FROM project_agent_session_attempts
WHERE project_id = ? AND project_agent_id = ?
ORDER BY attempt_number DESC;

-- name: ListProjectAgentAttemptsByProject :many
SELECT project_id, project_agent_id, attempt_number, session_id, created_at
FROM project_agent_session_attempts
WHERE project_id = ?
ORDER BY project_agent_id, attempt_number DESC;

-- name: ListAllProjectAgentAttempts :many
SELECT project_id, project_agent_id, attempt_number, session_id, created_at
FROM project_agent_session_attempts
ORDER BY project_id, project_agent_id, attempt_number DESC;
