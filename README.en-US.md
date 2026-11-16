# Docker Deployment Platform

[中文](./README.md)

Docker Deployment Platform is a lightweight Docker Compose deployment platform for internal environments. It includes a control plane, a node agent, and a Vue admin console. It supports teams, projects, nodes, role-based permissions, deployment approval, deployment history, rollback, retry, and DingTalk / Feishu / WeCom notifications.


<img width="1681" height="653" alt="image" src="https://github.com/user-attachments/assets/ca287dda-4cef-4289-a99c-f01e13847153" />


## dingtalk
<img width="876" height="808" alt="image" src="https://github.com/user-attachments/assets/2419640a-f55a-4ecd-abf2-2830a68a5b14" />

## Feature Overview

- **Project management**: manage teams, projects, target nodes, and Compose configuration.
- **Deployment approval**: deployment requests wait for approval before execution.
- **Deployment execution**: the agent runs Docker Compose `pull` and `up -d --remove-orphans`.
- **History and diagnostics**: view global or per-project history, versions, status, duration, approver, and detailed output.
- **Rollback and retry**: roll back to a historical version or retry failed deployments.
- **Role-based access control**: control pages and actions through route permissions; an account can have multiple roles and projects.
- **Notifications**: built-in DingTalk, Feishu, and WeCom webhook adapters with team and role filtering.
- **Internationalization**: Chinese and English admin UI.
- **Themes**: light and dark modes.

## Architecture

```text
┌──────────────────────┐
│       Browser        │
│  Vue 3 Admin Console │
└──────────┬───────────┘
           │ HTTPS / HTTP
┌──────────▼───────────┐
│      Control Plane    │
│       cmd/server      │
│  Go + chi + SQLite    │
└─────┬──────────┬─────┘
      │ API      │ callback
┌─────▼─────┐ ┌──▼───────────────┐
│   Agent   │ │ Notification     │
│cmd/agent  │ │ DingTalk/Feishu │
│Docker CLI │ │ WeCom adapters   │
└─────┬─────┘ └──────────────────┘
      │
┌─────▼──────────────┐
│ Docker Compose CLI │
└────────────────────┘
```

### Control Plane

Entry point: `cmd/server`; core logic: `internal/server`.

The control plane is responsible for:

- Serving the admin console static files.
- Login, JWT, users, roles, route permissions, and project authorization.
- Managing teams, projects, nodes, and notification channels.
- Creating deployment snapshots and recording history and approval state.
- Dispatching deployments to node agents.
- Receiving agent callbacks and updating deployment status.
- Sending approval, success, failure, and rejection notifications.

Data is stored in `.data/control.sqlite` by default. The schema is created and upgraded automatically by SQL migrations in `internal/server/migrations`.

### Node Agent

Entry point: `cmd/agent`; core logic: `internal/agent`.

The agent runs on a host with Docker access and is responsible for:

- Authenticating control-plane requests with a Bearer API key.
- Using a file lock to prevent concurrent deployments of the same project.
- Writing each Compose file into its version directory.
- Running Docker Compose to pull images and start services.
- Atomically switching the `current.yml` symlink after a successful deployment.
- Attempting to restore the previous `current.yml` after a failed deployment.
- Retrying result callbacks to the control plane.

Example agent data layout:

```text
.data/agent
├── locks
│   └── my-app.lock
└── stacks
    └── my-app
        ├── current.yml -> versions/v1_after.yml
        └── versions
            ├── v1_after.yml
            └── v2_after.yml
```

### Admin Console

Source directory: `web`.

- Vue 3
- TypeScript
- Vite
- Element Plus
- Pinia
- vue-i18n

In production, the control plane serves `web/dist`. In development, the Vite dev server proxies API requests to the control plane.

### Notification Adapters

Source directory: `internal/notify`.

Built-in adapters:

- DingTalk
- Feishu
- WeCom

Each adapter stores a webhook URL and an optional signing secret. Message formatting is implemented by each channel adapter. Additional channels can be added with the same adapter interface.

## Deployment Flow

1. An administrator creates a team, node, and project.
2. A user edits or submits project Compose configuration and starts a deployment.
3. The control plane creates a deployment snapshot with status `pending_approval`.
4. The system sends approval notifications based on channel configuration.
5. A user with approval permission approves or rejects the request in the UI or from a notification.
6. After approval, the control plane calls the target node agent.
7. The agent runs Docker Compose and calls back the control plane.
8. The control plane updates deployment history and the project's current version.

