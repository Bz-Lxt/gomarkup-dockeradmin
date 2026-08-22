# DockerAdmin — API 接口文档

> 版本：v1.0｜Base URL：`http://localhost:19217`（Dev）｜所有时间戳为 ISO 8601（GMT+8）

## 通用约定

**成功响应**：`{"data": ...}`，HTTP 语义化状态码（200/201/204）。
**错误响应**：

```json
{
  "error": {
    "code": "validation_error",
    "message": "规则校验失败",
    "details": [{ "field": "threshold", "message": "阈值范围为 0 ~ 100" }]
  }
}
```

### 错误码表

| HTTP | code | 说明 |
|---|---|---|
| 400 | `invalid_json` / `invalid_param` | 请求体非法 / 参数非法 |
| 404 | `not_found` | 资源不存在 |
| 409 | `conflict` | 容器状态冲突 |
| 422 | `validation_error` | 字段校验失败（含 details 明细） |
| 500 | `docker_error` / `persist_error` | Docker 操作失败 / 持久化失败 |
| 503 | `docker_unavailable` / `no_data` | Docker 降级模式 / 采集器无数据 |

---

## 1. 健康检查

`GET /api/health`

```json
{ "data": { "status": "ok", "version": "1.0.0", "docker": "connected", "uptime_sec": 123, "collect_interval": "2s" } }
```

`docker` 字段：`connected` | `degraded`（无 docker.sock，容器功能不可用）。

## 2. 系统指标

`GET /api/metrics/current` — 最新快照（无数据时 503）。

`GET /api/metrics/history?minutes=30` — 历史窗口（minutes：1-720，默认 30）。

```json
{
  "data": [{
    "ts": "2026-08-20T14:30:00+08:00",
    "cpu": { "percent": 12.5, "per_core": [10.1, 15.0] },
    "mem": { "total": 10418655232, "used": 2359296000, "percent": 22.6 },
    "disk": [{ "mount": "/", "total": 5.0e11, "used": 2.5e11, "percent": 50.1 }],
    "net": [{ "iface": "eth0", "rx_bps": 1024.5, "tx_bps": 512.2 }],
    "load": [0.5, 0.4, 0.3],
    "procs": 42
  }]
}
```

`GET /api/stream/metrics` — SSE，每采集周期推送一帧（格式同上单条），心跳 `: ping`（15s）。

## 3. 容器

`GET /api/containers` — 列表（running 优先，按名称排序）：

```json
{ "data": [{ "id": "200a06c9...", "name": "web", "image": "nginx:latest", "state": "running", "status": "Up 2 hours", "cpu_percent": 1.5, "mem_used": 10485760, "mem_limit": 10418655232, "mem_percent": 0.1, "net_rx": 2736, "net_tx": 126, "created_at": "...", "uptime_sec": 7200 }] }
```

> 列表 `uptime_sec` 自创建时间起算；详情接口按 `StartedAt` 精确计算。

`GET /api/containers/:id` — 详情（额外含 `ports`、`mounts`、`env_preview`（敏感值脱敏 `****`））。

`POST /api/containers/:id/start` | `/stop` | `/restart` — 操作成功返回 **204**（已处于目标状态视为幂等成功）；容器不存在 404。

`GET /api/containers/:id/logs?tail=100` — `{ "data": { "lines": "..." } }`（tail：1-1000）。

`GET /api/stream/containers` — SSE 容器列表流；降级模式下返回 503。

## 4. 告警规则

`GET /api/alert-rules` — 列表（按创建顺序）。

`POST /api/alert-rules` — 创建，**201** + `Location: /api/alert-rules/{id}`：

```json
{
  "name": "CPU 过高", "metric": "cpu_percent", "target": "", "op": ">",
  "threshold": 80, "duration_sec": 30, "cooldown_sec": 300,
  "enabled": true, "webhook_url": "http://localhost:8080/api/mock/webhook", "notify_recovery": true
}
```

| 字段 | 类型 | 约束 |
|---|---|---|
| name | string | 必填 |
| metric | enum | `cpu_percent`/`mem_percent`/`disk_percent`/`net_rx_bps`/`net_tx_bps`/`container_cpu_percent`/`container_mem_percent` |
| target | string | 容器类指标必填（容器名或 ID） |
| op | enum | `>` `>=` `<` `<=` |
| threshold | number | 百分比类 0-100；容器 CPU 0-1e6；网络 0-1e12 |
| duration_sec | int | 0-86400，持续越限该时长才触发 |
| cooldown_sec | int | 0-86400，触发后冷却期内不重复告警 |
| webhook_url | string | 必填，http/https |
| notify_recovery | bool | 恢复时是否通知 |

`PUT /api/alert-rules/:id` — 全量更新（200）；`DELETE /api/alert-rules/:id` — 删除（204）。

`GET /api/alert-events?limit=50` — 触发记录（最新在前，limit ≤ 200）：

```json
{ "data": [{ "id": "ab12...", "rule_id": "...", "rule_name": "CPU 过高", "metric": "cpu_percent", "target": "", "value": 92.5, "threshold": 80, "op": ">", "kind": "fired", "webhook_status": 200, "webhook_error": "", "fired_at": "..." }] }
```

`kind`：`fired`（触发）| `recovered`（恢复）。

## 5. Mock Webhook

`POST /api/mock/webhook` — 接收任意 JSON 体（≤64KB），返回 `{ "data": { "received": true, "id": 1 } }`。

`GET /api/mock/webhook/receipts` — 最近 100 条接收记录（最新在前）。

**Webhook 推送载荷**（真实 Webhook 同构）：

```json
{ "rule_id": "...", "rule_name": "CPU 过高", "metric": "cpu_percent", "target": "", "op": ">", "threshold": 80, "value": 92.5, "kind": "fired", "fired_at": "2026-08-20T14:35:00+08:00" }
```

推送策略：5s 超时；仅网络错误/5xx 重试 1 次（4xx 不重试）。

## 6. 前端路由

非 `/api/` 路径一律回退到 `index.html`（SPA history 模式）；`/assets/*` 带 `immutable` 长缓存。
