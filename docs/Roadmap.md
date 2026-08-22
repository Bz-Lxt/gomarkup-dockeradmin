# DockerAdmin — 开发路线图

> 版本：v1.0｜日期：2026-08-20｜依据：`docs/Requirements.md` v1.0（冻结）
> 规模判定：< 10k LoC，单期交付（无强制 MVP/V1/V2 分期），任务级分解如下。

## 0. 架构决策记录（ADR）

| # | 决策 | 方案 | 理由 |
|---|---|---|---|
| ADR-1 | 构建顺序 | **UI-First** | 标准监控仪表盘，UI 结构不从数据模型推导，API 契约已冻结 |
| ADR-2 | 交付形态 | 单容器：多阶段 Dockerfile（Node 构建前端 → Go embed 嵌入 → Alpine 运行） | 满足 `docker compose up` 一键交付；避免 COPY 宿主机 dist 陷阱（devops 记忆） |
| ADR-3 | 宿主指标采集 | 挂载 `/proc`→`/host/proc`、`/`→`/rootfs`（均 `ro` + `required:false`），代码运行时探测存在性 | Linux 得真实宿主指标；macOS 挂载自动跳过，降级为 VM 指标（需求 §3） |
| ADR-4 | 容器采集 | `/var/run/docker.sock` + 官方 Docker SDK；socket 缺失时优雅降级 | 需求 F2.4 |
| ADR-5 | 实时推送 | SSE（非 WebSocket） | 单向推送场景足够，实现简单、自动重连 |
| ADR-6 | 历史存储 | 内存环形缓冲（默认 1h 窗口） | 需求 F1.2；无 TSDB 依赖 |
| ADR-7 | 规则持久化 | JSON 文件（`/data` 命名卷）+ 内存索引，热更新 | 需求 F4.2；重启不丢规则 |
| ADR-8 | 运行用户 | 容器内 root（docker.sock GID 跨宿主不可移植，与 cAdvisor 同策略），辅以 `no-new-privileges` + `cap_drop:ALL` | 安全与可移植性权衡，README 明示 |
| ADR-9 | 端口 | Dev 随机端口 **19217**（10000-60000 区间）；/deploy 阶段归一 8081+ | devops 记忆规则 |

## 1. API 契约（冻结，UI 先行依据）

统一信封：成功 `{data: ...}`；失败 `{error: {code, message, details?}}`。时间戳 ISO 8601（GMT+8）。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/health` | 健康检查 `{status, version, docker: connected\|degraded, uptime}` |
| GET | `/api/metrics/current` | 当前系统指标快照 |
| GET | `/api/metrics/history?minutes=30` | 历史窗口（环形缓冲） |
| GET | `/api/stream/metrics` | SSE，每采集周期推送快照 |
| GET | `/api/containers` | 容器列表（含实时 CPU%/MEM） |
| GET | `/api/containers/:id` | 单容器详情 |
| POST | `/api/containers/:id/start` `/stop` `/restart` | 容器操作，204 |
| GET | `/api/containers/:id/logs?tail=100` | 最近日志行 |
| GET | `/api/stream/containers` | SSE，容器状态流 |
| GET/POST | `/api/alert-rules` | 规则列表 / 创建（201 + Location） |
| PUT/DELETE | `/api/alert-rules/:id` | 更新 / 删除（204） |
| GET | `/api/alert-events?limit=50` | 触发记录 |
| POST | `/api/mock/webhook` | Mock Webhook 接收端 |
| GET | `/api/mock/webhook/receipts` | Mock 接收记录 |

**指标快照模型**：`{ts, cpu:{percent, per_core[]}, mem:{total,used,percent}, disk:[{mount,total,used,percent}], net:[{iface,rx_bps,tx_bps}], load:[1,5,15], procs}`

**告警规则模型**：`{id, name, metric, target, op, threshold, duration_sec, cooldown_sec, enabled, webhook_url, notify_recovery, created_at, updated_at}`

## 2. 任务分解

### PHASE 2 — UI Agent（先行）
- [ ] T1 `frontend-admin` 脚手架：Vue3 + Vite + Tailwind + vue-router + ECharts
- [ ] T2 `docs/DesignSpec.md` 设计规范（色板/字体/组件/断点）
- [ ] T3 布局与组件库：AppShell（侧边栏+顶栏）、MetricCard、LineChart、StatusBadge、Toast、ConfirmModal、Modal
- [ ] T4 Dashboard 页：四指标卡片 + 实时折线图（对接 SSE 契约，mock 数据先行）
- [ ] T5 Containers 页：表格 + 操作（二次确认）+ 详情抽屉（指标+日志）
- [ ] T6 Alerts 页：规则 CRUD（表单校验）+ 触发记录 + Mock Webhook 接收展示

### PHASE 3 — Logic Agent
- [x] T7 `backend` 骨架：config/logger/model + Gin 路由 + 统一错误信封 + 优雅退出
- [x] T8 采集器：gopsutil 系统指标 + 环形缓冲 + 宿主路径探测（/host/proc、/rootfs）
- [x] T9 Docker 模块：SDK 列表/详情/操作/日志/统计 + 降级逻辑（docker/docker v28.5.2 monorepo，规避 moby 拆分模块 toolchain 下载）
- [x] T10 告警引擎：规则 CRUD + 持续期/冷却期判定 + Webhook 通知器（5s 超时/1 次重试）+ Mock 接收端 + JSON 持久化
- [x] T11 SSE 流 + 前端联调（embed 静态资源）+ `docs/API.md`
- [x] T12 根级多阶段 Dockerfile + docker-compose.yml 联调通过（镜像构建 ✓，compose up ✓，健康检查 ✓）

### PHASE 4 — QA
- [x] T13 Go 单元测试：环形缓冲、告警引擎判定、API handler（httptest）— 4 包全绿
- [x] T14 `tests/api_smoke.py`：14/14 通过（Compose 内执行，成本 ¥0），见 `docs/QA_Record.md` Round 1

### PHASE 5 — Audit
- [x] T15 按 audit-rules.md 审核 → `docs/AuditReport.md`（Iteration 1：PASS）→ 知识收割（4 条规则入知识库）

## 3. 目录结构

```
DockerAdmin/
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/{config,logger,model,collector,dockermon,alert,api}/
│   ├── web/                    # 前端构建产物（embed 目标，构建期生成）
│   └── go.mod
├── frontend-admin/             # Vue3 + Vite + Tailwind + ECharts
├── tests/api_smoke.py
├── docs/{Requirements,Roadmap,API,DesignSpec,QA_Record,AuditReport}.md
├── Dockerfile                  # 根级多阶段（frontend build → go build → alpine）
├── docker-compose.yml          # Dev：随机端口 19217
└── .dockerignore / .gitignore
```
