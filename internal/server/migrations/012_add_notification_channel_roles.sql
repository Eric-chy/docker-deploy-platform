CREATE TABLE notification_channel_roles (
  channel_id INTEGER NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
  role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  PRIMARY KEY (channel_id, role_id)
);

INSERT INTO notification_channel_roles(channel_id, role_id)
SELECT nc.id, ur.role_id
FROM notification_channels nc
JOIN user_roles ur ON ur.user_id = nc.user_id
WHERE NOT EXISTS (
  SELECT 1 FROM users u
  WHERE u.id = nc.user_id AND CAST(u.is_admin AS INTEGER) = 1
);
