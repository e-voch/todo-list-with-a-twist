-- +goose Up
CREATE TABLE IF NOT EXISTS tasks (
    id INT PRIMARY KEY,
    title TEXT NOT NULL,
	description TEXT NOT NULL,
    recommended_steps TEXT
);

-- +goose Down
DROP TABLE tasks;