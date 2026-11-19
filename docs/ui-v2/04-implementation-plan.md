# 04 · 实施计划

> 本文只规划，不包含代码。阶段按依赖顺序排列，未完成的页面统一用占位，不阻塞下一阶段。

## 1. 阶段总览

| 阶段 | 主题 | 主要产出 | 完成标准 |
| --- | --- | --- | --- |
| 0 | 设计对齐 | token 对照、页面清单、图标缺口、路由文档 | 颜色/尺寸/页面/路由都有出处 |
| 1 | 路由与壳层 | 单 Router、双 Layout、Hub 导航、Workspaces 列表/创建 | 主流程可用，布局边界成立 |
| 2 | 设计系统 | 新 token、v2 原子组件、TitleBar/StatusBar/Hub 重绘 | Hub 端视觉对齐，Light/Dark 同步 |
| 3 | 工作台骨架 | 统一资源侧栏、标签模型、请求标签抽取 | 标签语义与状态持久化通过测试 |
| 4 | 请求详情 | Params/Auth/Headers/Body/Scripts/Settings + Response | 设计稿 4–22 全部覆盖 |
| 5 | 管理端其余页 | Home、Inventory、Catalog、Network、Agent、Integrations、Reports、Settings 弹窗 | 所有导航可达，无死链 |
| 6 | 清理加固 | 删除旧路由/组件/图标栏、补测试与文档 | 构建、测试、手测矩阵通过 |

## 2. Phase 0 · 设计对齐

任务：

1. 从 Pixso 帧中提取 token：颜色、字号、行高、圆角、控件尺寸、区域尺寸。
2. 产出「旧 token → 新 token」对照表，标出会破坏旧页面的字段。
3. 产出「设计稿图标 → Lucide 名称」清单，更新 `scripts/icons.txt`。
4. 产出页面清单与路由映射（见 03）。
5. 确认首版交付范围与占位数据。

交付物：

- `docs/ui-v2/02-design-system.md`（本文件已初版）
- `docs/ui-v2/03-page-inventory.md`
- 图标映射表（可并入 02 的附录）
- token 对照表（建议新增 `docs/ui-v2/token-migration.md`）

门禁：token 值有设计出处；没有「大概这个颜色」的项。

## 3. Phase 1 · 路由与壳层

### 3.1 任务

| # | 任务 | 文件/模块 | 说明 |
| --- | --- | --- | --- |
| 1 | 路由常量与构造器 | `internal/routes` | `/hub/*`、`/w/{wsID}`、`WorkbenchOf` / `WorkbenchTabOf` / `WorkbenchID` |
| 2 | AppShell 拆分 | `internal/app` | TitleBar + Router + StatusBar；删除 IconRail 调用 |
| 3 | HubLayout | `internal/app/layout_hub.go` | 团队侧栏 + `r.View` 子路由 |
| 4 | Hub 页面 | `features/hub` | Home 骨架、Workspaces 列表、Create 页、占位页 |
| 5 | WorkbenchLayout | `internal/app/layout_workbench.go` | 解析 `wsID`、校验工作区、按 `?tab=` 分发 |
| 6 | 现有页面接入 | `features/collection`、`environment`、`history` | 增加 `ViewWorkspace(wsID)`，内部跳转改新路径 |
| 7 | TitleBar / StatusBar | `internal/app/shell.go` | 工作区名从 URL 取；Workspace 下拉按 Push/Replace 语义跳转 |
| 8 | 测试 | `main_test.go` + 新增 router 测试 | 旧的「点击图标栏历史」必须重写 |

### 3.2 路由

```
/hub/{page...}     → HubLayout
/w/{wsID}          → WorkbenchLayout，标签从 ?tab= 读取
```

Phase 1 的 `?tab=` 支持：

- 空 / `collections` → 现有集合工作台
- `environments` → 现有环境页
- `history` → 现有历史页
- `docs` / `mock` → 占位页

Phase 3 再把 Environments / History 收进统一标签栏。

### 3.3 壳层

