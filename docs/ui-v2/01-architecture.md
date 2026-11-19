# 01 · 路由与布局架构

## 1. 目标

把当前「单外壳 + 图标栏 + 扁平路由」的结构，重构为：

- **一个 Router**：一个窗口、一份历史、一个当前位置。
- **两套 Layout**：团队级管理端 `HubLayout`、工作区工作台 `WorkbenchLayout`。
- **公共壳层外置**：TitleBar、StatusBar 不随页面切换重建。
- **URL 驱动工作区上下文**：`wsID` 从路径读取，不再依赖 store 的活跃工作区副作用。

## 2. 现状与问题

当前 `internal/app/app.go` 在窗口顶层硬编码：

```
TitleBar
Row( IconRail, content(router) )
StatusBar
```

问题：

1. 管理端（团队视角）与工作台（工作区视角）共用同一套图标栏，职责混淆。
2. 新设计稿已经取消 Icon Rail，工作台改为「资源侧栏 + 标签编辑区」。
3. 路由是扁平的 `/collections`、`/environments`、`/history`，工作区身份不在 URL 里；切换工作区只改 store，返回/深链没有确定语义。
4. 设置是独立页面，而新稿是全局弹窗。

## 3. 总体结构

```
┌──────────────────────────────────────────────────────────────┐
│ AppShell · TitleBar（常驻）                                   │
│ 团队/工作区切换 · 全局搜索 · Invite · 通知 · Settings · 头像   │
├──────────────────────────────────────────────────────────────┤
│ Router.View                                                   │
│                                                              │
│  /hub/{page...}                  /w/{wsID}                   │
│  ┌───────────┬──────────────┐    ┌───────────┬────────────┐  │
│  │ HubLayout │ 页面内容      │    │ Workbench │ 标签 +     │  │
│  │ 团队导航   │ r.View       │    │ 资源侧栏   │ 编辑区      │  │
│  └───────────┴──────────────┘    └───────────┴────────────┘  │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│ AppShell · StatusBar（常驻）                                  │
└──────────────────────────────────────────────────────────────┘
```

职责边界：

| 区域 | 职责 | 是否随路由重建 |
| --- | --- | --- |
| TitleBar | 团队/工作区上下文、全局搜索、设置/通知入口 | 否 |
| StatusBar | 当前工作区、模式、Console、版本 | 否 |
| HubLayout 侧栏 | 团队级导航 | 仅在 Hub 内切换时不重建 |
| Hub 页面内容 | Home / Workspaces / 其他管理页 | 是（内容过渡） |
| WorkbenchLayout 侧栏 | Collections / Environments / Specs / Mocks / Datasets / Flows / Toolsets / Documents | 同一工作区内不重建 |
| Workbench 标签区 | Overview、Request、Environment、Spec、Mock、Doc | 标签切换只换内容 |
| Settings / 搜索 / 通知 / Workspace 下拉 | 全局 overlay | 与路由无关 |

## 4. 路由表

```
/                          → 启动兜底 /hub/home

/hub/{page...}             → HubLayout
  /hub/home
  /hub/workspaces
  /hub/workspaces/new
  /hub/inventory
  /hub/catalog
  /hub/network/private
  /hub/network/public
  /hub/agent-context
  /hub/integrations
  /hub/reports

/w/{wsID}                  → WorkbenchLayout
  ?tab=collections         （Phase 1 默认，沿用现有集合工作台）
  ?tab=overview            （Phase 3 增加）
  ?tab=request:{nodeID}
  ?tab=env:{envID}
  ?tab=spec:{specID}
  ?tab=mock:{mockID}
  ?tab=doc:{docID}
```

说明：

- 管理端所有页面共享 `/hub` 前缀，`HubLayout` 用 `Route.View` 承载内页；在 `/hub/*` 内切换时侧栏保持，只有内容过渡。
- 工作台只有一个路径 `/w/{wsID}`，标签放在 query。这样标签切换不改变页面的路径身份，Router 会复用同一个 page，侧栏、标签列表、滚动状态都不会丢。
- 不采用 `/w/{wsID}/collections` 这类 section 路由：它会和 `?tab=` 形成两套事实来源，后续还要迁移。Phase 1 直接使用 query。
- 如果未来某个功能区需要自己的持久子布局（例如工作区设置子页），再引入 `/w/{wsID}/{section...}`，不影响现有 query 标签。

