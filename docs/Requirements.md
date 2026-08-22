# DockerAdmin — 需求规格说明书

> 版本：v1.0（冻结）｜日期：2026-08-20｜时区：GMT+8
> 本文档定义 **WHAT**（做什么），`docs/Roadmap.md` 定义 **WHEN**（何时做）。
> 原始 Prompt 存档：`docs/.meta/original_prompt.md`

---

## 1. 项目概述

**DockerAdmin** 是一个基于 Go 语言的本地容器/进程监控与管理工具，通过 Web UI 实时展示系统指标（CPU、内存、磁盘、网络）与 Docker 容器状态，并支持可配置的告警规则（阈值触发 Webhook 通知）。

**交付形态**：`docker compose up --build -d` 一键启动，浏览器经 `localhost` 访问 Web UI（满足 Docker Delivery Standard）。

## 2. 矛盾检测与裁决记录

| # | 原始表述 | 矛盾点 | 裁决 |
|---|---|---|---|
| 1 | 「Web UI **或** TUI（Bubbletea）」 | TUI 运行于终端，无法经 localhost 浏览器访问，违反 Docker Delivery Standard（Redline 1） | **主交付 = Web UI（Gin）**；TUI 降级为 V2 可选增强，不进入 MVP |

## 3. 平台限制声明（已知约束，非缺陷）

| 平台 | 系统指标采集 | 容器状态采集 |
|---|---|---|
| Linux | ✅ 真实宿主指标（挂载宿主 `/proc`、`/sys`） | ✅ docker.sock |
| macOS / Windows | ⚠️ 采集到的是 Docker Desktop Linux VM 的指标（与 cAdvisor/Netdata 行为一致） | ✅ docker.sock（全平台一致） |

> 该限制将写入 `README.md` 使用说明，避免验收误解。

## 4. 功能需求

### F1 系统指标采集（核心）
- F1.1 实时采集 CPU 使用率（总体 + 每核）、内存（用量/总量/百分比）、磁盘（各挂载点用量/IO）、网络（各接口收发速率）。
- F1.2 采集间隔可配置，默认 **2s**；历史数据存内存环形缓冲，默认保留 **1h**（可配）。
- F1.3 采集库：`gopsutil/v3`；容器内通过挂载宿主 `/proc`、`/sys` 实现。

### F2 Docker 容器监控与管理
- F2.1 容器列表：名称、镜像、状态、CPU%、内存用量/限制、网络 IO、运行时长。
- F2.2 容器操作：启动 / 停止 / 重启（带二次确认）。
- F2.3 容器详情：单容器实时指标流 + 最近 100 行日志查看。
- F2.4 经 `/var/run/docker.sock` 使用官方 Docker SDK；socket 不可用时优雅降级（隐藏容器模块，仅展示系统指标，UI 明示降级状态）。

### F3 Web UI（frontend-admin）
- F3.1 总览仪表盘：系统四指标实时卡片 + 折线图（ECharts），SSE 推送，**端到端刷新延迟 ≤ 3s**。
- F3.2 容器管理页：表格 + 状态徽标 + 操作按钮 + 详情抽屉（实时指标 + 日志）。
- F3.3 告警规则页：规则 CRUD、启停开关、最近触发记录（最近 50 条）。
- F3.4 设计标准：Vue3 + Vite + TailwindCSS + ECharts；深浅色适配、响应式布局、加载/空/错误三态齐全（Aesthetic Excellence 红线）。

### F4 告警引擎
- F4.1 规则模型：`{指标类型, 运算符(>/>=/</<=), 阈值, 持续时间(s), 冷却期(s), 启用开关, Webhook URL}`；示例：CPU > 80% 持续 30s。
- F4.2 规则 **热更新**：CRUD 即时生效，无需重启。
- F4.3 触发逻辑：持续超阈值达 `持续时间` 才触发；触发后进入 `冷却期` 防告警风暴；恢复时可发恢复通知（可配）。
- F4.4 Webhook 通知：HTTP POST JSON（含规则名、当前值、阈值、时间戳 GMT+8）；超时 5s，失败重试 1 次。
- F4.5 **Mock Webhook**：内置 `/api/mock/webhook` 接收端 + UI 展示最近接收记录，用于无外部服务时演示；真实/Mock 切换方式写入 `README.md` §7（Mock Legitimacy Standard）。

### F5 配置与 API
- F5.1 配置：环境变量 + 默认值（采集间隔、保留窗口、端口、时区 TZ=Asia/Shanghai）。
- F5.2 REST API：指标查询（当前值 + 历史窗口）、容器列表/操作/日志、告警规则 CRUD、触发记录、健康检查 `/api/health`。
- F5.3 实时通道：SSE `/api/stream/metrics`、`/api/stream/containers`。
- F5.4 API 文档：`docs/API.md`，含每端点请求/响应示例、参数类型、错误码表（全局记忆规则）。

## 5. 非功能需求

- **N1 性能**：空闲时采集进程自身 CPU 开销 < 1%；内存占用 < 100MB。
- **N2 健壮性**：外部数据（Webhook 响应、Docker API 返回、配置文件）反序列化必须做结构完整性校验（字段存在性、类型、边界值）。
- **N3 日志**：统一 Logger（level 可控），生产模式屏蔽 debug；禁止散落 `fmt.Println`/`console.log`。
- **N4 时区**：全链路 GMT+8（容器 `TZ=Asia/Shanghai`）。
- **N5 跨平台**：镜像支持 ARM64 + AMD64；多阶段构建，最终镜像基于 alpine/distroless。
- **N6 安全**：容器操作仅限本工具启动的 docker.sock；Webhook URL 校验（禁内网保留段以外的非法 scheme，仅允许 http/https）；API 无鉴权但默认仅监听容器网络，由 compose 映射端口。

## 6. 验收基线（可测量）

| # | 指标 | 基线 |
|---|---|---|
| A1 | 启动 | `docker compose up --build -d` 后 ≤ 60s 可访问 Web UI |
| A2 | 实时性 | 指标变化到 UI 呈现 ≤ 3s |
| A3 | 告警时延 | 条件满足到 Webhook 发出 ≤ 5s（含持续时间判定） |
| A4 | 告警准确性 | 构造 CPU/内存压测，规则触发率 100%，冷却期内不重复触发 |
| A5 | 降级 | 移除 docker.sock 挂载后系统指标模块仍正常工作，UI 明示降级 |
| A6 | 测试 | Go 单元测试覆盖采集器、告警引擎、API handler；API Smoke 测试全绿（Mock 模式，成本 ¥0） |
| A7 | 资源 | 空闲 10 分钟，容器 CPU 均值 < 1%，内存 < 100MB |

## 7. 范围边界

**MVP（本次交付）**：F1 系统指标、F2 容器监控与管理、F3 Web UI、F4 告警引擎、F5 配置与 API。
**明确不做（V2+ 候选）**：TUI（Bubbletea）、多主机/Agent 模式、Prometheus 导出、用户鉴权体系、告警渠道扩展（邮件/钉钉/飞书）、持久化存储（TSDB）。