- TitleBar 常驻；团队上下文显示写死的 `wang's Team`，工作区上下文显示 URL 对应的工作区名。
- TitleBar 的 Workspace 下拉：
  - 从 Hub 选工作区：`Push("/w/{id}")`
  - 从 Workbench 切换：`Replace("/w/{id}")`
  - Create：`Push("/hub/workspaces/new")`
  - View all：`Push("/hub/workspaces")`
- StatusBar 常驻；工作区名从 URL 解析，不读 `ActiveWorkspace()`。
- Settings 入口先打开现有设置页的临时弹窗；Phase 5 换成正式 Dialog。

### 3.4 守卫

- `wsID` 不存在：兜底页 + 「返回工作区列表」，不自动重定向。
- 删除当前工作区：动作方 `Replace("/hub/workspaces")`。
- 创建成功：`Replace("/w/{newID}")`。
- `store == nil`：Hub 显示「存储未就绪」，不崩溃。

### 3.5 验收

- [ ] 启动 `/hub/home`，TitleBar / StatusBar 正常。
- [ ] Hub 侧栏 9 个入口都能点击，未实现页面显示占位。
- [ ] Workspaces 列表显示真实工作区；搜索可过滤；行点击进入工作台。
- [ ] Create 页可创建工作区，成功后进入新工作台。
- [ ] 工作台 `/w/{id}` 显示集合工作台；`?tab=environments/history` 可切。
- [ ] Alt+← 从工作台回到来源页；`Replace` 操作不增加历史条目。
- [ ] 无效 `wsID` 显示兜底页。
- [ ] `go build ./...`、`go test ./...`、`go tool mygo vet .` 通过。

## 4. Phase 2 · 设计系统

任务：

1. 重构 `components/theme`：新 token、语义命名、Light/Dark。
2. 新建 `components/v2`：Button、IconButton、Input、Search、Select、Checkbox、Segment、Tab、Breadcrumb、Table、TreeRow、Badge、Dialog、Popover、Tooltip、EmptyState。
3. 先重绘 TitleBar、StatusBar、Hub 侧栏、Workspaces 列表、Create 页。
4. 图标按映射表补齐；字体用系统栈。
5. 建立「禁止硬编码颜色」的检查清单（代码评审项）。

验收：

- [ ] Hub 端 1280×800 与设计稿 28/29 一致（允许字体近似）。
- [ ] Light / Dark 都有截图对照。
- [ ] 新组件不引用 `ui.Hex`；页面不直接写颜色。
- [ ] 旧 `components/v1` 仍可编译，但不再新增使用。

## 5. Phase 3 · 工作台骨架

任务：

1. 统一资源侧栏：Collections / Environments / Specs / Mocks / Datasets / Flows / Toolsets / Documents。
2. 标签模型：`WorkbenchState`、`Tab`、每工作区缓存。
3. 请求标签从 `features/collection` 内部状态抽到共享 tab host。
4. 标签条 UI：选中/hover/关闭/溢出。
5. 深链：`?tab=request:{id}` 打开对应请求；资源不存在回 Overview。
6. 关闭标签：切相邻，最后回 Overview；`Replace` 更新 URL。

验收：

- [ ] 开/关/切标签行为正确；右键/中键关闭（如实现）。
- [ ] 切换 Hub ↔ Workbench 后标签列表与激活标签不丢。
- [ ] 切换工作区后各自标签互不污染。
- [ ] 深链可恢复；无效 tab 安全回退。
- [ ] Alt+← 不按标签回退。
- [ ] 大树滚动、搜索过滤不卡顿（至少 500 节点手测）。

## 6. Phase 4 · 请求详情

任务：

- 请求区：Docs / Params / Authorization / Headers / Body / Scripts / Settings。
- Body：None / FormData / Urlencoded / Binary / Raw / Bulk Edit，以及格式下拉、类型下拉。
- 响应区：Body / Cookies / Headers / Test Results；状态码、耗时、大小。
- 方法下拉、代码片段下拉、保存/发送的分割按钮。
- 空态、错误态、加载态。

验收：

- [ ] 设计稿 4–22 号画面全部覆盖。
- [ ] 双主题、最小窗口 960×600 可用。
- [ ] 请求发送、环境变量、脚本执行等现有能力不回归。

