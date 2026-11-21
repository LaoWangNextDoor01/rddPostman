package app

import (
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/features/collection"
	"rddpostman/features/environment"
	"rddpostman/features/history"
	"rddpostman/internal/domain"
	"rddpostman/internal/routes"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// workbenchLayout 实现 (workbench) 分组布局（路由 /w/{wsID}/{section...}）：
//   - 左侧栏：顶部模块图标切换条（Collections / History / Environments / Specs）+ r.View 动态面板；
//   - 右侧区：常驻多 Tab 请求编辑区（直接挂载在 WorkbenchLayout 下，不随左侧图标切换卸载或重建）。
func (a *App) workbenchLayout(c *ui.Context, r *ui.Route) {
	if a.store == nil {
		return
	}
	snap := a.store.Snapshot()
	wsID := domain.ID(r.Param("wsID"))
	ws := snap.Workspace(wsID)
	if ws == nil {
		ws = snap.ActiveWorkspace()
	}
	if ws == nil {
		return
	}
	if snap.ActiveWorkspaceID != ws.ID {
		_ = a.store.SetActiveWorkspace(ws.ID)
	}
	wsID = ws.ID
	idStr := string(wsID)

	root := ui.Box(c.Key(keyLayoutWorkbench + "-" + idStr)).Fill()
	root.Children(func() {
		collection.WorkbenchView(c, a.store, a.router, wsID, collection.WorkbenchSlots{
			RenderLeft: func(renderCollections func(), openRequest func(nodeID string), openEnv func(envID domain.ID)) {
				m, tk := thV2.Metrics, thV2.Of(c)
				ui.Column(c).Fill().Background(tk.BgApp).Children(func() {
					// 1. 顶部工作台左侧模块图标切换条（对齐 Pixso 8-请求-Body：36px 高、BgApp、无底线）
					a.workbenchSidebarTabs(c, idStr)

					// 2. 下方面板区：走 MyGo 原生嵌套 r.View，确保同一 wsID 下切换左侧面板时
					//    WorkbenchLayout 与右侧多 Tab 主编辑区完全保持挂载不动。
					ui.Box(c).Grow(1).FillWidth().Background(tk.BgApp).Children(func() {
						r.View(c, func(sub *ui.Route) {
							switch {
							case sub.Match(routes.SectionCollections), sub.Match("/"):
								sub.Title(locales.CollectionsTitle())
								renderCollections()
							case sub.Match(routes.SectionHistory):
								sub.Title(locales.HistoryTitle())
								history.SidebarView(c, a.store, wsID, openRequest)
							case sub.Match(routes.SectionEnvironments):
								sub.Title(locales.EnvironmentTitle())
								environment.SidebarView(c, a.store, wsID, openEnv)
							case sub.Match(routes.SectionSpecs), sub.Match(routes.SectionDocs):
								sub.Title(locales.DocsTitle())
								workbenchSidebarPlaceholder(c, "SPECS & DOCS", "API specifications and workspace documentation will appear here.", icons.File, m, tk)
							default:
								sub.Title(locales.CollectionsTitle())
								renderCollections()
							}
						})
					})
				})
			},
			RenderEnvTab: func(envID domain.ID) {
				environment.EditorView(c, a.store, wsID, envID)
			},
		})
	})
}

func (a *App) workbenchSidebarTabs(c *ui.Context, wsID string) {
	m, tk := thV2.Metrics, thV2.Of(c)
	curPath := a.router.Path()

	type tabItem struct {
		key     string
		label   string
		target  string
		section string
		icon    *ui.SVG
	}
	items := []tabItem{
		{key: "collections", label: "Collections", target: routes.WorkbenchCollectionsOf(wsID), section: routes.SectionCollections, icon: icons.Layers},
		{key: "environments", label: "Environments", target: routes.WorkbenchEnvironmentsOf(wsID), section: routes.SectionEnvironments, icon: icons.Globe},
		{key: "history", label: "History", target: routes.WorkbenchHistoryOf(wsID), section: routes.SectionHistory, icon: icons.Clock},
	}

	ui.Row(c).FillWidth().Height(36).Padding(0, m.Space3).Gap(m.Space1).
		AlignItems(ui.Center).Justify(ui.Center).
		Background(tk.BgApp).Children(func() {
		for _, it := range items {
			item := it
			active := strings.HasSuffix(curPath, item.section) ||
				(item.section == routes.SectionCollections && curPath == "/w/"+wsID)
			btn := v2.IconButton(c, v2.IconButtonOpts{
				Key:      "wb-tab-" + item.key,
				Label:    item.label,
				Tooltip:  item.label,
				Icon:     item.icon,
				Size:     28,
				IconSize: m.IconSM,
				Selected: active,
			})
			if btn.Clicked() {
				a.router.Push(item.target)
			}
		}
	})
}

func workbenchSidebarPlaceholder(c *ui.Context, title, desc string, icon *ui.SVG, m thV2.Metric, tk thV2.Tokens) {
	ui.Column(c).Fill().Background(tk.BgApp).Children(func() {
		ui.Row(c).FillWidth().Height(32).Padding(0, m.Space3).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontSize(11).FontWeight(600).TextColor(tk.TextMuted).SingleLine()
		})
		ui.Column(c).Grow(1).FillWidth().Center().Padding(m.Space4).Gap(m.Space2).Children(func() {
			ui.Icon(c, icon).FontSize(22).TextColor(tk.TextMuted)
			ui.Text(c, title).FontSize(m.FontSM).FontWeight(500).TextColor(tk.TextSecondary)
			ui.Text(c, desc).FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
	})
}
