-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions
ADD COLUMN mcp_profile TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN mcp_profile;
-- +goose StatementEnd