## 7. Phase 5 · 管理端其余页与弹窗

任务：

- Home 骨架、Inventory、Catalog、Private/Public Network、Agent Context、Integrations、Reports。
- Settings Dialog（Appearance / Data mode / Shortcuts / Language）。
- Workspace 下拉的 Hover / Click / List Hover 三种状态。
- 通知中心入口与空态。
- 所有占位页统一空态组件。

验收：

- [ ] 侧栏所有入口可达，无死链。
- [ ] Settings 弹窗在 Hub / Workbench 都能打开，关闭后页面状态不丢。
- [ ] 空态、错误态、加载态一致。

## 8. Phase 6 · 清理与加固

任务：

- 删除旧路由、`iconRail`、`features/demo`、`components/v1` 中不再使用的组件。
- 统一 locales：新增 Hub / Workbench / 空态文案，清理无用 key。
- 快捷键：保留 Alt+←/→、Ctrl+1..n；新增标签切换/关闭快捷键时先定规范。
- 无障碍：焦点顺序、图标按钮 label、对比度、ReduceMotion。
- 性能：树虚拟化、响应体折叠/虚拟滚动、避免每帧重复计算。
- 测试：路由、标签、守卫、主题、最小窗口。

验收：

- [ ] `CGO_ENABLED=0 go test ./...`、`go tool mygo vet .` 通过。
- [ ] Windows / macOS 手测矩阵通过。
- [ ] 旧文档/旧代码没有残留引用。

## 9. 测试矩阵

### 9.1 路由与布局（`ui.NewTester`）

1. Hub 内切换页面：侧栏按钮计数不重置。
2. Hub → Workbench → Alt+←：回到来源 Hub 页，侧栏状态保持。
3. Workbench 内 `Replace("?tab=environments")`：`CanGoBack` 不变，页面状态保持。
4. 标题栏切换工作区：`Replace`，历史条数不增加。
5. 无效 `wsID`：显示兜底页，不 panic。
6. 删除当前工作区：路由回到 `/hub/workspaces`。
7. Settings 打开/关闭：底层页面滚动/输入不丢。

### 9.2 视觉

| 矩阵 | 值 |
| --- | --- |
| 尺寸 | 1280×800（设计基准）、960×600（最小） |
| 主题 | Light、Dark |
| 平台 | Windows、macOS |
| 页面 | Hub Home / Workspaces / Create / Workbench 集合 / 请求详情 / Settings |

### 9.3 命令

```
go build ./...
go test ./...
go tool mygo vet .
CGO_ENABLED=0 go test ./...
```

## 10. 风险与缓解

| 风险 | 影响 | 缓解 |
| --- | --- | --- |
| 60+ 画面一次铺太大 | 长期不可交付 | 按阶段门禁；未实现页面统一占位 |
| 标签模型抽取牵动 collection 状态 | 请求编辑回归 | Phase 3 前先定 `WorkbenchState` 与测试；不碰发送引擎 |
| 设计 token 有未标注值 | 视觉反复 | Phase 0 逐项核对；文档标「待核对」 |
| 单 Router 转场只能全局统一 | 动效受限 | Phase 1 统一 Fade；Phase 6 再评估 |
| 图标/字体与设计稿不完全一致 | 观感差异 | Lucide 近似 + 系统字体；不改设计稿 |
| 现有测试依赖 IconRail | 假红/漏测 | Phase 1 同步重写 `main_test.go` |
| Team/权限占位被误当真实功能 | 需求蔓延 | mock 数据集中在 `features/hub`，不写 store |

## 11. 待确认

1. 首版内部演示范围是否就是「Hub + Workspaces + 集合工作台 + 请求详情」。
2. Dark 是否在 Phase 2 同步验收，还是允许 Phase 2 只交 Light、Phase 3 补 Dark。
3. Hub Home 的内容结构是否等设计补充，还是先用「最近工作区 + 快捷入口」骨架。
4. 管理端 7 个占位页是否接受统一空态，还是需要每个页面单独文案。
