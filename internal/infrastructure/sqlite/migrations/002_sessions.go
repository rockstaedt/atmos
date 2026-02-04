package migrations

func init() {
	Register(Migration{
		Version:     2,
		Description: "Add sessions table",
		SQL: `
CREATE TABLE sessions (
    token TEXT PRIMARY KEY,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
`,
	})
}
