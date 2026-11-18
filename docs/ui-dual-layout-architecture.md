# Postman 风格双布局（Dual Layout）架构设计方案

## 1. 概述与背景

### 1.1 现状与痛点
当前 `rddPostman` 采用全局单外壳结构（`internal/app/app.go`），在顶层窗口直接硬编码了 `a.iconRail(c)`（集合、历史、环境等图标栏）与底部状态栏：
* **界面不协调**：在浏览“所有工作空间 / 首页”等全局大厅页面时，依然展示着具体工作空间专用的图标栏，挤占页面可用空间。
* **职责混淆**：大厅页（管理视角）与工作台（调试视角）的 UI 逻辑强行揉合在一起，缺乏清晰的上下文边界。

### 1.2 改造目标
对标官方 Postman 客户端的设计哲学：
1. **解耦两套独立 Layout**：
   * **工作空间大厅布局（`HubLayout`）**：宏观管理模式，无 Icon Rail，左侧扁平菜单 + 右侧工作空间卡片网格。
   * **工作台沉浸式布局（`WorkbenchLayout`）**：研发调试模式，包含 Icon Rail + 可拖拽树形侧栏 + Request 多标签编辑区 + 底部状态栏。
2. **利用 MyGo 原生路由能力**：通过 `*ui.Router` 与 `r.View(c, ...)` 嵌套机制，实现工作台侧栏状态不丢失、子页面平滑过渡、全局路由自由跳转与历史栈完整支持。

---

## 2. Postman 界面模型与对比

```
                        ┌────────────────────────────────────────────────────────┐
                        │                   全局标题栏 TitleBar                   │
                        │   (Logo / 工作空间下拉切换 / 全局搜索 / 设置 / 个人头像) │
                        └────────────────────────────────────────────────────────┘
                                                    │
                     ┌──────────────────────────────┴──────────────────────────────┐
                     ▼                                                             ▼
         【模式 A: HubLayout 工作空间大厅】                              【模式 B: WorkbenchLayout 工作台】
       （路由：/workspaces 或 /home）                                 （路由：/workspaces/{wsID}/{section...}）
    ┌───────────┬───────────────────────────────┐       ┌────┬────────────┬─────────────────────────────┐
    │ 扁平菜单   │ 工作空间大厅主区域             │       │Icon│ 树形侧栏   │ 顶部 Request Tabs            │
    │ (Flat)    │                               │       │Rail│ (Tree)     ├─────────────────────────────┤
    │ ───────── │ ┌─────────┐  ┌─────────┐      │       │    │ ────────── │ Request 主编辑区             │
    │ • 全部空间 │ │工作空间A│  │工作空间B│      │       │ 📁 │ ▶ 集合 1   │ (Params, Headers, Body...)  │
    │ • 个人空间 │ └─────────┘  └─────────┘      │       │    │   ▶ 目录A  │                             │
    │ • 团队空间 │ ┌─────────┐  ┌─────────┐      │       │ 🌐 │     GET xx │ ─────────────────────────── │
    │ • 最近访问 │ │工作空间C│  │ + 新建 │      │       │    │ ▶ 集合 2   │ 响应结果面板 (Response)     │
    │           │ └─────────┘  └─────────┘      │       │ 🕒 │            │                             │
    └───────────┴───────────────────────────────┘       ├────┴────────────┴─────────────────────────────┤
                                                        │ 底部状态栏 StatusBar (Console / Cookies)       │
                                                        └───────────────────────────────────────────────┘
```

### 两种布局的特征对比

| 布局属性 | 模式 A：工作空间大厅（`HubLayout`） | 模式 B：工作空间工作台（`WorkbenchLayout`） |
| :--- | :--- | :--- |
| **核心职责** | 空间的创建、组织、检索、切换 | 具体的接口测试、文档编写、脚本与环境调试 |
| **最左侧栏** | **无** Icon Rail，为常规扁平列表菜单 | **有** 窄型垂直 Icon Rail（Collections, Envs, History...） |
| **二级侧栏** | 无（直接在右侧卡片区平铺展示） | **有** 可折叠、可左右拖拽宽度的树形侧栏（Tree View） |
| **主内容区** | 卡片瀑布流/网格（Card Grid） | 多标签页（Tabbed Workbench）+ 请求/响应拆分视图 |
| **底部状态栏** | **无** | **有**（Console 控制台、环境指示器、网络代理） |
| **页面路由** | `/workspaces` | `/workspaces/{wsID}/collections`, `/workspaces/{wsID}/environments`... |

