CREATE TABLE user_projects (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, project_id)
);

INSERT INTO user_projects(user_id, project_id)
SELECT ut.user_id, p.id
FROM user_teams ut
JOIN projects p ON p.team_id = ut.team_id;

ALTER TABLE notification_channels
ADD COLUMN user_id INTEGER REFERENCES users(id) ON DELETE CASCADE;

UPDATE notification_channels
SET user_id = (
  SELECT id FROM users WHERE CAST(is_admin AS INTEGER) = 1 ORDER BY id LIMIT 1
)
WHERE user_id IS NULL;
