# Nexo

Nexo 是一个面向数字内容订阅与权益业务的 Go + React 全栈项目。

## 当前完成范围

- PostgreSQL 用户表和自动迁移
- 注册、登录、JWT 鉴权
- 当前用户信息和资料更新
- USER / OPERATOR / ADMIN 角色
- ADMIN 用户列表、角色和状态管理接口
- Content / Episode 内容管理和发布
- 会员计划、订阅和固定期限会员权益
- 订单、钱包虚拟币、钱包行锁扣款
- Outbox + RabbitMQ 异步权益履约和重复消费幂等
- Entitlement 统一访问权限校验
- Redis 内容详情缓存
- Cloudflare R2 presigned URL 上传与播放；启动 API 必须配置 R2
- WatchProgress 观看进度和乐观锁
- React 用户端和 OPERATOR 内容运营台
- Docker Compose + PostgreSQL + Redis + RabbitMQ + pgAdmin + Nginx

## 本地启动

```bash
cp .env.example .env
docker compose up -d --build
```

访问：

- Web：<http://localhost>
- API 存活检查：<http://localhost/health>
- API 就绪检查：<http://localhost/ready>
- pgAdmin：<http://localhost:5050>
- RabbitMQ 管理台：<http://localhost:15672>

pgAdmin 登录账号默认为 `admin@example.com`，密码为 `.env` 中的 `PGADMIN_DEFAULT_PASSWORD`。

pgAdmin 连接 PostgreSQL 时使用：

- Host：`postgres`
- Port：`5432`
- Database：`nexo`
- Username：`nexo`
- Password：`.env` 中的 `POSTGRES_PASSWORD`

初始化管理员：

```bash
ADMIN_EMAIL=admin@nexo.local ADMIN_PASSWORD=change-me-123 go run ./cmd/seed-admin
```

公开注册始终创建 `USER`。创建 `OPERATOR` 可以先用 ADMIN 登录后调用用户管理接口，或者直接在 pgAdmin 中执行更新。

v1 固定使用 Cloudflare R2，不提供模拟对象存储。启动 Docker Compose 前，在 `.env` 中填写：

```text
R2_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com
R2_ACCESS_KEY=<access-key>
R2_SECRET_KEY=<secret-key>
R2_BUCKET=nexo-media
```

`.env.example` 中的 R2 配置默认为空，必须填写 Cloudflare 控制台提供的 Endpoint 和密钥；Docker Compose 会拒绝空配置，API 也会在启动时校验 R2 HTTPS Endpoint 和密钥。Episode 只保存 bucket 与 object key，存储服务固定为 R2。

视频对象只在数据库中保存 `object_key`，播放时由 API 做权益校验并生成短期 presigned URL。

运营人员在内容台选择 MP4 后，浏览器通过短期 presigned PUT 地址直传 R2，Go API 不中转视频文件。v1 限制单个文件不超过 1 GiB；发布 Episode 前 API 会通过对象元数据确认文件已上传、大小合规且 Content-Type 为 `video/mp4`。PUT 地址约 15 分钟有效，播放 GET 地址约 10 分钟有效。

首次启用浏览器上传前，还需在 Cloudflare R2 对应桶的 CORS 设置中加入规则。Docker Web 页面默认使用 `http://localhost`；Vite 开发服务器使用 `http://localhost:5173`。只有确实会用到的来源才加入；本地也可按需加入 `http://127.0.0.1`。

建议规则：

| R2 CORS 项 | 值 |
| --- | --- |
| Allowed origins | 上述实际页面来源，逐个填写 |
| Allowed methods | `PUT`, `GET`, `HEAD` |
| Allowed headers | `Content-Type`, `Range` |
| Expose headers | `ETag`, `Content-Length`, `Content-Range`, `Accept-Ranges` |
| Max age | `3600` 秒 |

生产环境应将来源换成实际 HTTPS 域名，不要用 `*`。

> API 的 `ALLOWED_ORIGIN` 只配置浏览器到 Nexo API 的跨域访问；R2 桶的 CORS 是另一处独立配置，缺失时浏览器直传会被浏览器拦截，即使签名地址本身有效。

## 主要业务链路

```text
创建并发布 Content / Episode
→ 用户创建订单
→ 钱包支付或 Mock 支付回调
→ Outbox 发布 order.paid
→ RabbitMQ Entitlement Worker
→ 创建 Subscription / Entitlement
→ Episode 播放权限校验
→ 生成 R2 播放地址
```

## 后端目录结构

```text
cmd/                   服务端与初始化命令
internal/handler/      router.go 负责路由组装；user/content/order/wallet/playback/progress 按资源处理 HTTP
internal/service/      用例编排与业务校验
internal/repository/   PostgreSQL 查询及事务操作
internal/model/        跨层共享的业务实体和数据类型
                       TransactionManager 统一管理订单、钱包和权益事务
internal/health/       有超时边界的依赖就绪探测
internal/worker/       Outbox 发布与 MQ 消费
internal/{auth,cache,config,database,mq,storage}/
                       认证及基础设施适配
```

Handler 文件按业务资源与 Service/Repository 同名组织，例如 `handler/user.go`、`service/user.go`、`repository/user.go`。v1 保留共享的 `Router` 依赖容器，不再为每个资源额外创建一套几乎只转发调用的 Handler 结构体；如果某个资源的依赖或中间件明显独立，再单独拆结构体。

## 健康检查

- `GET /health` 是存活检查，只确认 HTTP 进程仍能响应，不访问外部依赖。
- `GET /ready` 是就绪检查，并发探测 PostgreSQL、Redis 和 RabbitMQ；总检查时限为 2 秒。全部通过返回 `200` 与 `ready`，任一失败或超时返回 `503` 与 `not_ready`，并标记不可用的依赖，不向客户端暴露底层错误或连接信息。
- RabbitMQ 探测检查当前 AMQP 连接和 Channel 是否仍打开；连接断开后由 AMQP 心跳机制更新状态。R2 不纳入通用就绪检查，因为它只影响媒体上传/播放，不影响其他 API 能否提供服务。

## 开发模式

后端：

```bash
go run ./cmd/server
```

前端：

```bash
cd frontend
npm install
npm run dev
```

开发前需要先启动 PostgreSQL，并设置 `DATABASE_URL`。

## 待办清单

- [x] 统一 Go 源码说明注释的语言：将 `internal/handler/router.go`、`internal/handler/health.go`、`internal/health/checker.go`、`internal/cache/redis.go` 和 `internal/mq/rabbitmq.go` 中的英文说明注释改为准确、自然的中文；保留 Go 编译指令等非说明性注释。
- [x] 精简 v1 基础设施抽象：订单、钱包和权益共用 `TransactionRunner` 与 `Transaction`，删除按业务重复定义的六个事务接口，同时保留内存测试替身；对象存储固定为 Cloudflare R2，移除运行时 `MockStorage`、存储切换配置及 Episode 的 provider 字段，缺少 R2 配置时 API 启动失败。事务、行锁、真实 R2 上传/播放和订单履约流程保持不变；模拟支付回调仍保留。
