-- +goose Up
-- +goose StatementBegin
CREATE TABLE external_session_sources (
    project_id     TEXT NOT NULL REFERENCES projects (id),
    source_task_id TEXT NOT NULL
        CHECK (
            length(source_task_id) BETWEEN 1 AND 128
            AND source_task_id GLOB '[A-Za-z0-9]*'
            AND source_task_id NOT GLOB '*[^A-Za-z0-9._:-]*'
        ),
    session_id     TEXT NOT NULL UNIQUE REFERENCES sessions (id) ON DELETE CASCADE,
    created_at     TIMESTAMP NOT NULL,
    PRIMARY KEY (project_id, source_task_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE external_session_sources;
-- +goose StatementEnd
