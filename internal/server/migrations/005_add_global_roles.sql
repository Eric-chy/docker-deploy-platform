CREATE TABLE roles (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL UNIQUE,
  permissions TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_roles (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, role_id)
);

INSERT INTO roles(code, name, permissions) VALUES
  ('viewer', '只读', '["project:view"]'),
  ('operator', '发布', '["project:view","deploy:publish","deploy:rollback","deploy:retry"]'),
  ('manager', '管理', '["project:view","project:create","project:update","project:delete","deploy:publish","deploy:rollback","deploy:retry"]');

INSERT INTO user_roles(user_id, role_id)
SELECT DISTINCT ut.user_id, r.id
FROM user_teams ut
JOIN roles r ON r.code = ut.role;
