PRAGMA foreign_keys = OFF;

ALTER TABLE teams ADD COLUMN created_at TEXT;
UPDATE teams SET created_at = strftime('%Y-%m-%d %H:%M:%S', 'now');

CREATE TABLE teams_new (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO teams_new(id, name, created_at)
SELECT id, name, created_at FROM teams;

DROP TABLE teams;
ALTER TABLE teams_new RENAME TO teams;

PRAGMA foreign_keys = ON;
