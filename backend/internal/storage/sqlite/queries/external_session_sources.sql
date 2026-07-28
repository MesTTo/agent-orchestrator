-- name: GetSessionByExternalSource :one
SELECT s.id, s.project_id, s.num, s.issue_id, s.kind, s.harness,
    s.activity_state, s.activity_last_at, s.is_terminated, s.branch,
    s.workspace_path, s.runtime_handle_id, s.agent_session_id, s.prompt,
    s.created_at, s.updated_at, s.display_name, s.first_signal_at,
    s.preview_url, s.preview_revision, s.cleanup_generation,
    s.runtime_launch_id, s.workspace_repo_path, s.terminate_on_pr_merge,
    s.mcp_profile
FROM external_session_sources AS source
JOIN sessions AS s ON s.id = source.session_id
WHERE source.project_id = ? AND source.source_task_id = ?;

-- name: InsertExternalSessionSource :exec
INSERT INTO external_session_sources (
    project_id, source_task_id, session_id, created_at
) VALUES (?, ?, ?, ?);
