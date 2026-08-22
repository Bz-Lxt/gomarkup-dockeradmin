# API 契约验证记录（Phase 3 Contract Gate）

> 验证时间：2026-08-20 14:1x (GMT+8)｜验证方式：对本机 Docker daemon 发起真实调用（curl --unix-socket）

## Provider 1: Docker Engine API（本地 daemon）

| 项 | 结果 |
|---|---|
| 端点 | `unix://$HOME/.docker/run/docker.sock`（macOS Docker Desktop 4.80.0）；**注意**：本机无 `/var/run/docker.sock` 符号链接 |
| 认证 | 无（unix socket 文件权限即认证） |
| Engine / API | 29.6.1 / **1.55**（MinAPIVersion 1.40）→ SDK 必须开启版本协商（`WithAPIVersionNegotiation`） |
| 平台 | linux/arm64（Docker Desktop VM），NCPU=10，MemTotal≈10.4GB（VM 口径，印证需求 §3 平台限制） |

### 已验证端点（真实调用）

1. `GET /version` → `{Version, ApiVersion, Os, Arch, Platform.Name}` ✅
2. `GET /containers/json?all=true` → 数组，元素含 `Id, Names(["/name"]), Image, State, Status, Ports[], Created(unix秒), Labels` ✅（本机 7 个容器）
3. `GET /containers/{id}/stats?stream=false` → 单次快照，含：
   - `cpu_stats.cpu_usage.total_usage` (ns)、`cpu_stats.system_cpu_usage`、`cpu_stats.online_cpus`、`precpu_stats`（首帧可能为零值，需守卫除零）
   - `memory_stats.usage` / `memory_stats.limit`
   - `networks.{iface}.{rx_bytes,tx_bytes}`（累计值，速率需自行求差分）✅
   - CPU% 公式：`(Δtotal_usage / Δsystem_cpu_usage) × online_cpus × 100`
4. `GET /containers/{id}/logs?stdout=true&stderr=true&tail=N` → **多路复用二进制流，8 字节帧头**（`[stream_type,0,0,0,len×4]`），非 TTY 容器必须经 `stdcopy.StdCopy` 解复用 ✅（实测帧头 `01 00 00 00 00 00 00 57`）
5. 错误格式：`{"message": "..."}` + 语义化 HTTP 状态码（404 No such container 等）✅

### 派生决策

- **双 socket 挂载**（compose）：`/var/run/docker.sock`（Linux 惯例）+ `$HOME/.docker/run/docker.sock`（macOS Docker Desktop 惯例）→ 容器内 `/run/host-docker.sock`，均 `required:false`；后端按 `DOCKER_HOST` → `/var/run/docker.sock` → `/run/host-docker.sock` 顺序探测。保证两端 `docker compose up` 零手工配置。
- 容器操作（start/stop/restart）POST 端点幂等性由 daemon 保证；304（已处于目标状态）视为成功。

## Provider 2: Webhook 通知

- 无外部 provider：交付内置 Mock 接收端 `POST /api/mock/webhook`（契约自定义，见 `docs/API.md`）。
- 真实 Webhook 为用户自备 URL（通用 HTTP POST JSON），无预置契约可验证 → 标记 **UNVERIFIED（无用户 URL）**，实现按通用规范：5s 超时、仅对网络错误/5xx 重试 1 次（窄重试）、4xx 不重试。
