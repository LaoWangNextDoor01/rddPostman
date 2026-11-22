// Package app 组装应用外壳（RootLayout）与两套分组布局（HubLayout / WorkbenchLayout）。
package app

import (
	"runtime"
	"sync"

	"rddpostman/components/theme"
	"rddpostman/features/demo"
	"rddpostman/features/settings"
	"rddpostman/internal/logger"
	"rddpostman/internal/routes"
	"rddpostman/internal/store"
	"rddpostman/locales"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// App 保存窗口级状态。
type App struct {
	router            *ui.Router
	store             *store.Store
	quitFn            func()
	mu                sync.Mutex
	settingsRequested bool
	settingsTab       string
	forceMacMenu      bool
	forceWindowsMenu  bool
}

// New 创建应用状态，默认进入 (hub) 工作空间列表页。
func New() *App {
	s, err := store.Open()
	if err != nil {
		logger.Error("打开本地数据失败: %v", err)
	} else if len(s.Snapshot().Problems) > 0 {
		for _, p := range s.Snapshot().Problems {
			logger.Warn("数据加载警告: %s", p)
		}
	}
	return NewWithStore(s)
}

// NewWithStore 使用指定的 Store 实例创建应用（便于测试隔离临时数据目录）。
func NewWithStore(s *store.Store) *App {
	a := &App{
		router: ui.NewRouter(routes.HubWorkspaces),
		store:  s,
	}
	a.InitPlatformMenu()
	return a
}

// Router 返回应用根路由（便于测试断言路径与历史栈）。
func (a *App) Router() *ui.Router {
	return a.router
}

// Store 返回应用数据存储。
func (a *App) Store() *store.Store {
	return a.store
}

// Close 在退出前把防抖中的编辑立即落盘。
func (a *App) Close() {
	if a.store != nil {
		_ = a.store.Close()
	}
}

// SetQuitFunc 供测试拦截应用退出（避免在单元测试中直接退出进程）。
func (a *App) SetQuitFunc(fn func()) {
	a.quitFn = fn
}

// OpenSettings 标记唤起全局设置弹窗，并可选指定目标 Tab（例如 "general", "about"）。
func (a *App) OpenSettings(tab ...string) {
	a.mu.Lock()
	a.settingsRequested = true
	if len(tab) > 0 && tab[0] != "" {
		a.settingsTab = tab[0]
	} else {
		a.settingsTab = "general"
	}
	a.mu.Unlock()

	a.dispatch(nil, nil)
}

// SetForceMacMenu 供测试强制启用 macOS 菜单分支（隐藏标题栏汉堡菜单）。
func (a *App) SetForceMacMenu(force bool) {
	a.mu.Lock()
	a.forceMacMenu = force
	a.mu.Unlock()
}

// SetForceWindowsMenu 供测试强制启用 Windows 自绘汉堡菜单分支。
func (a *App) SetForceWindowsMenu(force bool) {
	a.mu.Lock()
	a.forceWindowsMenu = force
	a.mu.Unlock()
}

// showWindowsMenu 判断是否在标题栏渲染 Windows 汉堡菜单按钮：
// macOS 环境下默认由系统顶部菜单栏承载，标题栏隐藏汉堡按钮；
// Windows/Linux 环境下默认保留标题栏汉堡菜单与 300px 浮层。
func (a *App) showWindowsMenu() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.forceWindowsMenu {
		return true
	}
	if a.forceMacMenu {
		return false
	}
	return runtime.GOOS != "darwin"
}

func (a *App) quit() {
	if a.quitFn != nil {
		a.quitFn()
		return
	}
	a.Close()
	mygo.App.Quit()
}

// activeWorkbenchRoute 构造当前活跃工作空间下的工作台路由；无工作空间时退回 Hub 大厅。
func (a *App) activeWorkbenchRoute(section string) string {
	if a.store != nil {
		if ws := a.store.Snapshot().ActiveWorkspace(); ws != nil {
			return "/w/" + string(ws.ID) + section
		}
	}
	return routes.HubWorkspaces
}

// View 绘制 RootLayout：常驻顶部 TitleBar + 分组 Layout 路由槽 + 常驻底部 StatusBar + 全局弹窗。
func (a *App) View(c *ui.Context) {
	c.SetTheme(theme.Apply(c.Theme(), c.Preferences()))

	root := ui.Column(c).Fill()
	settingsOpen := ui.Local(root, "global-settings-open", func() bool {
		return false
	})

	a.mu.Lock()
	if a.settingsRequested {
		*settingsOpen = true
		a.settingsRequested = false
	}
	activeTab := a.settingsTab
	a.mu.Unlock()

	root.Children(func() {
		a.titleBar(c, settingsOpen)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			a.content(c)
		})
		a.statusBar(c)
	})

	// 全局设置弹窗挂载在 RootLayout 层，在 (hub) 与 (workbench) 任意位置均可无损唤起。
	settings.Dialog(c, settingsOpen, activeTab)
}

// content 按路由前缀将页面分发到 (hub) 与 (workbench) 两套独立分组 Layout。
func (a *App) content(c *ui.Context) {
	tk := theme.Of(c)
	ui.Box(c).Grow(1).Background(tk.BgBase).Children(func() {
		a.router.View(c, func(r *ui.Route) {
			switch {
			// 分组 Layout 1：(hub) 全局组织与工作空间管理布局
			case r.Match(routes.HubPattern):
				a.hubLayout(c, r)

			// 分组 Layout 2：(workbench) 工作台沉浸式布局（左侧切换面板，右侧多 Tab 保活）
			case r.Match(routes.WorkbenchPattern):
				a.workbenchLayout(c, r)

			// 演示页
			case r.Match(routes.Demo):
				r.Title(locales.RailDemo())
				demo.View(c)

			default:
				a.router.Replace(routes.HubWorkspaces)
			}
		})
	})
}
