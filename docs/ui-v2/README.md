# rddPostman UI v2 重构方案

> 状态：规划中，尚未开始编码。
> 设计源：[Pixso · rddPostman V1](https://pixso.cn/app/design/EMFtfhEwU9AggnqdzkFnQw)，页面 `设计V1`、`基准版 · 只读参考`、`组件`。

## 1. 目标

- 按新设计稿整体重构 UI，不复用旧页面结构与旧视觉。
- 拆成两套布局：团队级管理端 `HubLayout` 与工作区工作台 `WorkbenchLayout`。
- 用 URL 表达「当前在哪个工作区、哪个标签」，让深链、返回、窗口恢复都有确定行为。
- 重建 theme token 与基础组件，统一 Light / Dark，不再散落硬编码颜色。
- 保持现有数据能力可用：工作区、集合、请求、环境、历史、发送引擎都不重写，只换 UI 外壳与页面。

## 2. 非目标

- 不实现 Team、成员、角色、权限、云端同步的真实后端；本版只做 UI 占位，数据写死。
- 不实现 Application Inventory、API Catalog、Private / Public API Network、Agent Context、Integrations、Reports 的真实业务；先做页面骨架与空态。
- 不修改设计稿；缺失图标用 Lucide 相近图标替代。
- 不在本版引入新的数据存储后端（SQLite / 远端）。

## 3. 已确认决策

| 主题 | 决策 |
| --- | --- |
| 管理端前缀 | `/hub`（不用 `/console`，避免与调试 Console 混淆；不用 `/team`，避免以后多团队时变成历史包袱） |
| 布局 | 单 Router + 两个顶层 Layout；公共 TitleBar / StatusBar 外置 |
| 工作区上下文 | URL 中的 `wsID` 是唯一事实来源；`store.ActiveWorkspaceID` 仅作「上次打开」偏好 |
| 工作台标签 | `?tab=` 查询参数 + `Replace`，标签切换不写入历史 |
| 标签状态 | App 级 `map[wsID]*WorkbenchState` 缓存，不放进 store，不落盘 |
| Settings | 全局弹窗，不占路由 |
| 标题栏高度 | 36（内容 35 + 分割线 1），与 Postman 并排实测对齐；设计稿当初的 36 测量正确 |
| Team / 角色 / 权限 | UI 占位，数据写死，不扩 domain |
| 图标 / 字体 | Lucide 近似；系统字体栈；不改设计稿 |
| 主题 | token、组件、旧页面全部可重构；Light / Dark 同步规划 |

## 4. 文档索引

| 文档 | 内容 |
| --- | --- |
| [01-architecture.md](01-architecture.md) | Router、双 Layout、导航语义、状态归属、守卫与迁移 |
| [02-design-system.md](02-design-system.md) | 颜色 / 字体 / 间距 / 圆角 / 组件规格 / 图标策略 |
| [03-page-inventory.md](03-page-inventory.md) | 设计稿画面清单、路由映射、占位数据、设计缺口 |
| [04-implementation-plan.md](04-implementation-plan.md) | 阶段计划、Phase 1 任务拆解、测试矩阵、风险 |
| [05-token-migration.md](05-token-migration.md) | 旧 token / Metric / 组件到 v2 的迁移顺序与对照 |

## 5. 旧文档

`docs/ui-dual-layout-architecture.md` 中「工作台保留图标栏」的前提已被新设计稿推翻：新稿取消了 Icon Rail，工作台改为「统一资源侧栏 + 多标签编辑区」。该文档仅保留作历史参考，以本目录为准。
