# 03 · 页面清单与路由映射

> 帧名称、ID 来自 Pixso `设计V1` 页；`基准版 · 只读参考` 仅用于对照，不作为实现目标。

## 1. 管理端（HubLayout）

| 页面 | 路由 | 设计稿 | 本版策略 |
| --- | --- | --- | --- |
| Home | `/hub/home` | 无独立帧 | 先做团队首页骨架：最近工作区、快捷入口、空态 |
| Workspaces 列表 | `/hub/workspaces` | 28 / 30（Dark） | 真实数据 + 搜索 + 行 hover；创建者/角色/最近活动先写死 |
| Create workspace | `/hub/workspaces/new` | 29 / 31（Dark） | 名称输入 + 创建可用；类型、权限、Local folder 先做 UI 占位 |
| Application Inventory | `/hub/inventory` | 无帧，仅侧栏入口 | 占位页 + 空态 |
| API Catalog | `/hub/catalog` | 无帧 | 占位页 + 空态 |
| Private API Network | `/hub/network/private` | 无帧 | 占位页 + 空态 |
| Public API Network | `/hub/network/public` | 无帧 | 占位页 + 空态 |
| Agent Context | `/hub/agent-context` | 无帧 | 占位页 + 空态 |
| Integrations | `/hub/integrations` | 无帧 | 占位页 + 空态 |
| Reports | `/hub/reports` | 无帧 | 占位页 + 空态 |

管理端侧栏结构（按 28 号画面）：

```
[团队头像] wang's Team        [Invite]
Home
Workspaces
Application Inventory
API Catalog
────────────
Private API Network
Public API Network
────────────
Agent Context
Integrations
Reports
────────────
Settings
```

本版 Team 名称、成员数、Invite、Upgrade、通知红点均可写死；不要为它们扩 domain。

## 2. 工作台（WorkbenchLayout）

路由：`/w/{wsID}`，标签用 query 表达。

### 2.1 画面对应

| 帧 | 名称 | 目标 |
| --- | --- | --- |
| 32:73920 | 1-侧栏-集合无请求 | 默认工作台：资源侧栏 + 无打开标签 |
| 32:74490 | 2-侧栏-集合为空 | 集合为空空态 |
| 32:75050 | 3-侧栏-目录无请求 | 目录选中、右侧空态 |
| 34:443949 | 22-侧栏-搜索筛选 | 侧栏搜索 + 过滤结果 |
| 32:76288 | 4-请求-Docs | `?tab=request:{id}` → Docs |
| 32:77797 | 5-请求-Params | Params |
| 32:78341 | 6-请求-Authorization | Authorization |
| 32:78864 | 7-请求-Headers | Headers |
| 13:2 | 8-请求-Body | Body |
| 32:79408 | 9-请求-Scripts | Scripts |
| 32:79921 | 10-请求-Settings | Settings |
| 32:93967 | 11-请求体-None | Body 空态 |
| 32:94453 | 12-请求体-FormData | FormData |
| 32:94992 | 13-请求体-Urlencoded | Urlencoded |
| 32:95517 | 14-请求体-Binary | Binary |
| 34:9342 | 15-请求体-BulkEdit | Bulk Edit |
| 34:9860 | 16-请求体-Raw-格式下拉 | Raw + 格式菜单 |
| 34:10338 | 17-请求体-FormData-类型下拉 | FormData 类型菜单 |
| 32:96955 | 18-响应-Body | Response Body |
| 13:970 | 19-响应-Cookies | Response Cookies |
| 13:503 | 20-响应-Headers | Response Headers |
| 32:97430 | 21-响应-TestResults | Response Test Results |

### 2.2 标签模型

| 标签 | URL | 说明 |
| --- | --- | --- |
| Overview | `?tab=overview` | 工作区概览；Phase 3 增加 |
| Collections | `?tab=collections` | Phase 1 默认，承载现有集合树 + 请求编辑 |
| Request | `?tab=request:{nodeID}` | 请求详情；打开时自动展开对应集合 |
| Environment | `?tab=env:{envID}` | 环境编辑器 |
| Spec | `?tab=spec:{specID}` | API Spec |
| Mock | `?tab=mock:{mockID}` | Mock 服务 |
| Doc | `?tab=doc:{docID}` | 文档 |

