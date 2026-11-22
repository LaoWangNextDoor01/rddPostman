package app

import (
	"runtime"

	"rddpostman/internal/routes"

	"github.com/egoist/mygo"
)

// InitPlatformMenu 初始化跨平台系统菜单：
// - macOS: 注册原生顶部系统菜单栏（AppMenu、File、Edit、View、Window、Help）；
// - Windows/Linux: 不注册全局菜单栏，保持标题栏内自绘汉堡菜单与 300px 浮层。
func (a *App) InitPlatformMenu() {
	if runtime.GOOS == "darwin" || a.isMacMenuForced() {
		mygo.App.SetName("rddPostman")
		mygo.App.SetMenu(a.buildMacMenu())
	}
}

func (a *App) isMacMenuForced() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.forceMacMenu
}

// buildMacMenu 构建 macOS 原生应用菜单栏。
func (a *App) buildMacMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		// 1. App 菜单（以应用名命名的系统菜单）
		{
			Role: mygo.RoleAppMenu,
			Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleAbout},
				mygo.Separator(),
				{
					ID:          "settings",
					Label:       "Settings...",
					Accelerator: "Cmd+,",
					Click: func(_ *mygo.MenuItem, win *mygo.Window) {
						a.dispatch(win, func() {
							a.OpenSettings("general")
						})
					},
				},
				mygo.Separator(),
				{Role: mygo.RoleServices},
				mygo.Separator(),
				{Role: mygo.RoleHide},
				{Role: mygo.RoleHideOthers},
				{Role: mygo.RoleUnhide},
				mygo.Separator(),
				{Role: mygo.RoleQuit},
			},
		},

		// 2. File 菜单：创建、工作区、导入导出与关闭
		{
			Label: "File",
			Submenu: []*mygo.MenuItem{
				{
					ID:          "new_request",
					Label:       "New Request",
					Accelerator: "Cmd+N",
					Click: func(_ *mygo.MenuItem, win *mygo.Window) {
						a.dispatch(win, func() {
							a.router.Push(a.activeWorkbenchRoute(routes.SectionCollections))
						})
					},
				},
				{
					ID:          "new_collection",
					Label:       "New Collection",
					Accelerator: "Shift+Cmd+N",
					Click: func(_ *mygo.MenuItem, win *mygo.Window) {
						a.dispatch(win, func() {
							a.router.Push(a.activeWorkbenchRoute(routes.SectionCollections))
						})
					},
				},
				{
					ID:    "new_workspace",
					Label: "New Workspace",
					Click: func(_ *mygo.MenuItem, win *mygo.Window) {
						a.dispatch(win, func() {
							a.router.Push(routes.HubWorkspaceNew)
						})
					},
				},
				mygo.Separator(),
				{
					ID:          "import",
					Label:       "Import...",
					Accelerator: "Cmd+O",
					Click: func(_ *mygo.MenuItem, win *mygo.Window) {
						a.dispatch(win, func() {
							a.router.Push(a.activeWorkbenchRoute(routes.SectionCollections))
						})
					},
				},
				{
					ID:    "export",
					Label: "Export...",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						// 导出占位
					},
				},
				mygo.Separator(),
				{
					Role: mygo.RoleClose,
				},
			},
		},

		// 3. Edit 菜单：系统原生编辑能力（撤销、重做、剪切、复制、粘贴、全选）
		{Role: mygo.RoleEditMenu},

		// 4. View 菜单：重载、开发者工具、缩放与全屏切换
		{Role: mygo.RoleViewMenu},

		// 5. Window 菜单：最小化、缩放、置于顶层
		{Role: mygo.RoleWindowMenu},

		// 6. Help 菜单：帮助与关于
		{
			Label: "Help",
			Submenu: []*mygo.MenuItem{
				{
					ID:    "help",
					Label: "rddPostman Help",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						// 帮助占位
					},
				},
				{
					ID:    "about_postman",
					Label: "About Postman",
					Click: func(_ *mygo.MenuItem, win *mygo.Window) {
						a.dispatch(win, func() {
							a.OpenSettings("about")
						})
					},
				},
			},
		},
	})
}

// dispatch 在 UI 主线程执行操作并请求重绘。
func (a *App) dispatch(win *mygo.Window, fn func()) {
	if win == nil {
		win = mygo.FocusedWindow()
		if win == nil {
			if wins := mygo.Windows(); len(wins) > 0 {
				win = wins[0]
			}
		}
	}
	if win != nil {
		win.Update(func() {
			if fn != nil {
				fn()
			}
		})
	} else if fn != nil {
		fn()
	}
}