Deployment statuses:

| Status             | Description                                                                           |
| ------------------ | ------------------------------------------------------------------------------------- |
| `pending_approval` | Waiting for approval                                                                  |
| `running`          | Deploying                                                                             |
| `success`          | Deployment succeeded                                                                  |
| `failed`           | Deployment failed                                                                     |
| `partial`          | Deployment failed and recovery failed, or the current-version switch did not complete |
| `failed_recovered` | Deployment failed, but the previous version was recovered                             |
| `rejected`         | Rejected                                                                              |

## Permission Model

Core permissions:

| Code              | Default Name       | Route              |
| ----------------- | ------------------ | ------------------ |
| `project:view`    | Project Management | `/deploy/projects` |
| `project:create`  | Create Project     | `/deploy/projects` |
| `project:update`  | Edit Project       | `/deploy/projects` |
| `project:delete`  | Delete Project     | `/deploy/projects` |
| `deploy:publish`  | Publish Project    | `/deploy/history`  |
| `deploy:rollback` | Rollback Project   | `/deploy/history`  |
| `deploy:retry`    | Retry Deployment   | `/deploy/history`  |
| `deploy:approve`  | Approve Deployment | `/deploy/history`  |
| `team:view`       | Team Management    | `/deploy/teams`    |
| `node:view`       | Node Management    | `/deploy/nodes`    |
| `account:view`    | Account Management | `/accounts/users`  |

Permission rules:

- Permission management maintains route name, permission code, and route path.
- Role management can select multiple route permissions.
- Account management can bind an account to multiple roles and projects.
- An account's effective permissions are the union of all role permissions.
- When role permissions change, permissions of accounts with that role are rebuilt.
- Deleting a permission assigned to a role is blocked until the role is updated.
- Super administrators have all permissions.

## Notification Rules

A notification channel contains:

- Channel type: DingTalk / Feishu / WeCom
- Webhook URL
- Optional signing secret
- Teams to notify
- Recipient roles

Send conditions:

1. The channel is enabled.
2. The channel's selected teams include the project's team.
3. A non-admin recipient account has access to the project.
4. A non-admin recipient account has one of the roles selected by the channel.
5. That role has `deploy:approve`.
6. Super administrators receive notifications by default.

Approval buttons use tokenized backend links. No login state is required. After the first user processes a request, later requests show an already-processed result.

## Quick Start

### Requirements

- Go 1.22 or newer
- Node.js 20.19+ or 22.13+
- pnpm 9+
- Docker Engine and Docker Compose

### 1. Prepare Environment Variables

Generate and store your own secrets; the following commands are examples:

```bash
export DEPLOY_JWT_SECRET=$(openssl rand -hex 32)
export DEPLOY_SECRET_KEY=$(openssl rand -hex 16)
export DEPLOY_CALLBACK_TOKEN=$(openssl rand -hex 32)
export DEPLOY_PUBLIC_URL=http://127.0.0.1:8080
export DEPLOY_DATA_DIR=.data
export DEPLOY_STATIC_DIR=web/dist
export DEPLOY_ADDR=127.0.0.1:8080
```

> `DEPLOY_SECRET_KEY` must be exactly 32 characters; `openssl rand -hex 16` generates 32 characters.

### 2. Build the Frontend

```bash
cd web
pnpm install
pnpm typecheck
pnpm build
cd ..
```

### 3. Initialize and Start the Control Plane

Create the first super administrator on the first start:

```bash
go run ./cmd/server \
  -init-admin admin \
  -admin-password 'ChangeMe-Please-123'
```

Subsequent starts do not need the initialization flags:

```bash
go run ./cmd/server
```

Open:

```text
http://127.0.0.1:8080
```

Health check:

```bash
curl http://127.0.0.1:8080/api/health
```

### 4. Start a Node Agent

Generate an agent API key:

```bash
export AGENT_API_KEY=$(openssl rand -hex 32)
export AGENT_CALLBACK_TOKEN="$DEPLOY_CALLBACK_TOKEN"
export AGENT_CONTROL_URL=http://127.0.0.1:8080
export AGENT_ADDR=127.0.0.1:8120
export AGENT_DATA_DIR=.data/agent

go run ./cmd/agent
```

