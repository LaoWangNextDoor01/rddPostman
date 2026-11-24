package app

import (
	"runtime"

	"rddpostman/internal/routes"
	"rddpostman/locales"

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
					Label:       locales.MenuSettings(),
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
			Label: locales.MenuFile(),
			Submenu: []*mygo.MenuItem{
				{
					ID:          "new_request",
					Label:       locales.MenuNewRequest(),
					Accelerator: "Cmd+N",
					Click: func(_ *mygo.MenuItem, win *mygo.Window) {
						a.dispatch(win, func() {
							a.triggerNewRequest()
						})
					},
				},
				{
					ID:          "new_collection",
					Label:       locales.MenuNewCollection(),
					Accelerator: "Shift+Cmd+N",
					Click: func(_ *mygo.MenuItem, win *mygo.Window) {
						a.dispatch(win, func() {
							a.triggerNewCollection()
						})
					},
				},
				{
					ID:    "new_workspace",
					Label: locales.MenuNewWorkspace(),
					Click: func(_ *mygo.MenuItem, win *mygo.Window) {
						a.dispatch(win, func() {
							a.router.Push(routes.HubWorkspaceNew)
						})
					},
				},
				mygo.Separator(),
				{
					ID:          "import",
					Label:       locales.MenuImport(),
					Accelerator: "Cmd+O",
					Click: func(_ *mygo.MenuItem, win *mygo.Window) {
						a.dispatch(win, func() {
							a.triggerImportPostman()
						})
					},
				},
				{
					ID:    "export",
					Label: locales.MenuExport(),
					Click: func(_ *mygo.MenuItem, win *mygo.Window) {
						a.dispatch(win, func() {
							a.OpenExport()
						})
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
			Label: locales.MenuHelp(),
			Submenu: []*mygo.MenuItem{
				{
					ID:    "help",
					Label: locales.MenuAppHelp(),
					Click: func(_ *mygo.MenuItem, win *mygo.Window) {
						a.dispatch(win, func() {
							a.OpenSettings("shortcuts")
						})
					},
				},
				{
					ID:    "about_postman",
					Label: locales.MenuAbout(),
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
