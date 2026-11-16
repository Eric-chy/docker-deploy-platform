ALTER TABLE deploy_snapshots
ADD COLUMN approval_token_hash TEXT NOT NULL DEFAULT '';
