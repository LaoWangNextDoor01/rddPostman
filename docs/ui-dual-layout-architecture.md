# 双分组 Layout（HubLayout / WorkbenchLayout）架构设计

## 1. 总体分层架构

借鉴 Next.js Route Groups 思路，应用外壳拆分为**常驻 RootLayout** 与 **两套分组 Layout**，依托 MyGo `ui.Router` 的原生嵌套 `r.View` 按路径前缀保活各层状态：

```text
RootLayout (internal/app/app.go + shell.go)
├── 常驻 TitleBar（< > 路由历史、Workspace 切换下拉、全局搜索、通知、设置入口、账号）
├── 主路由槽 (a.router.View)
│   ├── (hub) 分组布局  /hub/{section...}           ← internal/app/layout_hub.go
│   │   ├── 左侧：常驻团队头 + Hub 导航菜单（可拖拽 Split 宽度）
│   │   └── 右侧 r.View 子路由：
│   │       ├── /hub/workspaces                   → features/workspace.ListView
│   │       ├── /hub/workspaces/new               → features/workspace.CreateView
│   │       └── /hub/home, /hub/catalog, ...      → Hub 其余管理页
│   │
│   └── (workbench) 分组布局  /w/{wsID}/{section...} ← internal/app/layout_workbench.go
│       ├── 左侧：顶部模块图标切换条 + r.View 面板槽（Collections / History / Environments / Specs）
│       └── 右侧：常驻多 Tab 主编辑区（挂载在 r.View 外层，切左侧图标完全保活不卸载）
│           ├── Request Tab（请求编辑器）
│           ├── Overview Tab（集合/目录概览）
│           └── Environment Tab（环境变量 KEY/VALUE 编辑器，features/environment.EditorView）
├── 常驻 StatusBar（当前工作空间、日志/Console 弹窗、版本号）
└── 全局弹窗层：Settings Dialog（features/settings.Dialog，不切路由、两套 Layout 通用）
```

---

## 2. 路由与状态保活机制

在 `internal/routes/routes.go` 中约定两套分组前缀：

| 分组 | 路由模板 | 保活前缀段 | 子路由示例 |
| :--- | :--- | :--- | :--- |
| **`(hub)`** | `/hub/{section...}` | `["hub"]` | `/hub/workspaces`、`/hub/workspaces/new`、`/hub/home`、`/hub/catalog` |
| **`(workbench)`** | `/w/{wsID}/{section...}` | `["w", wsID]` | `/w/{wsID}/collections`、`/w/{wsID}/history`、`/w/{wsID}/environments`、`/w/{wsID}/specs` |

- **同组切换保活**：MyGo `Router` 在发现新路径与当前路径共享外层 `r.View` 对应的前缀段时，保持外层 Layout 节点不卸载，因此：
  - 在 `(hub)` 内从 `/hub/workspaces` 进入 `/hub/workspaces/new` 或切换菜单项，左侧 Hub 侧栏宽度与滚动位置保持不变；
  - 在 `(workbench)` 内点击顶部图标从 `Collections` 切到 `History` 或 `Environments`，**仅左侧面板切换，右侧多 Tab 请求与环境变量编辑区完全保持挂载不动**；
  - 切换到另一个 `wsID` 时，前缀 `["w", wsID]` 变化，自动切换至对应工作空间的独立工作台状态。
- **全局设置弹窗**：挂载在 `RootLayout` 顶层（`settings.Dialog`），在任意页面点击标题栏齿轮或菜单 `Settings` 直接弹起，关闭后不改变路由、不丢失页面与 Tab 状态。

---

## 3. 关键目录分工

- `internal/routes/routes.go`：分组模板与路径构造器。
- `internal/app/app.go`：RootLayout 装配、分组路由分发、全局 Settings 弹窗。
- `internal/app/shell.go`：TitleBar（`< >` 历史按钮、Workspace 搜索/图标下拉、快捷键角标）、StatusBar、系统日志弹窗。
- `internal/app/layout_hub.go`：`(hub)` 分组布局与团队侧栏。
- `internal/app/layout_workbench.go`：`(workbench)` 分组布局、左侧图标条与多类型 Tab 注入。
- `features/workspace/`：`ListView`、`CreateView`、本地目录扫描、工作区图标复制与共享渲染（`RenderIconBadge`）。
- `features/collection/`：`WorkbenchView`、多类型 Tab 栏（Request / Overview / Environment）、集合树与请求编辑器。
- `features/environment/`：`SidebarView`（工作台侧栏列表）与 `EditorView`（右侧环境变量 Tab 编辑器）。
- `features/history/`：`SidebarView`（按日期分组的请求历史侧栏，点击打开对应请求 Tab）。
- `features/settings/`：`Dialog`（RootLayout 全局设置弹窗）。
