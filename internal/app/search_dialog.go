package app

import (
	"fmt"
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/features/collection"
	"rddpostman/features/settings"
	"rddpostman/features/tools"
	"rddpostman/internal/domain"
	"rddpostman/internal/routes"

	"github.com/egoist/mygo/ui"
)

type searchResultItem struct {
	key      string
	kind     string // "command" | "request" | "collection" | "environment" | "workspace"
	badge    string
	title    string
	subtitle string
	shortcut string
	icon     *ui.SVG
	run      func()
}

// globalSearchDialog 绘制全局快速搜索与命令面板（Cmd/Ctrl+K 与 Cmd/Ctrl+Shift+P）。
func (a *App) globalSearchDialog(
	c *ui.Context,
	open *bool,
	query *string,
	focusInput *bool,
	settingsOpen, globalsOpen, exportOpen *bool,
) {
	if open == nil {
		return
	}
	m, tk := thV2.Metrics, thV2.Of(c)

	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		selIdx := ui.Local(panel, "search-sel-idx", func() int { return 0 })

		rawQuery := strings.TrimSpace(*query)
		commandOnly := strings.HasPrefix(rawQuery, ">")
		kw := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(rawQuery, ">")))

		items := a.buildSearchItems(kw, commandOnly, open, settingsOpen, globalsOpen, exportOpen)
		if len(items) == 0 {
			*selIdx = 0
		} else if *selIdx >= len(items) {
			*selIdx = len(items) - 1
		} else if *selIdx < 0 {
			*selIdx = 0
		}

		if panel.Shortcut(0, ui.KeyDown) && len(items) > 0 {
			*selIdx = (*selIdx + 1) % len(items)
		}
		if panel.Shortcut(0, ui.KeyUp) && len(items) > 0 {
			*selIdx = (*selIdx - 1 + len(items)) % len(items)
		}
		if panel.Shortcut(0, ui.KeyEscape) {
			*open = false
		}

		backdrop.Background(tk.Overlay)
		panel.Width(600).MaxHeight(460).Radius(m.RadiusLG).
			Background(tk.BgSurface).
			Border(m.Border, tk.BorderPopover).
			Shadow(0, 14, 36, 0, tk.Overlay).
			Clip()

		panel.Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				// 1. 顶部搜索输入框
				ui.Row(c).FillWidth().Height(46).Padding(0, m.Space4).Gap(m.Space2).
					AlignItems(ui.Center).
					BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
					ui.Icon(c, icons.Search).FontSize(m.IconSM).TextColor(tk.TextMuted)
					in := ui.TextInputBase(c.Key(keyGlobalSearchInput), query).
						Grow(1).FillHeight().
						FontSize(m.FontMD).TextColor(tk.TextPrimary).
						Placeholder("Search requests, collections, workspaces, or type > for commands...").
						PlaceholderColor(tk.TextMuted).
						Label("Global search input")
					if focusInput != nil && *focusInput {
						in.Focus()
						*focusInput = false
					}
					if in.Changed() {
						*selIdx = 0
					}
					if in.Submitted() && len(items) > 0 && *selIdx >= 0 && *selIdx < len(items) {
						items[*selIdx].run()
					}
					if in.Shortcut(0, ui.KeyDown) && len(items) > 0 {
						*selIdx = (*selIdx + 1) % len(items)
					}
					if in.Shortcut(0, ui.KeyUp) && len(items) > 0 {
						*selIdx = (*selIdx - 1 + len(items)) % len(items)
					}
					if in.Shortcut(0, ui.KeyEscape) {
						*open = false
					}

					ui.Box(c).Padding(2, 6).Radius(4).Background(tk.BgSubtle).Children(func() {
						ui.Text(c, "ESC").FontSize(10).TextColor(tk.TextMuted)
					})
				})

				// 2. 结果列表
				ui.Scroll(c).MaxHeight(360).FillWidth().Children(func() {
					ui.Column(c).FillWidth().Padding(m.Space2).Gap(2).Children(func() {
						if len(items) == 0 {
							ui.Column(c).FillWidth().Padding(28, m.Space4).Center().Gap(m.Space1).Children(func() {
								ui.Text(c, "No matching results").FontSize(m.FontSM).FontWeight(500).
									TextColor(tk.TextSecondary)
								ui.Text(c, "Try searching by request name, URL, workspace, or prefix with > for commands.").
									FontSize(m.FontXS).TextColor(tk.TextMuted)
							})
							return
						}
						for idx, it := range items {
							item := it
							isSel := idx == *selIdx
							row := ui.ButtonBase(c.Key("gsearch-"+item.key)).
								FillWidth().Height(36).Padding(0, 10).Gap(8).
								Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.Start).
								Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(item.title)
							if isSel {
								row.Background(tk.AccentSoft)
							} else if row.Hovered() {
								row.Background(tk.BgHover)
							}
							if row.Clicked() {
								item.run()
							}
							row.Children(func() {
								if item.badge != "" {
									ui.Text(c, item.badge).FontSize(10).Bold().
										TextColor(searchMethodColor(tk, item.badge)).Width(44).SingleLine()
								} else if item.icon != nil {
									ui.Box(c).Width(20).Center().Children(func() {
										ui.Icon(c, item.icon).FontSize(13).TextColor(tk.TextSecondary)
									})
								}
								ui.Column(c).Grow(1).Gap(1).Children(func() {
									ui.Text(c, item.title).FontSize(m.FontSM).FontWeight(500).
										TextColor(tk.TextPrimary).SingleLine()
									if item.subtitle != "" {
										ui.Text(c, item.subtitle).FontSize(11).
											TextColor(tk.TextMuted).SingleLine()
									}
								})
								if item.shortcut != "" {
									ui.Box(c).Padding(2, 6).Radius(4).Background(tk.BgSubtle).Children(func() {
										ui.Text(c, item.shortcut).FontSize(10).TextColor(tk.TextMuted).SingleLine()
									})
								}
							})
						}
					})
				})

				// 3. 底部提示条
				ui.Row(c).FillWidth().Height(28).Padding(0, m.Space4).Gap(m.Space3).
					AlignItems(ui.Center).Background(tk.BgApp).
					BorderWidth(m.Border, 0, 0, 0).BorderColor(tk.Border).Children(func() {
					ui.Text(c, "↑↓ Navigate").FontSize(10).TextColor(tk.TextMuted)
					ui.Text(c, "↵ Open").FontSize(10).TextColor(tk.TextMuted)
					ui.Text(c, "> Commands").FontSize(10).TextColor(tk.TextMuted)
					ui.Spacer(c)
					ui.Text(c, fmt.Sprintf("%d results", len(items))).FontSize(10).TextColor(tk.TextMuted)
				})
			})
		})
	})
}