## 5. MyGo 机制与约束

### 5.1 Layout 复用规则

MyGo 的 `Route.View` 把当前页变成 layout，内层页面由剩余路径匹配。一个 layout 是否在跳转时复用，取决于它的通配符前缀是否保持不变：

- `/hub/{page...}` 消耗 1 段路径：`/hub/home` ↔ `/hub/workspaces` 前缀都是 `hub`，所以 HubLayout 页面复用，侧栏不重建。
- `/w/{wsID}` 不含 `Route.View`，工作台是单个页面；query 变化路径不变，页面复用。
- `/hub/*` ↔ `/w/*` 第一段就不同，整个 Layout 页面被替换，产生一次过渡。

### 5.2 `Route.View` 的前提

`Route.View` 要求当前 `Match` 的模式里含有 `{name...}` 通配符，否则会 panic。因此 Hub 的路由必须写成 `/hub/{page...}`，不能只写 `/hub`。

### 5.3 转场

- 单个 Router 只有一个 `Transition` 配置，无法给 Hub 内页和工作台标签分别设置动画。
- 本版建议统一 `TransitionFade`（150ms），桌面端更稳；不在 Phase 1 做滑动转场。
- query 的 `Replace` 不触发页面切换动画，符合「标签切换」的预期。

## 6. 导航语义

| 动作 | 调用 | 理由 |
| --- | --- | --- |
| 从 Workspaces 列表进入工作区 | `Push("/w/{id}")` | 返回应回到列表 |
| 标题栏切换工作区 | `Replace("/w/{id}?tab=...")` | 切换上下文，不污染历史 |
| 创建完成 | `Replace("/w/{newID}")` | 返回不回创建表单 |
| Hub 内导航 | `Push("/hub/...")` | 内容前进，侧栏保持 |
| 工作台切标签 | `Replace("?tab=...")` | 标签是并行工作集，不是层级页面 |
| 关闭最后一个标签 | `Replace("?tab=overview")` | 落到 Overview，不关闭工作台 |
| 删除当前工作区 | `Replace("/hub/workspaces")` | 避免历史里留下死路由 |
| 打开 Settings | 本地状态，不改路由 | 弹窗覆盖当前页面，不卸载 Layout |

## 7. 状态归属

| 状态 | 存放位置 | 是否持久化 |
| --- | --- | --- |
| 当前 Hub 页面 | URL `/hub/...` | 否 |
| 当前工作区 | URL `/w/{wsID}` | 否；`store.ActiveWorkspaceID` 只记「上次打开」 |
| 当前激活标签 | URL `?tab=` | 否 |
| 打开的标签列表 | App 级 `map[domain.ID]*WorkbenchState` | 否（本版不落盘） |
| 每个工作区的侧栏展开状态 | 同上，或页面本地 | 否 |
| 滚动位置、输入草稿、搜索词 | 页面本地 `ui.Local` | 否 |
| 集合、环境、历史、请求内容 | `internal/store` | 是 |
| 主题、语言、窗口大小 | 现有 Preferences / Window State | 是 |

`WorkbenchState` 建议结构：

```go
type WorkbenchState struct {
    Tabs   []Tab
    Active string
}

type Tab struct {
    ID         string      // 稳定身份，如 "request:nodeID"
    Kind       TabKind     // overview / request / environment / spec / mock / doc
    ResourceID domain.ID
    Title      string
    Dirty      bool
}
```

约束：

- `store` 只存业务数据，不存 UI 标签。
- 切换工作区时，从 map 取出该工作区的标签列表；第一次进入时创建默认 Overview / Collections。
- 应用退出后标签不恢复（本版）；如果以后要恢复，另存到本地 UI 配置，不进 store。

## 8. 工作区上下文与守卫

### 8.1 URL 是唯一事实来源

