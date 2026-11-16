ALTER TABLE deploy_snapshots ADD COLUMN approval_status TEXT NOT NULL DEFAULT '';
ALTER TABLE deploy_snapshots ADD COLUMN approver_id INTEGER;
ALTER TABLE deploy_snapshots ADD COLUMN approval_comment TEXT NOT NULL DEFAULT '';
ALTER TABLE deploy_snapshots ADD COLUMN approval_at TEXT;

CREATE TABLE notification_channels (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  type TEXT NOT NULL,
  name TEXT NOT NULL UNIQUE,
  webhook_url TEXT NOT NULL,
  secret_encrypted TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO route_permissions(code, name, path) VALUES
  ('deploy:approve', 'Approve Deployment', '/deploy/history');
