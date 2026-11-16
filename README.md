# Docker 部署管理平台

[English](./README.en-US.md)

Docker Deployment Platform 是一个面向内网环境的轻量级 Docker Compose 发布平台，包含控制面、节点服务和 Vue 管理界面。它支持项目组、项目、节点、角色权限、发布审批、发布历史、回滚、失败重试和钉钉 / 飞书 / 企业微信通知。

## 管理后台
<img width="1680" height="613" alt="image" src="https://github.com/user-attachments/assets/52263f39-d886-424e-b3f9-3d46831853c2" />


## 钉钉审批
<img width="836" height="542" alt="image" src="https://github.com/user-attachments/assets/5d78e69a-2d4b-4836-9494-ecbcc768c8d0" />



## 功能概览

- **项目管理**：维护项目组、项目、目标节点和 Compose 配置。
- **发布审批**：发布申请先进入待审批状态，审批通过后才真正执行。
- **发布执行**：节点服务调用 Docker Compose 执行 `pull` 和 `up -d --remove-orphans`。
- **历史与诊断**：查看全局或项目发布历史、版本、状态、耗时、审批人和详细输出。
- **回滚与重试**：支持按历史版本回滚，失败任务可重新发布。
- **角色权限**：按路由权限控制页面和操作，账号可绑定多个角色和多个项目。
- **通知集成**：支持钉钉、飞书、企业微信 Webhook；通知可按项目组和角色过滤。
- **国际化**：管理界面支持中文和英文。
- **主题**：支持亮色和暗色模式。

## 架构

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

### 控制面

代码入口：`cmd/server`，核心逻辑在 `internal/server`。

控制面负责：

- 托管管理界面静态资源。
- 提供登录、JWT、用户、角色、路由权限和项目授权。
- 管理项目组、项目、节点和通知渠道。
- 创建发布快照，记录发布历史和审批状态。
- 调度节点服务执行发布。
- 接收节点服务回调并更新发布状态。
- 发送审批、成功、失败、拒绝等通知。

数据默认存储在 `.data/control.sqlite`。数据库结构通过 `internal/server/migrations` 中的 SQL 迁移自动创建和升级。

### 节点服务

代码入口：`cmd/agent`，核心逻辑在 `internal/agent`。

节点服务部署在能访问 Docker 的主机上，负责：

- 通过 Bearer API Key 验证控制面请求。
- 使用文件锁避免同一项目并发发布。
- 将每次 Compose 文件写入版本目录。
- 调用 Docker Compose 拉取镜像并启动服务。
- 发布成功后原子切换 `current.yml` 软链接。
- 发布失败时尝试恢复上一个 `current.yml`。
- 将执行结果重试回调给控制面。

节点端数据布局示例：

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

### 管理界面

代码目录：`web`。

- Vue 3
- TypeScript
- Vite
- Element Plus
- Pinia
- vue-i18n

生产模式下，控制面直接托管 `web/dist`。开发模式下，Vite Dev Server 代理 API 到控制面。

### 通知适配器

代码目录：`internal/notify`。

内置适配器：

- 钉钉
- 飞书
- 企业微信

适配器只保存 Webhook 地址和可选签名密钥，具体消息格式由各适配器实现。后续可以按同样接口扩展其他通知渠道。

## 发布流程

1. 管理员创建项目组、节点和项目。
2. 用户编辑或提交项目 Compose 配置并发起发布。
3. 控制面保存发布快照，状态为 `pending_approval`。
4. 系统按通知渠道配置发送审批消息。
5. 有审批权限的用户点击同意或拒绝，或在页面中处理。
6. 审批通过后，控制面调用目标节点服务。
7. 节点服务执行 Docker Compose 并回调控制面。
8. 控制面更新发布历史和项目当前版本。

发布状态含义：