- `WorkbenchLayout` 用 `r.Param("wsID")` 查 `Snapshot.Workspace(id)`，所有工作区数据都按这个 ID 取。
- TitleBar 和 StatusBar 也按当前 URL 解析工作区名，不再读 `ActiveWorkspace()`。
- 现有 `snap.ActiveWorkspace()` 的页面（collection / environment / history）在重构时补 `ViewWorkspace(wsID)` 变体；这是 Phase 1 的兼容改动。

### 8.2 无效工作区

- 不自动重定向。渲染「工作区不存在」兜底页，提供「返回工作区列表」按钮。
- 原因：在 build 期间调用 `Push/Replace` 容易造成重定向循环；MyGo 也建议把副作用放到动作里，而不是渲染代码里。
- 删除当前工作区时，由执行删除的动作方 `Replace("/hub/workspaces")`。

### 8.3 启动与恢复

- 默认启动 `/hub/home`。
- 可选：在 Phase 6 记住上次 `router.Location()`，启动时如果工作区仍存在就恢复到工作台，否则回 `/hub/home`。
- 不把「上次位置」写进 store，避免业务数据和 UI 状态耦合。

## 9. 全局层

以下能力不做路由：

- Settings 弹窗（设计稿 24）
- 全局搜索（TitleBar）
- 通知中心
- Workspace 下拉（23、25、26、27）
- 新建 / 重命名 / 删除确认等对话框
- Console / 日志弹窗

原因：这些是覆盖层，打开时不应卸载当前 Layout；用页面路由会导致返回键关闭弹窗、状态丢失、URL 污染。

## 10. 旧路由迁移

| 旧路径 | 新归属 |
| --- | --- |
| `/home` | `/hub/home` |
| `/collections` | `/w/{wsID}?tab=collections` |
| `/collections/{id}` | 工作台标签 `?tab=request:{id}`（Phase 3） |
| `/environments` | `/w/{wsID}?tab=env:{id}` 或 Environments 标签 |
| `/history` | 侧栏 History 模式 / 工作台标签 |
| `/docs` | 请求详情内的 Docs 标签 |
| `/mock` | 工作台 Mock 标签 |
| `/network` | `/hub/network/public` / `/private` |
| `/settings` | 全局弹窗 |
| `/demo` | 删除 |

旧路由常量在迁移期保留，标记 deprecated；Phase 6 统一删除。

## 11. 为什么不采用双 Router

| 方案 | 问题 |
| --- | --- |
| 两套 Router（管理端一套、工作台一套） | 两份 Location、两份历史；TitleBar/StatusBar 要判断当前属于哪套；MyGo 的全局 Alt+←/→ 只作用于第一个 Router；深链和窗口恢复要额外维护 mode 状态 |
| 两套 Layout 常驻、显隐切换 | 隐藏的 Layout 仍在焦点树与无障碍树里，需要自己处理焦点、滚动、资源占用；Router 已经能在历史里保留页面状态，没有必要 |
| 单 Layout + 条件插槽 | 两种侧栏、两种内容区、两种标题栏上下文会堆成一个巨型条件组件，职责不清 |

单 Router + 双 Layout 的代价是转场动画只能全局统一，但换来的是单历史、单位置、跨模式返回自然、深链简单。

## 12. 验收清单

- [ ] 启动进入 `/hub/home`，TitleBar / StatusBar 常驻。
- [ ] `/hub/*` 内切换页面时，Hub 侧栏不重建、状态保留。
- [ ] 从列表进入 `/w/{id}` 后，Alt+← 能回到列表。
- [ ] 标题栏切换工作区使用 `Replace`，不增加历史条目。
- [ ] 工作台内切 `?tab=` 不增加历史条目，页面状态不丢。
- [ ] 无效 `wsID` 显示兜底页，不崩溃、不循环重定向。
- [ ] 删除当前工作区后回到 `/hub/workspaces`，历史里没有死路由。
- [ ] Settings 弹窗可在 Hub / Workbench 两套 Layout 上打开，关闭后页面状态不丢。
- [ ] `go test ./...` 与 `go tool mygo vet .` 通过。
