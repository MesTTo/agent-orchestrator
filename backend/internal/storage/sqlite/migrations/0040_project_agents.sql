-- +goose Up
-- +goose StatementBegin
CREATE TABLE project_agents (
    id                  TEXT NOT NULL PRIMARY KEY,
    project_id          TEXT NOT NULL REFERENCES projects (id),
    project_agent_key   TEXT NOT NULL
        CHECK (
            length(project_agent_key) BETWEEN 1 AND 128
            AND project_agent_key GLOB '[A-Za-z0-9]*'
            AND project_agent_key NOT GLOB '*[^A-Za-z0-9._:-]*'
        ),
    role                TEXT NOT NULL
        CHECK (
            length(role) BETWEEN 1 AND 128
            AND role GLOB '[A-Za-z0-9]*'
            AND role NOT GLOB '*[^A-Za-z0-9._:-]*'
        ),
    notes_relative_path TEXT NOT NULL
        CHECK (
            length(notes_relative_path) BETWEEN 1 AND 512
            AND substr(notes_relative_path, 1, 1) <> '/'
            AND notes_relative_path NOT GLOB '*\*'
            AND notes_relative_path NOT GLOB '*../*'
            AND notes_relative_path NOT GLOB '*/..*'
        ),
    created_at          TIMESTAMP NOT NULL,
    updated_at          TIMESTAMP NOT NULL,
    UNIQUE (project_id, project_agent_key),
    UNIQUE (project_id, id)
);

CREATE UNIQUE INDEX sessions_project_id_id
    ON sessions (project_id, id);

CREATE TABLE project_agent_session_attempts (
    project_id       TEXT NOT NULL,
    project_agent_id TEXT NOT NULL,
    attempt_number   INTEGER NOT NULL CHECK (attempt_number > 0),
    session_id       TEXT NOT NULL UNIQUE,
    created_at       TIMESTAMP NOT NULL,
    PRIMARY KEY (project_id, project_agent_id, attempt_number),
    FOREIGN KEY (project_id, project_agent_id)
        REFERENCES project_agents (project_id, id),
    FOREIGN KEY (project_id, session_id)
        REFERENCES sessions (project_id, id) ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE project_agent_session_attempts;
DROP INDEX sessions_project_id_id;
DROP TABLE project_agents;
-- +goose StatementEnd
