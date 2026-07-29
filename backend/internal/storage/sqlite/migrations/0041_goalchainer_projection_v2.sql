-- +goose Up
-- GoalChainer v2 adds stable project-agent metadata inside the stored JSON
-- projection. The relational envelope still indexes only project, protocol,
-- source revision, and update time.
-- +goose StatementBegin
DROP TRIGGER IF EXISTS goalchainer_projection_cdc_delete;
DROP TRIGGER IF EXISTS goalchainer_projection_cdc_update;
DROP TRIGGER IF EXISTS goalchainer_projection_cdc_insert;

ALTER TABLE goalchainer_projections RENAME TO goalchainer_projections_v1;

CREATE TABLE goalchainer_projections (
    project_id      TEXT PRIMARY KEY REFERENCES projects (id) ON DELETE CASCADE,
    protocol        TEXT NOT NULL CHECK (
        protocol IN ('goalchainer-ui-v1', 'goalchainer-ui-v2')
    ),
    source_revision TEXT NOT NULL CHECK (
        length(source_revision) = 64
        AND source_revision NOT GLOB '*[^0-9a-f]*'
    ),
    projection      TEXT NOT NULL CHECK (json_valid(projection)),
    updated_at      TIMESTAMP NOT NULL
);

INSERT INTO goalchainer_projections (
    project_id, protocol, source_revision, projection, updated_at
)
SELECT project_id, protocol, source_revision, projection, updated_at
FROM goalchainer_projections_v1;

DROP TABLE goalchainer_projections_v1;

CREATE TRIGGER goalchainer_projection_cdc_insert
AFTER INSERT ON goalchainer_projections
BEGIN
    INSERT INTO change_log (project_id, event_type, payload, created_at)
    VALUES (NEW.project_id, 'goalchainer_projection_changed',
        json_object('projectId', NEW.project_id, 'revision', NEW.source_revision, 'action', 'upsert'),
        NEW.updated_at);
END;

CREATE TRIGGER goalchainer_projection_cdc_update
AFTER UPDATE ON goalchainer_projections
WHEN OLD.source_revision <> NEW.source_revision
BEGIN
    INSERT INTO change_log (project_id, event_type, payload, created_at)
    VALUES (NEW.project_id, 'goalchainer_projection_changed',
        json_object('projectId', NEW.project_id, 'revision', NEW.source_revision, 'action', 'upsert'),
        NEW.updated_at);
END;

CREATE TRIGGER goalchainer_projection_cdc_delete
AFTER DELETE ON goalchainer_projections
BEGIN
    INSERT INTO change_log (project_id, event_type, payload, created_at)
    VALUES (OLD.project_id, 'goalchainer_projection_changed',
        json_object('projectId', OLD.project_id, 'revision', OLD.source_revision, 'action', 'delete'),
        datetime('now'));
END;
-- +goose StatementEnd

-- +goose Down
-- Down migration is deliberately lossless for v1 rows and refuses to proceed
-- while a v2 projection remains connected.
-- +goose StatementBegin
DROP TRIGGER IF EXISTS goalchainer_projection_cdc_delete;
DROP TRIGGER IF EXISTS goalchainer_projection_cdc_update;
DROP TRIGGER IF EXISTS goalchainer_projection_cdc_insert;

ALTER TABLE goalchainer_projections RENAME TO goalchainer_projections_v2;

CREATE TABLE goalchainer_projections (
    project_id      TEXT PRIMARY KEY REFERENCES projects (id) ON DELETE CASCADE,
    protocol        TEXT NOT NULL CHECK (protocol = 'goalchainer-ui-v1'),
    source_revision TEXT NOT NULL CHECK (
        length(source_revision) = 64
        AND source_revision NOT GLOB '*[^0-9a-f]*'
    ),
    projection      TEXT NOT NULL CHECK (json_valid(projection)),
    updated_at      TIMESTAMP NOT NULL
);

INSERT INTO goalchainer_projections (
    project_id, protocol, source_revision, projection, updated_at
)
SELECT project_id, protocol, source_revision, projection, updated_at
FROM goalchainer_projections_v2;

DROP TABLE goalchainer_projections_v2;

CREATE TRIGGER goalchainer_projection_cdc_insert
AFTER INSERT ON goalchainer_projections
BEGIN
    INSERT INTO change_log (project_id, event_type, payload, created_at)
    VALUES (NEW.project_id, 'goalchainer_projection_changed',
        json_object('projectId', NEW.project_id, 'revision', NEW.source_revision, 'action', 'upsert'),
        NEW.updated_at);
END;

CREATE TRIGGER goalchainer_projection_cdc_update
AFTER UPDATE ON goalchainer_projections
WHEN OLD.source_revision <> NEW.source_revision
BEGIN
    INSERT INTO change_log (project_id, event_type, payload, created_at)
    VALUES (NEW.project_id, 'goalchainer_projection_changed',
        json_object('projectId', NEW.project_id, 'revision', NEW.source_revision, 'action', 'upsert'),
        NEW.updated_at);
END;

CREATE TRIGGER goalchainer_projection_cdc_delete
AFTER DELETE ON goalchainer_projections
BEGIN
    INSERT INTO change_log (project_id, event_type, payload, created_at)
    VALUES (OLD.project_id, 'goalchainer_projection_changed',
        json_object('projectId', OLD.project_id, 'revision', OLD.source_revision, 'action', 'delete'),
        datetime('now'));
END;
-- +goose StatementEnd
