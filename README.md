# OpenStory

> 开源自托管 AI 视频创作工作流平台 — 输入创意，生成剧本、角色、分镜、图片/视频，合成 15-30 秒预览视频。

## 架构

```
cmd/
  api/            HTTP API 服务器 (Gin)
  worker/         Asynq 异步任务 worker
  outbox-relay/   Outbox → Kafka 事件中继
  consumer/       Kafka 事件消费者

internal/
  config/         环境变量配置
  observability/  zerolog 日志、OpenTelemetry (TODO)
  db/             PostgreSQL 连接池 (pgx)
  http/           Gin 路由、中间件、Handler
  auth/           认证授权 (TODO)
  project/        项目管理 (TODO)
  workflow/       工作流 DAG 编排 (TODO)
  task/           任务执行 (TODO)
  asset/          MinIO 资源管理 (TODO)
  provider/       AI 模型供应商网关 (TODO)
  eventbus/       Kafka 事件总线 (TODO)
  outbox/         Outbox Pattern (TODO)
  consumer/       事件消费处理 (TODO)
  billing/        积分计费 (TODO)
  moderation/     内容审核 (TODO)
```

## 技术栈

| 组件 | 技术 |
|---|---|
| 语言 | Go 1.26 |
| HTTP | Gin |
| 数据库 | PostgreSQL 17 (pgx) |
| 缓存 | Redis 8 |
| 消息队列 | Apache Kafka 4.2.0 |
| 对象存储 | MinIO |
| 日志 | zerolog |
| 迁移 | goose |
| 容器化 | Docker Compose |

## 数据库 Schema

共 16 张业务表，3 个迁移文件 (`migrations/`)：

| 分类 | 表名 | 说明 |
|---|---|---|
| 用户 | `users` | 用户账户，支持 admin/user 角色 |
| 用户 | `refresh_tokens` | JWT 刷新令牌 |
| 项目 | `projects` | 视频创作项目 |
| 项目 | `works` | 最终产出作品 |
| 项目 | `assets` | MinIO 媒体资源 |
| 工作流 | `workflows` | DAG 工作流定义 |
| 工作流 | `workflow_nodes` | DAG 节点 |
| 工作流 | `workflow_edges` | DAG 边 |
| 工作流 | `workflow_versions` | 工作流版本快照 |
| 任务 | `generation_tasks` | AI 生成任务 (幂等键) |
| 任务 | `task_events` | 任务状态变更事件 |
| 事件 | `outbox_events` | 事务性 Outbox (BIGSERIAL) |
| 计费 | `credit_accounts` | 积分账户 (CHECK >= 0) |
| 计费 | `credit_ledger` | 积分流水 |
| 审核 | `moderation_records` | 内容审核记录 |
| 审计 | `audit_logs` | 操作审计日志 |

**Seed 数据**: admin 用户 (10000 积分) + demo 用户 (500 积分) + 示例项目

## 快速开始

### 前置条件

- Go 1.26+
- Docker & Docker Compose
- (可选) goose, golangci-lint

### 启动所有服务

```bash
# 复制环境变量
cp .env.example .env

# 启动所有服务 (PostgreSQL, Redis, Kafka, MinIO, API, Worker)
make dev

# 或只启动基础设施，本地运行 API
make dev-infra
make migrate
go run ./cmd/api
```

### 验证

```bash
# 健康检查
curl http://localhost:18080/healthz

# 深度就绪检查
curl http://localhost:18080/readyz
```

### 常用命令

```bash
make help          # 查看所有命令
make dev           # 启动所有服务
make stop          # 停止所有服务
make test          # 运行测试
make lint          # 代码检查
make migrate       # 数据库迁移
make build         # 构建二进制
```

## API 端点

| Method | Path | Auth | 描述 |
|---|---|---|---|
| GET | `/healthz` | — | 存活探针 |
| GET | `/readyz` | — | 就绪探针 (检查 DB + Redis) |
| POST | `/api/auth/register` | — | 注册新用户 |
| POST | `/api/auth/login` | — | 登录 (返回 JWT access + refresh token) |
| POST | `/api/auth/refresh` | — | 刷新 token |
| GET | `/api/me` | ✅ | 当前用户信息 |
| POST | `/api/projects` | ✅ | 创建项目 |
| GET | `/api/projects` | ✅ | 项目列表 (分页) |
| GET | `/api/projects/:id` | ✅ | 项目详情 |
| PATCH | `/api/projects/:id` | ✅ | 更新项目 |
| POST | `/api/projects/:id/workflows` | ✅ | 创建工作流 |
| GET | `/api/workflows/:id` | ✅ | 获取工作流 (含节点和边) |
| PUT | `/api/workflows/:id` | ✅ | 更新工作流 (DAG 校验) |
| POST | `/api/workflows/:id/validate` | ✅ | 验证 DAG + 拓扑排序 |
| POST | `/api/workflows/:id/snapshot` | ✅ | 创建版本快照 |
| POST | `/api/works/:id/publish` | ✅ | 发布作品 |
| GET | `/api/feed` | — | 公开动态流 (分页) |

## 端口映射

| 服务 | 端口 |
|---|---|
| API | `18080` |
| PostgreSQL | `15432` |
| Redis | `16379` |
| Kafka | `19092` |
| MinIO API | `19000` |
| MinIO Console | `19001` |

## License

MIT
