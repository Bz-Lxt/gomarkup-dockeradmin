# DockerAdmin — QA 验证记录

> 成本安全声明：所有测试均在本地 Docker 网络内运行，无任何计费 API 调用。

## Round 1 — 2026-08-20 15:27 (GMT+8)

**环境**：macOS (Apple Silicon) + Docker Desktop 29.6.1｜镜像 `dockeradmin-app`（多阶段构建，12min 首构）
**执行方式**：`docker compose --profile test run --rm tests`（Compose 网络内，python:3.12-alpine）
**本轮成本**：¥0

### Go 单元测试（宿主机，`go test ./...`）

| 包 | 结果 | 覆盖点 |
|---|---|---|
| internal/collector | PASS | 环形缓冲：写入/窗口裁剪/最新值/并发读写 |
| internal/model | PASS | 规则校验（8 类非法字段）+ 比较器（4 种算子边界） |
| internal/alert | PASS | 引擎判定：未越限/持续期门控/冷却期/恢复通知/禁用规则/容器指标；持久化跳过脏数据 |
| internal/api | PASS | httptest：健康/指标 503 降级/规则 CRUD/422 信封/Mock Webhook/SPA 回退 |

### API 冒烟测试（Compose 内，14/14 通过，82s）

```
[PASS] test_health                                健康检查，docker=connected
[PASS] test_metrics_current_shape                 快照模型完整（cpu/mem/disk/net/load/procs，ts 带 +08:00）
[PASS] test_metrics_history_window                历史窗口返回环形缓冲数据
[PASS] test_metrics_history_param_validation      minutes=0 → 400 错误信封
[PASS] test_sse_metrics_stream                    SSE 12s 内收到 data 帧
[PASS] test_containers_list_or_degraded           19 容器，含自身 dockeradmin-app
[PASS] test_container_detail_and_logs             详情含 env_preview 脱敏；日志 tail 正常
[PASS] test_container_start_idempotent_on_running 运行中容器 start → 204 幂等
[PASS] test_container_action_not_found            不存在容器 stop → 404 not_found
[PASS] test_alert_rule_crud                       201+Location / 列表 / PUT / DELETE 全链路
[PASS] test_alert_rule_validation_error_envelope  threshold=150 → 422 + details；容器指标缺 target → 422
[PASS] test_alert_fires_and_webhook_received      必触发规则 → 引擎判定 → Webhook 200 → Mock 接收记录，全链路 < 30s
[PASS] test_mock_webhook_roundtrip                直发 marker → receipts 可查
[PASS] test_spa_fallback_and_index                / 与 /containers 均回退 index.html
```

### 观察项（非阻塞，macOS 环境特有）

- **O1 磁盘挂载噪音**：macOS 下 `/host/proc` 解析为 VM 路径，磁盘列表出现 VM 内部挂载 `/oldroot`（100%，tmpfs 残留）。Linux 宿主挂载模式下不受影响。建议：采集器过滤 `percent=100 && total<1GB` 的残留挂载或按设备前缀白名单。
- **O2 虚拟网卡噪音**：`net` 列表含 erspan0/gre0/sit0 等 9 个零流量虚拟接口。建议：过滤全零流量的内核虚拟接口（lo 除外保留）。

### 结论

**PASS**。功能需求 F1-F4 全部验证通过，Docker 降级路径（503 信封）经单元测试覆盖，Compose 一键启动实测可用。
