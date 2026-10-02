CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
	description TEXT NOT NULL,
    recommended_steps TEXT 
);