---

## 3. 路由体系设计

### 3.1 路由规划规范

在 `internal/routes/routes.go` 中规范路径常量：

```go
package routes

const (
    // 大厅与全局页面
    HubWorkspaces = "/workspaces" // 工作空间大厅（全部工作空间列表）
    Settings      = "/settings"   // 全局设置

    // 工作台模式（带工作空间 ID 前缀与动态通配符）
    WorkbenchRoot = "/workspaces/{wsID}/{section...}"
    
    // 子功能区相对路径
    SectionCollections  = "/collections"
    SectionCollection   = "/collections/{id}"
    SectionEnvironments = "/environments"
    SectionHistory      = "/history"
    SectionMock         = "/mock"
    SectionNetwork      = "/network"
)

// WorkbenchCollectionsOf 辅助构造进入指定工作空间的集合页
func WorkbenchCollectionsOf(wsID string) string {
    return "/workspaces/" + wsID + "/collections"
}

func WorkbenchCollectionOf(wsID, collID string) string {
    return "/workspaces/" + wsID + "/collections/" + collID
}
```

### 3.2 导航动作规范

* **进入工作空间**：在大厅卡片点击某空间，执行 `router.Push("/workspaces/" + wsID + "/collections")`。
* **返回大厅**：在工作台顶部下拉选择“查看所有工作空间”，或点击全局返回按钮，执行 `router.Push("/workspaces")`。
* **工作台内部切换**：点击 Icon Rail 图标（如切换到环境），执行 `router.Push("/workspaces/" + wsID + "/environments")`。由于处于同一 `WorkbenchLayout`，MyGo 保证仅内部主区域发生平滑切换，工作台外壳与侧栏树维持挂载。

---

## 4. 技术实现方案（MyGo 代码范式）

### 4.1 顶层入口：`internal/app/app.go`

剥离硬编码的 `iconRail`，仅保留全局通用的标题栏：

```go
func (a *App) View(c *ui.Context) {
    c.SetTheme(theme.Apply(c.Theme(), c.Preferences()))
    bar := c.TitleBar()

    ui.Column(c).Fill().Children(func() {
        // 1. 全局自绘标题栏（常驻）
        a.titleBar(c, bar)

        // 2. 主区域：由 Router 根据当前 URL 分发到不同 Layout
        a.router.View(c, func(r *ui.Route) {
            switch {
            // 模式 A：工作空间大厅
            case r.Match("/workspaces"), r.Match("/home"):
                r.Title("工作空间大厅")
                a.hubLayout(c, r)

            // 模式 B：具体工作空间工作台
            case r.Match("/workspaces/{wsID}/{section...}"):
                a.workbenchLayout(c, r)

            // 全局独立页面
            case r.Match("/settings"):
                r.Title("系统设置")
                settings.View(c)

            default:
                // 404 兜底，自动退回大厅
                a.hubLayout(c, r)
            }
        })
    })
}
```

### 4.2 大厅布局实现：`hubLayout`

位于 `internal/app/hub_layout.go`（或 `features/home`）：

