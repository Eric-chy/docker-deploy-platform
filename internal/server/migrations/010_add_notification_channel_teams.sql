CREATE TABLE notification_channel_teams (
  channel_id INTEGER NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
  team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
  PRIMARY KEY (channel_id, team_id)
);

INSERT INTO notification_channel_teams(channel_id, team_id)
SELECT nc.id, t.id
FROM notification_channels nc
JOIN teams t ON CAST(1 AS INTEGER) = 1
WHERE nc.user_id IS NOT NULL;
