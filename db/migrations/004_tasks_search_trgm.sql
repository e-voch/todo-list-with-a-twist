-- +goose Up
-- Trigram indexes so the search (title/description ILIKE '%q%') reads only
-- matching rows instead of the whole table.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX tasks_title_trgm_idx       ON tasks USING GIN (title gin_trgm_ops);
CREATE INDEX tasks_description_trgm_idx ON tasks USING GIN (description gin_trgm_ops);

-- +goose Down
DROP INDEX IF EXISTS tasks_description_trgm_idx;
DROP INDEX IF EXISTS tasks_title_trgm_idx;
-- pg_trgm is left installed: other things may use it.