```go
func (a *App) hubLayout(c *ui.Context, r *ui.Route) {
    m := theme.Metrics
    tk := theme.Of(c)

    ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
        // 左侧普通扁平分类栏（220px）
        ui.Column(c).Width(220).Background(tk.BgSurface).
            BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).
            Padding(m.Space3).Gap(m.Space1).Children(func() {
                ui.Text(c, "Workspaces").FontSize(m.FontLG).Bold().MarginBottom(m.Space2)
                
                a.hubMenuItem(c, "全部工作空间", true)
                a.hubMenuItem(c, "个人工作空间", false)
                a.hubMenuItem(c, "团队共享", false)
            })

        // 右侧内容区：标题 + 搜索筛选 + 卡片网格
        ui.Scroll(c).Grow(1).Padding(m.Space4).Children(func() {
            ui.Row(c).JustifyContent(ui.SpaceBetween).AlignItems(ui.Center).MarginBottom(m.Space4).Children(func() {
                ui.Text(c, "所有工作空间").FontSize(m.FontXL).Bold()
                if ui.Button(c, "+ 新建工作空间").Clicked() {
                    // 打开新建弹窗
                }
            })

            // 卡片网格流
            snap := a.store.Snapshot()
            ui.Grid(c).Columns(3).Gap(m.Space3).Children(func() {
                for _, ws := range snap.Workspaces {
                    wsID := ws.ID
                    card := ui.ButtonBase(c.Key("ws-" + string(wsID))).
                        Radius(m.RadiusMD).Padding(m.Space3).Background(tk.BgSurface).
                        Border(m.Border, tk.Border)

                    if card.Clicked() {
                        // 🚀 核心联动：切换数据状态并跳转进该工作台
                        _ = a.store.SelectWorkspace(wsID)
                        a.router.Push(fmt.Sprintf("/workspaces/%s/collections", wsID))
                    }

                    card.Children(func() {
                        ui.Text(c, ws.Name).FontSize(m.FontMD).Bold()
                        ui.Text(c, ws.Description).FontSize(m.FontSM).TextColor(tk.TextSecondary)
                    })
                }
            })
        })
    })
}
```

### 4.3 工作台布局实现：`workbenchLayout`

位于 `internal/app/workbench_layout.go`：

```go
func (a *App) workbenchLayout(c *ui.Context, r *ui.Route) {
    wsID := r.Param("wsID") // 自动解析当前工作空间 ID

    ui.Column(c).Grow(1).Children(func() {
        // 主交互区
        ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
            // 1. 工作台专属的垂直 Icon Rail（下沉到这里）
            a.workbenchIconRail(c, wsID)

            // 2. 工作台子视图挂载点：保持外壳不动，仅替换内部内容
            r.View(c, func(sub *ui.Route) {
                switch {
                case sub.Match("/collections"):
                    sub.Title("集合")
                    collection.View(c, a.store, a.router)

                case sub.Match("/collections/{id}"):
                    sub.Title("请求详情")
                    request.View(c, a.router)

                case sub.Match("/environments"):
                    sub.Title("环境变量")
                    environment.View(c, a.router)

                case sub.Match("/history"):
                    sub.Title("历史记录")
                    history.View(c, a.router)

                case sub.Match("/mock"):
                    sub.Title("Mock 服务")
                    mock.View(c, a.router)
                }
            })
        })

        // 3. 工作台专属的底部状态栏（Console、网络状态）
        a.statusBar(c)
    })
}
```

---

## 5. 状态管理与生命周期隔离

1. **工作空间活动上下文（Active Workspace）**：
   * 路由中的 `{wsID}` 成为最高优先级的事实来源（Single Source of Truth）。
   * 用户直接通过深层链接进入时，App 自动通过 `store.SelectWorkspace(domain.ID(wsID))` 同步活跃空间。
2. **UI 瞬时状态隔离**：
   * **工作台状态**（集合树各节点的展开/折叠、侧边栏宽度、已打开的 Request Tab 列表）：与工作空间关联，保存在以 `workspace-{wsID}` 为 Key 的 `ui.Local` 中。
   * **大厅状态**（搜索关键词、过滤类别）：仅在大厅生命周期内有效，不干扰工作台。
3. **后退与前进体验**：
   * 在工作台里调试完，点击系统后退按钮或 `Alt + Left`，自然回到上一个请求或退回大厅。

---

## 6. 后续实施路径计划

待应用的核心功能（请求执行、数据持久化、环境解析等）完善后，按以下步骤平滑重构：

```
[阶段 1: 路由体系升级]
  └── 更新 internal/routes/routes.go，引入 /workspaces 与 /workspaces/{wsID}/...

[阶段 2: 拆分 Layout 骨架]
  ├── 重构 internal/app/app.go 顶层 switch
  ├── 将 iconRail 从全局移入 workbenchLayout
  └── 编写 hubLayout 雏形与菜单结构

[阶段 3: 大厅与卡片交互]
  ├── 将 features/home 改造为标准工作空间大厅
  └── 实现卡片点击跳转到具体工作空间工作台

[阶段 4: 工作台子路由对接]
  └── 将 collection.View, environment.View 等完整挂载进 r.View 内部

[阶段 5: 体验与动效调优]
  └── 调整页面转场动画 (TransitionFade / TransitionAuto) 与快捷键
```