| 状态               | 说明                                 |
| ------------------ | ------------------------------------ |
| `pending_approval` | 等待审批                             |
| `running`          | 正在发布                             |
| `success`          | 发布成功                             |
| `failed`           | 发布失败                             |
| `partial`          | 发布失败且恢复失败，或状态切换未完成 |
| `failed_recovered` | 发布失败，但已恢复上一版本           |
| `rejected`         | 已拒绝                               |

## 权限模型

系统内置以下核心权限：

| 权限标识          | 默认名称           | 路径               |
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

权限规则：

- 权限管理维护路由名称、权限标识和路由路径。
- 角色管理可以勾选多个路由权限。
- 账号管理可以为账号绑定多个角色和多个项目。
- 账号实际权限是所有角色权限的并集。
- 修改角色权限后，拥有该角色的账号权限会同步重建。
- 删除已被角色使用的权限时，系统会提示先修改对应角色。
- 超级管理员拥有全部权限。

## 通知规则

通知渠道包含：

- 通知类型：钉钉 / 飞书 / 企业微信
- Webhook 地址
- 可选签名密钥
- 通知项目组
- 接收角色

发送条件：

1. 通知渠道启用。
2. 渠道勾选的项目组包含当前项目所属项目组。
3. 非超管接收账号拥有当前项目权限。
4. 非超管接收账号拥有渠道勾选的某个角色。
5. 该角色拥有 `deploy:approve` 权限。
6. 超级管理员默认收到通知。

审批消息中的同意 / 拒绝按钮使用带 token 的后端链接。链接不需要登录态；第一个人处理后，后续点击会提示已处理。

## 快速开始

### 环境要求

- Go 1.22 或更新版本
- Node.js 20.19+ 或 22.13+
- pnpm 9+
- Docker Engine 和 Docker Compose

### 1. 准备环境变量

以下密钥仅作为示例，请自行生成并妥善保存：

```bash
export DEPLOY_JWT_SECRET=$(openssl rand -hex 32)
export DEPLOY_SECRET_KEY=$(openssl rand -hex 16)
export DEPLOY_CALLBACK_TOKEN=$(openssl rand -hex 32)
export DEPLOY_PUBLIC_URL=http://127.0.0.1:8080
export DEPLOY_DATA_DIR=.data
export DEPLOY_STATIC_DIR=web/dist
export DEPLOY_ADDR=127.0.0.1:8080
```

> `DEPLOY_SECRET_KEY` 必须正好 32 个字符，`openssl rand -hex 16` 会生成 32 个字符。

### 2. 构建前端

```bash
cd web
pnpm install
pnpm typecheck
pnpm build
cd ..
```

### 3. 初始化并启动控制面

首次启动时创建超级管理员：

```bash
go run ./cmd/server \
  -init-admin admin \
  -admin-password 'ChangeMe-Please-123'
```

之后启动不需要初始化参数：

```bash
go run ./cmd/server
```

访问：

```text
http://127.0.0.1:8080
```

健康检查：

```bash
curl http://127.0.0.1:8080/api/health
```

### 4. 启动节点服务

生成节点 API Key：

```bash
export AGENT_API_KEY=$(openssl rand -hex 32)
export AGENT_CALLBACK_TOKEN="$DEPLOY_CALLBACK_TOKEN"
export AGENT_CONTROL_URL=http://127.0.0.1:8080
export AGENT_ADDR=127.0.0.1:8120
export AGENT_DATA_DIR=.data/agent

go run ./cmd/agent
```

节点服务要求 `AGENT_API_KEY` 和控制面节点管理中保存的 API Key 完全一致。

### 5. 在页面中完成初始化

1. 登录管理后台。
2. 在「节点管理」中创建节点，地址填写 `http://127.0.0.1:8120`，API Key 填 `AGENT_API_KEY`。
3. 在「项目组管理」中创建项目组。
4. 在「项目管理」中创建项目，选择项目组、目标节点并填写初始 Compose YAML。
5. 在「账号管理」中按需要创建角色、账号、勾选权限和项目。
6. 在「通知渠道」中按项目组和角色配置钉钉 / 飞书 / 企业微信 Webhook。
7. 在项目列表中发起发布并测试审批流程。

