CREATE TABLE route_permissions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  path TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE role_permissions (
  role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_id INTEGER NOT NULL REFERENCES route_permissions(id) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_permissions (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  permission_id INTEGER NOT NULL REFERENCES route_permissions(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, permission_id)
);

INSERT INTO route_permissions(code, name, path) VALUES
  ('project:view', 'Project Management', '/deploy/projects'),
  ('project:create', 'Create Project', '/deploy/projects'),
  ('project:update', 'Edit Project', '/deploy/projects'),
  ('project:delete', 'Delete Project', '/deploy/projects'),
  ('deploy:publish', 'Publish Project', '/deploy/history'),
  ('deploy:rollback', 'Rollback Project', '/deploy/history'),
  ('deploy:retry', 'Retry Deployment', '/deploy/history'),
  ('team:view', 'Team Management', '/deploy/teams'),
  ('node:view', 'Node Management', '/deploy/nodes'),
  ('account:view', 'Account Management', '/accounts/users');

INSERT INTO role_permissions(role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN route_permissions p
  ON EXISTS (
    SELECT 1
    FROM json_each(r.permissions) permission
    WHERE permission.value = p.code
  );

INSERT INTO user_permissions(user_id, permission_id)
SELECT ur.user_id, rp.permission_id
FROM user_roles ur
JOIN role_permissions rp ON rp.role_id = ur.role_id;