规则：

- 标签列表存 App 级 `map[wsID]*WorkbenchState`，切换工作区后保留。
- 标签切换 `Replace`，不写历史；关闭当前标签后切到相邻标签，没有则回 Overview。
- 深链进入时，如果 `tab` 指向的资源存在就打开，否则回 Overview。
- 请求编辑器的内部 Params/Headers/Body 等子标签是**页面内状态**，不进 URL；URL 只到 request 级。

### 2.3 资源侧栏

按 1 号画面：顶部有三个模式按钮（Collections / History / 文档类），下面是搜索框，再下面是一组可折叠分区：

```
COLLECTIONS
ENVIRONMENTS
SPECS
MOCKS
DATASETS
FLOWS
TOOLSETS
DOCUMENTS
```

Phase 1 先沿用现有集合侧栏（搜索 + 树 + 环境菜单），Phase 3 再重构成统一手风琴。不要把「分区展开状态」放进路由。

## 3. 全局层

| 画面 | 帧 | 实现 |
| --- | --- | --- |
| Workspace 下拉 | 23 / 25 / 26 / 27 | TitleBar 内的 Popover；Create → `/hub/workspaces/new`，View all → `/hub/workspaces`，选中工作区 → `Replace("/w/{id}")` |
| Settings 弹窗 | 24 | 全局 Dialog；Appearance / Data mode / Shortcuts / Language |
| 通知中心 | 无帧 | 先做入口与空态 Popover |
| 全局搜索 | TitleBar 搜索框 | 先做静态入口，Phase 5 接搜索 |

## 4. 深色与平台

- `设计V1` 中 1–27 都有 Dark 对应帧（`48:*`），30 / 31 是管理端 Dark。
- Phase 2 同步做 Light / Dark token；不接受「先只做 Light、Dark 后面补」导致组件写死浅色。
- 标题栏是自绘的，Windows / macOS 的窗口按钮占位不同；继续使用 `c.TitleBar()` 的 `Left/Right/Height`，不要写死 40 / 72。设计稿标题栏已统一为 32（含底部 1px 分割线），代码同样跟随平台（Windows 32 / macOS 交通灯 / Linux 桌面）。
- 设计稿按 1280×800 绘制；最小窗口 960×600 仍需可用，侧栏可折叠/缩窄在 Phase 3 处理。

## 5. 写死数据清单（本版允许）

| 位置 | 写死内容 |
| --- | --- |
| Hub 团队头部 | `wang's Team`、`1 member`、Invite |
| Workspaces 列表 | Creator `You`、Contributors `1`、Last activity `Just now`、Access `Internal`、Your role `Admin`（默认工作区可显示 `Default`） |
| Create workspace | 访问范围 `Everyone in wang's Team`、类型 Internal / Partner / Public、Open a local folder 入口 |
| TitleBar | Upgrade、通知红点、头像 |
| 管理端占位页 | 标题、说明、空态文案 |
| 工作台空态 | Overview 的最近请求/集合统计可先用现有数据或占位 |

原则：写死数据集中在 `features/hub` 的 mock 常量里，不散落到组件；不写入 store，不参与持久化。

## 6. 设计缺口

以下内容在稿中没有独立画面，需要后续补设计或先按现有结构实现：

- Hub Home 的内容结构（最近工作区、快捷入口、团队动态）。
- Application Inventory / API Catalog / Private & Public Network / Agent Context / Integrations / Reports 的页面内容。
- 通知中心。
- 全局搜索的结果页/浮层。
- 工作区设置（重命名、删除、成员、权限）入口与弹窗。
- Mock / Spec / Dataset / Flow / Toolset / Document 的编辑页。
- 请求标签的右键菜单、拖拽排序、溢出处理。
- 侧栏折叠、窄窗口适配。

这些缺口不阻塞 Phase 1–3；Phase 5 前需要产品确认优先级。
