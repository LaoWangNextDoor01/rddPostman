// Package app 组装应用外壳：标题栏、图标栏、侧栏、主区域与状态栏。
package app

import (
	"rddpostman/components/theme"
	v1 "rddpostman/components/v1"
	"rddpostman/features/collection"
	"rddpostman/features/demo"
	"rddpostman/features/environment"
	"rddpostman/features/history"
	"rddpostman/features/home"
	"rddpostman/features/request"
	"rddpostman/features/settings"
	"rddpostman/internal/logger"
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
	routeDocs         = routes.Docs
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
		logger.Error("打开本地数据失败: %v", err)
	} else if len(s.Snapshot().Problems) > 0 {
		for _, p := range s.Snapshot().Problems {
			logger.Warn("数据加载警告: %s", p)
		}
	}
	a.store = s
	return a
}

// Close 在退出前把防抖中的编辑立即落盘。
func (a *App) Close() {
	if a.store != nil {
		_ = a.store.Close()
	}
}

// View 绘制整个窗口。
func (a *App) View(c *ui.Context) {
	c.SetTheme(theme.Apply(c.Theme(), c.Preferences()))

	bar := c.TitleBar()
	root := ui.Column(c).Fill()
	root.Children(func() {
		a.titleBar(c, bar)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			a.iconRail(c)
			a.content(c)
		})
		a.statusBar(c)
	})
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
				collection.View(c, a.store, a.router)
			case r.Match(routeCollection):
				r.Title(locales.RequestPageTitle())
				request.View(c, a.router)
			case r.Match(routeEnvironments):
				r.Title(locales.EnvironmentTitle())
				environment.View(c, a.router)
			case r.Match(routeHistory):
				r.Title(locales.HistoryTitle())
				history.View(c, a.router)
			case r.Match(routeDocs):
				r.Title(locales.DocsTitle())
				placeholder(c, locales.DocsTitle(), locales.DocsDesc())
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
