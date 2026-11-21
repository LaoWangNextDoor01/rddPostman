package history

import (
	"strings"
	"time"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/domain"
	"rddpostman/internal/logger"
	"rddpostman/internal/store"

	"github.com/egoist/mygo/ui"
)

type historyDayGroup struct {
	label   string
	entries []domain.HistoryEntry
}

// SidebarView 在 WorkbenchLayout 左侧面板内绘制紧凑的请求历史列表（对齐 Pixso 33-侧栏-请求历史）。
// 点击关联到集合请求的历史项时，通过 openRequest 在右侧常驻多 Tab 工作区打开对应请求。
func SidebarView(c *ui.Context, db *store.Store, wsID domain.ID, openRequest func(nodeID string)) {
	m, tk := thV2.Metrics, thV2.Of(c)

	col := ui.Column(c).Fill().Background(tk.BgApp)
	collapsed := ui.Local(col, "hist-collapsed", func() map[string]bool {
		return make(map[string]bool)
	})

	var entries []domain.HistoryEntry
	if db != nil && wsID != "" {
		entries, _ = db.History(wsID, 100)
	}

	col.Children(func() {
		// 顶部标题行：REQUEST HISTORY + 清空按钮
		ui.Row(c).FillWidth().Height(32).Padding(0, m.Space3).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "REQUEST HISTORY").FontSize(11).FontWeight(600).
				TextColor(tk.TextMuted).SingleLine().Grow(1)

			if len(entries) > 0 && db != nil {
				clearBtn := ui.ButtonBase(c.Key("wb-history-clear")).
					Size(22, 22).Radius(m.RadiusSM).Center().
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Clear history")
				if clearBtn.Hovered() {
					clearBtn.Background(tk.BgHover)
				}
				if clearBtn.Clicked() {
					if err := db.ClearHistory(wsID); err != nil {
						logger.Error("清空历史记录失败: %v", err)
					} else {
						c.Invalidate()
					}
				}
				clearBtn.Children(func() {
					ui.Icon(c, icons.Trash).FontSize(m.IconXS).TextColor(tk.TextMuted)
				})
			}
		})

		if len(entries) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Padding(m.Space4).Gap(m.Space2).Children(func() {
				ui.Icon(c, icons.Clock).FontSize(22).TextColor(tk.TextMuted)
				ui.Text(c, "No request history yet").FontSize(m.FontSM).FontWeight(500).TextColor(tk.TextSecondary)
				ui.Text(c, "Requests you send in this workspace will appear here.").
					FontSize(m.FontXS).TextColor(tk.TextMuted)
			})
			return
		}

		groups := groupHistoryByDay(entries, time.Now())
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(0, m.Space1, m.Space3, m.Space1).Gap(m.Space1).Children(func() {
				for _, g := range groups {
					group := g
					isClosed := (*collapsed)[group.label]

					hdr := ui.ButtonBase(c.Key("hist-day-"+group.label)).
						FillWidth().Height(26).Padding(0, m.Space2).Gap(m.Space15).
						Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.Start).
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(group.label)
					if hdr.Hovered() {
						hdr.Background(tk.BgHover)
					}
					if hdr.Clicked() {
						(*collapsed)[group.label] = !isClosed
					}
					hdr.Children(func() {
						arrow := "▾"
						if isClosed {
							arrow = "▸"
						}
						ui.Text(c, arrow).FontSize(10).TextColor(tk.TextMuted).Width(10)
						ui.Text(c, group.label).FontSize(m.FontXS).FontWeight(500).
							TextColor(tk.TextSecondary).SingleLine()
					})

					if isClosed {
						continue
					}

					for _, item := range group.entries {
						it := item
						method := strings.ToUpper(strings.TrimSpace(it.Method))
						if method == "" {
							method = "GET"
						}
						displayURL := strings.TrimSpace(it.URL)
						displayURL = strings.TrimPrefix(displayURL, "https://")
						displayURL = strings.TrimPrefix(displayURL, "http://")
						if displayURL == "" {
							displayURL = "(empty URL)"
						}

						line := ui.Row(c.Key("wb-hist-line-"+string(it.ID))).
							FillWidth().Height(28).Padding(0, 4).Gap(2).
							Radius(m.RadiusSM).AlignItems(ui.Center)
						menuOpen := ui.Local(line, "menu-open", func() bool { return false })
						hovered := line.Hovered() || *menuOpen
						if hovered {
							line.Background(tk.BgHover)
						}

						line.Children(func() {
							row := ui.ButtonBase(c.Key("wb-hist-item-"+string(it.ID))).
								Grow(1).FillHeight().Padding(0, 4).Gap(m.Space2).
								AlignItems(ui.Center).Justify(ui.Start).
								Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(method + " " + displayURL)
							if row.Clicked() {
								if it.RequestID != "" && openRequest != nil {
									openRequest(string(it.RequestID))
								}
							}
							row.Children(func() {
								ui.Text(c, method).FontSize(10).FontWeight(700).
									TextColor(sidebarMethodColor(tk, method)).Width(36).SingleLine()
								ui.Text(c, displayURL).FontSize(m.FontXS).
									TextColor(tk.TextPrimary).SingleLine().Grow(1)
							})

							// 悬停右侧操作（对齐 Pixso 34-侧栏-请求历史-悬停菜单：+ 与 ...）
							moreBtn := v2.IconButton(c, v2.IconButtonOpts{
								Key:      "wb-hist-more-" + string(it.ID),
								Label:    "History actions",
								Icon:     icons.Ellipsis,
								Size:     22,
								IconSize: 12,
							})
							if !hovered {
								moreBtn.Opacity(0).IgnoreInput(true)
							}
							if moreBtn.Clicked() {
								*menuOpen = !*menuOpen
							}
							v2.Popover(c, moreBtn, menuOpen, v2.PopoverOpts{Width: 164}, func(panel ui.Element) {
								ui.Column(c).Padding(4).Gap(2).Children(func() {
									if it.RequestID != "" && openRequest != nil {
										if v2.MenuButton(c, v2.MenuButtonProps{
											Icon: icons.Send, Key: "hist-open-" + string(it.ID), Label: "Open Request",
										}).Clicked() {
											openRequest(string(it.RequestID))
											*menuOpen = false
										}
									}
									if v2.MenuButton(c, v2.MenuButtonProps{
										Icon: icons.Copy, Key: "hist-copy-" + string(it.ID), Label: "Copy URL",
									}).Clicked() {
										c.Services().WriteClipboard(it.URL)
										*menuOpen = false
									}
								})
							})
						})
					}
				}
			})
		})
	})
}

func groupHistoryByDay(entries []domain.HistoryEntry, now time.Time) []historyDayGroup {
	todayY, todayM, todayD := now.Date()
	yestY, yestM, yestD := now.AddDate(0, 0, -1).Date()

	var groups []historyDayGroup
	index := make(map[string]int)
	for _, e := range entries {
		label := "Earlier"
		if !e.At.IsZero() {
			y, m, d := e.At.Local().Date()
			switch {
			case y == todayY && m == todayM && d == todayD:
				label = "Today"
			case y == yestY && m == yestM && d == yestD:
				label = "Yesterday"
			default:
				label = e.At.Local().Format("Jan 2")
			}
		}
		if idx, ok := index[label]; ok {
			groups[idx].entries = append(groups[idx].entries, e)
		} else {
			index[label] = len(groups)
			groups = append(groups, historyDayGroup{label: label, entries: []domain.HistoryEntry{e}})
		}
	}
	return groups
}

func sidebarMethodColor(tk thV2.Tokens, method string) ui.Color {
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
