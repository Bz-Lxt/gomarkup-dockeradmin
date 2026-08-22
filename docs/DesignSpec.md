# DockerAdmin — 设计规范（Design Spec）

> 版本：v1.0｜日期：2026-08-20｜美学方向：**Mission Control（任务控制中心）**
> 工业/utilitarian 底色 + 磷光信号色。拒绝通用 AI 审美（无 Inter、无紫渐变、无模板化布局）。

## 1. 设计概念

运维工具的灵感来自发射场控制席：深墨色舱底、工程网格底纹、等宽数字读数、信号状态灯。
**记忆点**：所有关键指标以大型等宽数字呈现，配磷光青（phosphor cyan）辉光；状态变化时有"信号灯"式脉冲反馈。

## 2. 色板（CSS Variables 驱动）

| Token | 值 | 用途 |
|---|---|---|
| `--ink-0` | `#070B11` | 页面底色（舱底） |
| `--ink-1` | `#0D141D` | 卡片/面板 |
| `--ink-2` | `#141D2A` | 悬浮/ hover 面 |
| `--line` | `#1E2A3A` | 分隔线/边框 |
| `--signal` | `#22D3EE` | 主强调（磷光青）：主指标、激活态、主按钮 |
| `--signal-dim` | `#0E7490` | 强调弱态/边框辉光 |
| `--ok` | `#34D399` | 正常/running |
| `--warn` | `#FBBF24` | 警告/接近阈值 |
| `--danger` | `#F87171` | 危险/超阈值/stopped |
| `--text-hi` | `#E8EEF5` | 主文本 |
| `--text-lo` | `#7C8DA6` | 次文本/标签 |

背景纹理：CSS 生成的 40px 工程网格线（`--line` 5% 透明度）+ 顶部径向辉光（signal 3%）。禁止纯色死黑。

## 3. 字体

| 角色 | 字体 | 用途 |
|---|---|---|
| Display | **Chakra Petch** 600/700 | 页标题、卡片标题、品牌标识（科技感字形） |
| Body | **IBM Plex Sans** 400/500 | 正文、表格、表单 |
| Mono | **IBM Plex Mono** 500/600 | 所有数值读数、日志、代码、时间戳 |

字体经 `@fontsource/*` 打包进镜像，无运行时外部依赖。数值统一 `font-variant-numeric: tabular-nums` 防跳动。

## 4. 组件规范

- **MetricCard**：标签（小字大写 tracking-widest）+ 大号 Mono 读数（带 signal 辉光）+ 迷你趋势线 + 阈值状态边条（ok/warn/danger 三段）。
- **LineChart**：ECharts 暗色主题；轴线 `--line`，面积渐变 signal→transparent；无网格硬线，仅虚线。
- **StatusBadge**：圆点（脉冲动画）+ 文字；running=ok、exited=danger、paused=warn、degraded=warn。
- **Modal/ConfirmModal**：自定义组件（**禁用原生 alert/confirm**）；遮罩 blur；危险操作按钮 danger 色 + 二次确认文案。
- **Toast**：右上角滑入；× 手动关闭 + 5s 自动消失；success/error/info 三型。
- **EmptyState**：Mono 图标字符 + 一句指引文案（禁止空白区域）。
- **Skeleton**：加载态脉冲骨架（卡片/表格行）。

## 5. 布局与响应式

- 桌面：左侧 240px 导航栏（Logo + 菜单 + 底部健康状态灯）+ 右侧内容区（`w-full`，**禁止 max-w 限宽**，仅 Modal 例外）。
- **≤768px**：侧边栏收起为顶栏 + 底部 Tab 导航；卡片单列；表格横向滚动。
- **≤480px**：读数字号降一档；图表高度压缩；表单单列。
- 间距体系：4 的倍数（4/8/12/16/24/32）；卡片圆角 8px，边框 1px `--line`。

## 6. 动效

- 页面加载：卡片交错上浮（stagger 60ms，`fadeSlideUp` 0.4s）。
- 数值刷新：200ms 颜色闪动（值上升→signal，下降→text-lo）。
- 状态灯：`pulse` 2s 无限循环。
- 图表：ECharts 默认平滑动画，SSE 数据追加不重绘全图。

## 7. 表单与反馈（client.md 合规）

- 所有表单字段：标签 + `*` 必填标注 + 字段下方红色行内错误文本；保存前统一校验入口，失败时 Toast 汇总。
- 数值字段在 schema 声明 min/max（如阈值 0-100、duration ≥ 0），校验读 schema 不硬编码。
- `select` 全局重置：`appearance:none` + 12×8 SVG 折线箭头（`#999`）+ `padding-right:36px`。
- 日期时间展示统一 `yyyy-MM-dd HH:mm:ss`（GMT+8）。
- 所有按钮必须有真实功能；危险操作（停止/删除）必经 ConfirmModal。
