-- +goose Up
ALTER TABLE tasks ADD COLUMN uuid UUID DEFAULT uuidv7();

-- +goose Down
ALTER TABLE tasks DROP COLUMN uuid;