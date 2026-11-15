// Package app 组装应用外壳：标题栏、图标栏、侧栏、主区域与状态栏。
package app

import (
	"log"

	"rddpostman/components/theme"
	v1 "rddpostman/components/v1"
	"rddpostman/features/collection"
	"rddpostman/features/demo"
	"rddpostman/features/environment"
	"rddpostman/features/history"
	"rddpostman/features/home"
	"rddpostman/features/request"
	"rddpostman/features/settings"
	"rddpostman/internal/routes"
	"rddpostman/internal/store"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// 路由别名，方便包内引用。
const (
	routerHome        = routes.Home
	routeWorkspace    = routes.Workspace
	routeCollections  = routes.Collections
	routeCollection   = routes.Collection
	routeEnvironments = routes.Environments
	routeHistory      = routes.History
	routeMock         = routes.Mock
	routeNetwork      = routes.Network
	routeDemo         = routes.Demo
	routeSettings     = routes.Settings
)

// App 保存窗口级状态。
type App struct {
	router *ui.Router
	store  *store.Store
}

// New 创建应用状态，默认打开集合首页。
func New() *App {
	a := &App{router: ui.NewRouter(routeCollections)}
	s, err := store.Open()
	if err != nil {
		log.Printf("打开本地数据失败: %v", err)
	}
	a.store = s
	return a
}

// View 绘制整个窗口。
func (a *App) View(c *ui.Context) {
	c.SetTheme(theme.Apply(c.Theme(), c.Preferences()))

	bar := c.TitleBar()
	root := ui.Column(c).Fill()
	// 当前工作区：标题栏选择器与状态栏共用。
	selected := ui.Local(root, "workspace", func() string {
		if ws := a.defaultWorkspace(); ws != nil {
			return string(ws.ID)
		}
		return ""
	})
	root.Children(func() {
		a.titleBar(c, bar, selected)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			a.iconRail(c)
			a.content(c)
		})
		a.statusBar(c, a.workspaceName(*selected))
	})
}

// workspaces 返回当前快照里的工作区。
func (a *App) workspaces() []*store.Workspace {
	if a.store == nil {
		return nil
	}
	return a.store.Snapshot().Workspaces
}

// workspace 按 ID 找一个工作区。
func (a *App) workspace(id string) *store.Workspace {
	for _, w := range a.workspaces() {
		if string(w.ID) == id {
			return w
		}
	}
	return nil
}

// defaultWorkspace 返回默认工作区，没有默认标记时退回第一个。
func (a *App) defaultWorkspace() *store.Workspace {
	list := a.workspaces()
	for _, w := range list {
		if w.Default {
			return w
		}
	}
	if len(list) > 0 {
		return list[0]
	}
	return nil
}

// workspaceName 返回工作区名，找不到时用「工作区」兜底。
func (a *App) workspaceName(id string) string {
	if w := a.workspace(id); w != nil {
		return w.Name
	}
	return locales.WorkspaceTitle()
}

// content 是图标栏右侧的主区域。每个页面自己决定是否带侧栏、是否滚动。
func (a *App) content(c *ui.Context) {
	tk := theme.Of(c)
	ui.Box(c).Grow(1).Background(tk.BgBase).Children(func() {
		a.router.View(c, func(r *ui.Route) {
			switch {
			case r.Match(routerHome):
				r.Title(locales.RailHome())
				home.View(c)
			case r.Match(routeWorkspace):
				r.Title(locales.WorkspaceTitle())
				home.View(c)
			case r.Match(routeCollections):
				r.Title(locales.CollectionsTitle())
				collection.View(c, a.router)
			case r.Match(routeCollection):
				r.Title(locales.RequestPageTitle())
				request.View(c, a.router)
			case r.Match(routeEnvironments):
				r.Title(locales.EnvironmentTitle())
				environment.View(c, a.router)
			case r.Match(routeHistory):
				r.Title(locales.HistoryTitle())
				history.View(c, a.router)
			case r.Match(routeMock):
				r.Title(locales.RailMock())
				placeholder(c, locales.RailMock(), locales.RailMockDesc())
			case r.Match(routeNetwork):
				r.Title(locales.RailNetwork())
				placeholder(c, locales.RailNetwork(), locales.RailNetworkDesc())
			case r.Match(routeDemo):
				r.Title(locales.RailDemo())
				demo.View(c)
			case r.Match(routeSettings):
				r.Title(locales.SettingsTitle())
				settings.View(c)
			default:
				r.Title(locales.NotFound())
				ui.Text(c, locales.NotFound()).FontSize(12).TextColor(tk.TextMuted).Padding(24)
			}
		})
	})
}

// placeholder 画尚未实现的模块页，保持外壳与路由可用。
func placeholder(c *ui.Context, title, desc string) {
	ui.Scroll(c).Grow(1).Children(func() {
		v1.PageHeader(c, title, desc)
		v1.EmptyState(c, locales.UnderConstruction())
	})
}
