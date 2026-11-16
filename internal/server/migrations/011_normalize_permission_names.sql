UPDATE route_permissions
SET name = CASE code
  WHEN 'project:view' THEN 'Project Management'
  WHEN 'project:create' THEN 'Create Project'
  WHEN 'project:update' THEN 'Edit Project'
  WHEN 'project:delete' THEN 'Delete Project'
  WHEN 'deploy:publish' THEN 'Publish Project'
  WHEN 'deploy:rollback' THEN 'Rollback Project'
  WHEN 'deploy:retry' THEN 'Retry Deployment'
  WHEN 'team:view' THEN 'Team Management'
  WHEN 'node:view' THEN 'Node Management'
  WHEN 'account:view' THEN 'Account Management'
  WHEN 'deploy:approve' THEN 'Approve Deployment'
  ELSE name
END
WHERE code IN (
  'project:view',
  'project:create',
  'project:update',
  'project:delete',
  'deploy:publish',
  'deploy:rollback',
  'deploy:retry',
  'team:view',
  'node:view',
  'account:view',
  'deploy:approve'
);