## 开发模式

控制面：

```bash
go run ./cmd/server
```

前端：

```bash
cd web
pnpm dev
```

前端开发地址：

```text
http://127.0.0.1:8848
```

节点服务：

```bash
go run ./cmd/agent
```

## 生产部署建议

1. 为控制面配置 HTTPS，或在反向代理层终止 TLS。
2. `DEPLOY_PUBLIC_URL` 必须是钉钉 / 飞书 / 企业微信客户端可访问的地址。
3. 不要提交 `.data` 目录，其中包含数据库、密文和发布配置。
4. 不要在不同环境复用 JWT、加密密钥和回调 token。
5. 定期备份 `.data/control.sqlite` 和节点端 `.data/agent/stacks`。
6. 生产环境建议使用二进制或进程管理器运行，而不是长期使用 `go run`。

构建二进制：

```bash
go build -o bin/server ./cmd/server
go build -o bin/agent ./cmd/agent
```

## 通知回调说明

- Webhook 通知本身由控制面主动访问钉钉 / 飞书 / 企业微信，通常不需要公网入站访问。
- 审批按钮需要钉钉 / 飞书 / 企业微信客户端能够访问 `DEPLOY_PUBLIC_URL`。
- 本地 `127.0.0.1` 只适合本机浏览器测试；测试审批按钮时需要使用公网地址或内网穿透。

## 验证

后端：

```bash
go test ./...
go vet ./...
```

前端：

```bash
cd web
pnpm typecheck
pnpm build
```

## 常见问题

### 节点显示 offline

1. 确认节点服务已启动。
2. 确认控制面配置的节点地址可访问。
3. 确认控制面保存的 API Key 和 `AGENT_API_KEY` 一致。
4. 访问 `http://<agent-address>/health` 检查服务是否存活。

### 发布失败，提示镜像拉取失败

1. 确认目标节点能访问镜像仓库。
2. 检查 Docker 代理、镜像加速和 DNS 配置。
3. 在发布历史点击「详细」查看完整输出。

### 点击审批按钮无法访问

1. 确认 `DEPLOY_PUBLIC_URL` 不是仅供本机访问的地址。
2. 使用公网域名或内网穿透暴露控制面。
3. 确认反向代理没有拦截 `GET /api/deployments/{id}/approve` 和 `GET /api/deployments/{id}/reject`。

### 修改角色后菜单没有变化

后端接口权限立即生效。前端菜单基于登录时保存的权限快照，重新登录或刷新后会同步。

## 截图

截图待补充。建议将图片放在 `docs/screenshots/`，并使用以下文件名：

| 页面         | 建议文件                                     |
| ------------ | -------------------------------------------- |
| 登录页       | `docs/screenshots/login.png`                 |
| 项目管理     | `docs/screenshots/projects.png`              |
| 发布历史     | `docs/screenshots/deployments.png`           |
| 发布详情     | `docs/screenshots/deployment-detail.png`     |
| 账号管理     | `docs/screenshots/accounts.png`              |
| 角色管理     | `docs/screenshots/roles.png`                 |
| 权限管理     | `docs/screenshots/permissions.png`           |
| 通知渠道     | `docs/screenshots/notification-channels.png` |
| 钉钉审批消息 | `docs/screenshots/dingtalk-approval.png`     |

## 当前边界

- 控制面使用单机 SQLite，不是高可用多实例架构。
- 控制面和节点服务之间使用 API Key，尚未实现 mTLS。
- 通知渠道基于 Webhook 机器人，不包含平台应用回调能力。
- 发布日志为回调后的完整输出，暂不支持实时流式日志。

## 许可证

- 本项目采用 [Apache License 2.0](./LICENSE)。
- 管理界面基于 [vue-pure-admin-thin](https://github.com/pure-admin/pure-admin-thin)，其 MIT 许可证保留在 `web/LICENSE`。