The agent's `AGENT_API_KEY` must exactly match the API key saved for the node in the control plane.

### 5. Complete Initialization in the UI

1. Sign in to the admin console.
2. In **Node Management**, create a node with address `http://127.0.0.1:8120` and the value of `AGENT_API_KEY`.
3. In **Team Management**, create a team.
4. In **Project Management**, create a project, select its team and node, and enter the initial Compose YAML.
5. In **Account Management**, create roles and accounts, then select permissions and projects.
6. In **Notification Channels**, configure DingTalk / Feishu / WeCom webhooks by team and role.
7. Start a deployment from the project list and test the approval flow.

## Development Mode

Control plane:

```bash
go run ./cmd/server
```

Frontend:

```bash
cd web
pnpm dev
```

Frontend development address:

```text
http://127.0.0.1:8848
```

Node agent:

```bash
go run ./cmd/agent
```

## Production Recommendations

1. Serve the control plane over HTTPS or terminate TLS at a reverse proxy.
2. `DEPLOY_PUBLIC_URL` must be reachable from DingTalk / Feishu / WeCom clients.
3. Do not commit `.data`; it contains the database, encrypted secrets, and deployment configuration.
4. Do not reuse JWT, encryption, or callback tokens across environments.
5. Back up `.data/control.sqlite` and the agent's `.data/agent/stacks` regularly.
6. Use binaries or a process manager in production rather than running `go run` indefinitely.

Build binaries:

```bash
go build -o bin/server ./cmd/server
go build -o bin/agent ./cmd/agent
```

## Notification Callback Notes

- Webhook notifications are outbound requests from the control plane and normally do not require public inbound access.
- Approval buttons require DingTalk / Feishu / WeCom clients to access `DEPLOY_PUBLIC_URL`.
- Local `127.0.0.1` is only suitable for local browser testing. Use a public URL or tunnel when testing approval buttons.

## Validation

Backend:

```bash
go test ./...
go vet ./...
```

Frontend:

```bash
cd web
pnpm typecheck
pnpm build
```

## Troubleshooting

### Node shows offline

1. Confirm the agent is running.
2. Confirm the configured node address is reachable from the control plane.
3. Confirm the API key stored by the control plane matches `AGENT_API_KEY`.
4. Check `http://<agent-address>/health`.

### Deployment fails while pulling images

1. Confirm the target node can access the image registry.
2. Check Docker proxy, registry mirror, and DNS settings.
3. Open **Detail** in deployment history to read the full output.

### Approval buttons cannot be opened

1. Confirm `DEPLOY_PUBLIC_URL` is not a localhost-only address.
2. Expose the control plane through a public domain or tunnel.
3. Ensure the reverse proxy does not block `GET /api/deployments/{id}/approve` or `GET /api/deployments/{id}/reject`.

### Menus do not change immediately after role updates

Backend API permissions take effect immediately. Frontend menus are built from the permission snapshot saved at login; sign in again or refresh to synchronize them.

## Screenshots

Screenshots to be added. Place images in `docs/screenshots/` and use the following names:

| Page                      | Suggested File                               |
| ------------------------- | -------------------------------------------- |
| Login                     | `docs/screenshots/login.png`                 |
| Projects                  | `docs/screenshots/projects.png`              |
| Deployment history        | `docs/screenshots/deployments.png`           |
| Deployment detail         | `docs/screenshots/deployment-detail.png`     |
| Accounts                  | `docs/screenshots/accounts.png`              |
| Roles                     | `docs/screenshots/roles.png`                 |
| Permissions               | `docs/screenshots/permissions.png`           |
| Notification channels     | `docs/screenshots/notification-channels.png` |
| DingTalk approval message | `docs/screenshots/dingtalk-approval.png`     |

## Current Boundaries

- The control plane uses single-node SQLite and is not a high-availability multi-instance system.
- Control-plane-to-agent communication uses an API key; mTLS is not implemented yet.
- Notifications use webhook robots rather than platform application callbacks.
- Deployment logs are complete callback outputs; real-time streaming logs are not implemented.

## License

- This project is licensed under the [Apache License 2.0](./LICENSE).
- The admin console is based on [vue-pure-admin-thin](https://github.com/pure-admin/pure-admin-thin); its MIT license is preserved in `web/LICENSE`.