func (a *App) buildSearchItems(
	kw string,
	commandOnly bool,
	open, settingsOpen, globalsOpen, exportOpen *bool,
) []searchResultItem {
	var out []searchResultItem
	match := func(fields ...string) bool {
		if kw == "" {
			return true
		}
		for _, f := range fields {
			if strings.Contains(strings.ToLower(f), kw) {
				return true
			}
		}
		return false
	}

	// 命令列表（支持 > 命令面板或关键词直接检索）
	commands := []searchResultItem{
		{
			key: "cmd-new-req", kind: "command", icon: icons.Send,
			title: "New Request", subtitle: "Create a draft HTTP request in the active workspace",
			shortcut: shortcutLabel(ui.KeyN),
			run: func() {
				*open = false
				a.triggerNewRequest()
			},
		},
		{
			key: "cmd-new-coll", kind: "command", icon: icons.FolderPlus,
			title: "New Collection", subtitle: "Create a new collection tab in the active workspace",
			shortcut: shortcutLabel(ui.KeyB),
			run: func() {
				*open = false
				a.triggerNewCollection()
			},
		},
		{
			key: "cmd-new-ws", kind: "command", icon: icons.LayoutGrid,
			title: "New Workspace", subtitle: "Create a new workspace",
			run: func() {
				*open = false
				a.router.Push(routes.HubWorkspaceNew)
			},
		},
		{
			key: "cmd-import", kind: "command", icon: icons.Upload,
			title: "Import Postman Collection", subtitle: "Import Postman v2.0/v2.1 JSON",
			shortcut: shortcutLabel(ui.KeyO),
			run: func() {
				*open = false
				a.triggerImportPostman()
			},
		},
		{
			key: "cmd-import-curl", kind: "command", icon: icons.Code,
			title: "Import cURL", subtitle: "Parse a cURL command into a request",
			run: func() {
				*open = false
				a.triggerImportCurl()
			},
		},
		{
			key: "cmd-export", kind: "command", icon: icons.Download,
			title: "Export Postman JSON", subtitle: "Export a collection or environment as JSON",
			run: func() {
				*open = false
				if exportOpen != nil {
					*exportOpen = true
				}
			},
		},
		{
			key: "cmd-globals", kind: "command", icon: icons.Globe,
			title: "Edit Globals", subtitle: "Manage global variables across all workspaces",
			run: func() {
				*open = false
				if globalsOpen != nil {
					*globalsOpen = true
				}
			},
		},
		{
			key: "cmd-settings", kind: "command", icon: icons.Settings,
			title: "Open Settings", subtitle: "Appearance, language, shortcuts, and data settings",
			shortcut: shortcutLabel(ui.KeyComma),
			run: func() {
				*open = false
				if settingsOpen != nil {
					*settingsOpen = true
				}
			},
		},
		{
			key: "cmd-toggle-theme", kind: "command", icon: icons.Sparkles,
			title: "Toggle Dark / Light Theme", subtitle: "Switch application appearance immediately",
			run: func() {
				*open = false
				if a.ThemeMode() == settings.ThemeDark {
					a.SetThemeMode(settings.ThemeLight)
				} else {
					a.SetThemeMode(settings.ThemeDark)
				}
			},
		},
		{
			key: "cmd-workspaces", kind: "command", icon: icons.LayoutGrid,
			title: "View All Workspaces", subtitle: "Go to Hub workspaces management",
			run: func() {
				*open = false
				a.router.Push(routes.HubWorkspaces)
			},
		},
	}

	if commandOnly {
		for _, cmd := range commands {
			if match(cmd.title, cmd.subtitle) {
				out = append(out, cmd)
			}
		}
		return out
	}

	// 1. 数据检索：Requests -> Collections -> Environments -> Workspaces
	if a.store != nil {
		snap := a.store.Snapshot()
		for _, ws := range snap.Workspaces {
			wsCopy := ws
			// Requests & Collections
			for _, coll := range wsCopy.Collections {
				collCopy := coll
				var walk func(nodes []*domain.Node)
				walk = func(nodes []*domain.Node) {
					for _, n := range nodes {
						node := n
						if node.Kind == domain.KindRequest && node.Request != nil {
							if match(node.Name, node.Request.URL, node.Request.Method, collCopy.Name, wsCopy.Name) {
								out = append(out, searchResultItem{
									key:      "req-" + string(node.ID),
									kind:     "request",
									badge:    strings.ToUpper(node.Request.Method),
									title:    node.Name,
									subtitle: fmt.Sprintf("%s / %s · %s", wsCopy.Name, collCopy.Name, node.Request.URL),
									run: func() {
										*open = false
										a.openWorkbenchTarget(wsCopy.ID, routes.SectionCollections, collection.WorkbenchCommand{
											Kind:     collection.CmdOpenTab,
											TargetID: string(node.ID),
										})
									},
								})
							}
						} else if len(node.Children) > 0 {
							walk(node.Children)
						}
					}
				}
				walk(collCopy.Items)

				if match(collCopy.Name, wsCopy.Name) {
					out = append(out, searchResultItem{
						key:      "coll-" + string(collCopy.ID),
						kind:     "collection",
						icon:     icons.Layers,
						title:    collCopy.Name,
						subtitle: "Collection · " + wsCopy.Name,
						run: func() {
							*open = false
							a.openWorkbenchTarget(wsCopy.ID, routes.SectionCollections, collection.WorkbenchCommand{
								Kind:     collection.CmdOpenTab,
								TargetID: string(collCopy.ID),
							})
						},
					})
				}
			}

			// Environments
			for _, env := range wsCopy.Environments {
				envCopy := env
				if match(envCopy.Name, wsCopy.Name) {
					out = append(out, searchResultItem{
						key:      "env-" + string(envCopy.ID),
						kind:     "environment",
						icon:     icons.Globe,
						title:    envCopy.Name,
						subtitle: "Environment · " + wsCopy.Name,
						run: func() {
							*open = false
							a.openWorkbenchTarget(wsCopy.ID, routes.SectionEnvironments, collection.WorkbenchCommand{
								Kind:  collection.CmdOpenEnv,
								EnvID: envCopy.ID,
							})
						},
					})
				}
			}

			// Workspaces
			if match(wsCopy.Name, wsCopy.Description) {
				out = append(out, searchResultItem{
					key:      "ws-" + string(wsCopy.ID),
					kind:     "workspace",
					icon:     icons.LayoutGrid,
					title:    wsCopy.Name,
					subtitle: "Workspace",
					run: func() {
						*open = false
						_ = a.store.SetActiveWorkspace(wsCopy.ID)
						a.router.Push(routes.WorkbenchCollectionsOf(string(wsCopy.ID)))
					},
				})
			}
		}
	}

	// 2. 开发者小工具快速检索（搜 md5 / base64 / json / jwt 等直达工作台工具 Tab）
	for _, def := range tools.All() {
		td := def
		if match(td.Name, td.ShortName, td.Description, td.Keywords, td.Category) {
			out = append(out, searchResultItem{
				key:      "tool-" + td.ID,
				kind:     "tool",
				icon:     td.Icon,
				title:    td.Name,
				subtitle: "Dev Tool · " + td.Description,
				run: func() {
					*open = false
					a.openWorkbenchTarget("", routes.SectionTools, collection.WorkbenchCommand{
						Kind:   collection.CmdOpenTool,
						ToolID: td.ID,
					})
				},
			})
		}
	}

	// 3. 追加匹配的快捷命令
	for _, cmd := range commands {
		if match(cmd.title, cmd.subtitle) {
			out = append(out, cmd)
		}
	}

	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func searchMethodColor(tk thV2.Tokens, method string) ui.Color {
	switch strings.ToUpper(method) {
	case "GET":
		return tk.MethodGet
	case "POST":
		return tk.MethodPost
	case "PUT":
		return tk.MethodPut
	case "DELETE":
		return tk.MethodDelete
	case "PATCH":
		return tk.MethodPatch
	default:
		return tk.TextMuted
	}
}
