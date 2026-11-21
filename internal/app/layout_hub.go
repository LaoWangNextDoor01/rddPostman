package app

import (
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/features/workspace"
	"rddpostman/internal/routes"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

const (
	hubMinSidebarWidth     float32 = 180
	hubMaxSidebarWidth     float32 = 460
	hubDefaultSidebarWidth float32 = 232
)

// hubLayout 实现 (hub) 分组布局：左侧常驻团队/管理菜单，右侧通过 r.View 承载子路由页面。
// 在 /hub/* 子路由间切换（如 Workspaces 列表 ↔ 创建页 ↔ API Catalog）时，左侧菜单宽度与状态完全保活。
func (a *App) hubLayout(c *ui.Context, r *ui.Route) {
	root := ui.Box(c.Key(keyLayoutHub)).Fill()
	width := ui.Local(root, "hub-sidebar-width", func() float32 {
		return hubDefaultSidebarWidth
	})
	if *width < hubMinSidebarWidth {
		*width = hubMinSidebarWidth
	} else if *width > hubMaxSidebarWidth {
		*width = hubMaxSidebarWidth
	}

	root.Children(func() {
		ui.Split(c, width, func() {
			a.hubSidebar(c)
		}, func() {
			r.View(c, func(sub *ui.Route) {
				switch {
				case sub.Match(routes.SubWorkspaces), sub.Match("/"):
					sub.Title(locales.WorkspaceTitle())
					workspace.ListView(c, a.store, a.router)
				case sub.Match(routes.SubWorkspaceNew):
					sub.Title("Create Workspace")
					workspace.CreateView(c, a.store, a.router)
				case sub.Match(routes.SubHome):
					sub.Title("Home")
					hubPlaceholder(c, "Home", "Recent workspaces and team activity will appear here.", icons.House)
				case sub.Match(routes.SubInventory):
					sub.Title("Application Inventory")
					hubPlaceholder(c, "Application Inventory", "Discover and track applications across your team.", icons.Boxes)
				case sub.Match(routes.SubAPICatalog):
					sub.Title("API Catalog")
					hubPlaceholder(c, "API Catalog", "Browse and manage your organization's API catalog.", icons.BookOpen)
				case sub.Match(routes.SubPrivateNetwork):
					sub.Title("Private API Network")
					hubPlaceholder(c, "Private API Network", "Share internal APIs, collections, and workspaces with your team.", icons.Lock)
				case sub.Match(routes.SubPublicNetwork):
					sub.Title("Public API Network")
					hubPlaceholder(c, "Public API Network", "Explore and publish public APIs for the community.", icons.Globe)
				case sub.Match(routes.SubAgentContext):
					sub.Title("Agent Context")
					hubPlaceholder(c, "Agent Context", "Configure context and tools for AI agents.", icons.Sparkles)
				case sub.Match(routes.SubIntegrations):
					sub.Title("Integrations")
					hubPlaceholder(c, "Integrations", "Connect third-party services and automate workflows.", icons.Plug)
				case sub.Match(routes.SubReports):
					sub.Title("Reports")
					hubPlaceholder(c, "Reports", "Review API usage, test runs, and workspace metrics.", icons.ChartColumn)
				default:
					sub.Title(locales.WorkspaceTitle())
					workspace.ListView(c, a.store, a.router)
				}
			})
		}).Fill().SplitReset(hubDefaultSidebarWidth)
	})
}

func (a *App) hubSidebar(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	curPath := a.router.Path()

	ui.Column(c).Fill().MinWidth(hubMinSidebarWidth).Background(tk.BgBase).Children(func() {
		// 1. 团队信息栏 (48px)
		ui.Row(c).FillWidth().Height(48).AlignItems(ui.Center).Padding(m.Space2, m.Space3).Gap(m.Space2).Children(func() {
			ui.Avatar(c, "W", nil).Size(26, 26).Border(1, tk.BorderPopover).
				Background(tk.BgSubtle).TextColor(tk.TextSecondary)

			ui.Column(c).Children(func() {
				ui.Text(c, "wang's Team").FontSize(13).FontWeight(600).TextColor(tk.TextPrimary).SingleLine()
				ui.Text(c, "1 member").FontSize(m.FontXS).TextColor(tk.TextMuted).SingleLine()
			})

			ui.Spacer(c)

			inviteBtn := ui.ButtonBase(c.Key("hub-team-invite")).
				Height(22).PaddingX(8).Radius(m.RadiusSM).
				Border(m.Border, tk.BorderPopover).
				Cursor(ui.CursorPointer).
				Role(ui.RoleButton).
				Label("Invite").
				AlignItems(ui.Center).
				Justify(ui.Center).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			if inviteBtn.Hovered() {
				inviteBtn.Background(tk.BgHover).TextColor(tk.TextPrimary)
			} else {
				inviteBtn.TextColor(tk.TextSecondary)
			}
			inviteBtn.Children(func() {
				ui.Text(c, "Invite").FontSize(m.FontXS).SingleLine()
			})
		})

		ui.Divider(c).BorderColor(tk.BgSubtle).Margin(0, m.Space3, m.Space15, m.Space3)

		// 2. 分组导航菜单（基于路由高亮，点击直接跳转对应子路由）
		ui.Column(c).PaddingX(m.Space2).Children(func() {
			menuItem := func(key, label, target string, icon *ui.SVG) {
				selected := curPath == target || strings.HasPrefix(curPath, target+"/")
				btn := v2.MenuButton(c, v2.MenuButtonProps{
					Icon:     icon,
					Key:      "hub-nav-" + key,
					Label:    label,
					Selected: selected,
				})
				if btn.Clicked() {
					a.router.Push(target)
				}
			}

			// Group 1
			menuItem("home", "Home", routes.HubHome, icons.House)
			menuItem("workspaces", "Workspaces", routes.HubWorkspaces, icons.LayoutGrid)
			menuItem("inventory", "Application Inventory", routes.HubInventory, icons.Boxes)
			menuItem("catalog", "API Catalog", routes.HubAPICatalog, icons.BookOpen)

			ui.Divider(c).BorderColor(tk.BgSubtle).MarginX(m.Space1).MarginY(m.Space15)

			// Group 2
			menuItem("private-network", "Private API Network", routes.HubPrivateNetwork, icons.Lock)
			menuItem("public-network", "Public API Network", routes.HubPublicNetwork, icons.Globe)

			ui.Divider(c).BorderColor(tk.BgSubtle).MarginX(m.Space1).MarginY(m.Space15)

			// Group 3
			menuItem("agent-context", "Agent Context", routes.HubAgentContext, icons.Sparkles)
			menuItem("integrations", "Integrations", routes.HubIntegrations, icons.Plug)
			menuItem("reports", "Reports", routes.HubReports, icons.ChartColumn)
		})

		// 3. 弹性空白
		ui.Spacer(c)

		// 4. 底部帮助链接
		hubBottomLinks(c)
	})
}

func hubBottomLinks(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	links := []string{
		"What is Postman",
		"How to Publish",
		"Learning Center",
		"Support Center",
		"Postman Enterprise",
	}

	ui.Column(c).Padding(0, m.Space4, 14, m.Space4).Gap(9).Children(func() {
		for _, title := range links {
			t := title
			link := ui.ButtonBase(c.Key("hub-link-" + t)).
				Role(ui.RoleLink).
				Cursor(ui.CursorPointer).
				Label(t).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast}).
				Justify(ui.Start)

			if link.Hovered() {
				link.TextColor(tk.TextPrimary)
			} else {
				link.TextColor(tk.TextMuted)
			}

			link.Children(func() {
				ui.Text(c, t).FontSize(m.FontMicro).SingleLine()
			})
		}
	})
}

func hubPlaceholder(c *ui.Context, title, desc string, icon *ui.SVG) {
	m, tk := thV2.Metrics, thV2.Of(c)

	ui.Column(c).Fill().Padding(22, m.Space6).Background(tk.BgBase).Children(func() {
		ui.Text(c, title).FontSize(m.FontXXL).Bold().TextColor(tk.TextPrimary).SingleLine()

		ui.Column(c).Grow(1).FillWidth().Center().Gap(m.Space2).Children(func() {
			ui.Box(c).Size(48, 48).Radius(m.RadiusFull).Background(tk.BgSubtle).Center().Children(func() {
				ui.Icon(c, icon).FontSize(22).TextColor(tk.TextMuted)
			})
			ui.Text(c, title).FontSize(m.FontLG).FontWeight(600).TextColor(tk.TextPrimary).SingleLine()
			ui.Text(c, desc).FontSize(m.FontSM).TextColor(tk.TextSecondary)
		})
	})
}
