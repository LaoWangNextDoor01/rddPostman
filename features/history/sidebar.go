package history

import (
	"fmt"
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
	search := ui.Local(col, "hist-sb-search", func() string { return "" })
	selectedID := ui.Local(col, "hist-sb-selected", func() domain.ID { return "" })
	clearOpen := ui.Local(col, "hist-clear-open", func() bool { return false })
	collapsed := ui.Local(col, "hist-collapsed", func() map[string]bool {
		return make(map[string]bool)
	})

	var entries []domain.HistoryEntry
	if db != nil && wsID != "" {
		entries, _ = db.History(wsID, 100)
	}

	kw := strings.ToLower(strings.TrimSpace(*search))

	col.Children(func() {
		// 1. 统一 34px 顶部搜索过滤条 + 清空历史按钮（与集合/环境/小工具侧栏完全齐平）
		ui.Row(c).FillWidth().Height(34).Padding(0, m.Space3).Gap(6).
			AlignItems(ui.Center).Children(func() {
			searchBox := ui.Row(c.Key("hist-sb-search-box")).Grow(1).Height(26).
				Padding(0, 8, 0, 6).Gap(6).AlignItems(ui.Center).
				Radius(m.RadiusMD).Background(tk.BgSubtle)
			searchBox.Children(func() {
				ui.Icon(c, icons.ListFilter).FontSize(13).TextColor(tk.TextMuted)
				ui.TextInputBase(c.Key("hist-sb-search-input"), search).Grow(1).
					FontSize(m.FontSM).TextColor(tk.TextPrimary).
					Placeholder("Search").PlaceholderColor(tk.TextMuted).
					Label("Filter history")
				if *search != "" {
					clr := v2.IconButton(c, v2.IconButtonOpts{
						Key:      "hist-sb-search-clear",
						Label:    "Clear filter",
						Icon:     icons.X,
						Size:     18,
						IconSize: 11,
						Color:    tk.TextMuted,
					})
					if clr.Clicked() {
						*search = ""
					}
				}
			})

			if len(entries) > 0 && db != nil {
				clearBtn := v2.IconButton(c, v2.IconButtonOpts{
					Key:      "wb-history-clear",
					Label:    "Clear history",
					Tooltip:  "Clear history",
					Icon:     icons.Trash,
					Size:     24,
					IconSize: 14,
					Radius:   m.RadiusSM,
					Color:    tk.TextMuted,
				})
				if clearBtn.Clicked() {
					*clearOpen = true
				}
			}
		})

		v2.ConfirmDialog(c, clearOpen, v2.ConfirmDialogOpts{
			Key:          "hist-clear-confirm",
			Title:        "清空请求历史",
			Description:  "确定要清空当前工作区的全部请求历史记录吗？此操作不可撤销。",
			ConfirmLabel: "确认清空",
			CancelLabel:  "取消",
			Danger:       true,
			OnConfirm: func() {
				if db == nil {
					return
				}
				if err := db.ClearHistory(wsID); err != nil {
					logger.Error("清空历史记录失败: %v", err)
				} else {
					c.Invalidate()
				}
			},
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

		var filtered []domain.HistoryEntry
		for _, e := range entries {
			if kw == "" ||
				strings.Contains(strings.ToLower(e.URL), kw) ||
				strings.Contains(strings.ToLower(e.Method), kw) {
				filtered = append(filtered, e)
			}
		}

		if len(filtered) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Padding(m.Space4).Gap(m.Space1).Children(func() {
				ui.Text(c, "No matching history").FontSize(m.FontXS).TextColor(tk.TextMuted)
			})
			return
		}

		groups := groupHistoryByDay(filtered, time.Now())
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(6, m.Space2, m.Space4, m.Space2).Gap(6).Children(func() {
				for _, g := range groups {
					group := g
					isOpen := kw != "" || !(*collapsed)[group.label]

					ui.Column(c).FillWidth().Gap(2).Children(func() {
						hdr := ui.ButtonBase(c.Key("hist-day-"+group.label)).
							FillWidth().Height(m.RowHeight).Padding(0, 8).Gap(5).
							Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.Start).
							Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(group.label).
							Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
						if hdr.Hovered() {
							hdr.Background(tk.BgSubtle)
						}
						if hdr.Clicked() {
							(*collapsed)[group.label] = isOpen
						}
						hdr.Children(func() {
							arrow := ui.Icon(c, icons.ChevronDown).FontSize(11).TextColor(tk.TextMuted)
							if !isOpen {
								arrow.Rotate(-90)
							}
							ui.Icon(c, icons.Clock).FontSize(13).TextColor(tk.TextSecondary)
							ui.Text(c, group.label).FontSize(m.FontSM).FontWeight(400).
								TextColor(tk.TextPrimary).SingleLine().Grow(1)
							ui.Text(c, fmt.Sprintf("%d", len(group.entries))).
								FontSize(10).TextColor(tk.TextMuted).SingleLine()
						})

						if !isOpen {
							return
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

							isSel := *selectedID == it.ID
							line := ui.Row(c.Key("wb-hist-line-"+string(it.ID))).
								FillWidth().Height(m.RowHeight).Padding(0, 4, 0, 22).Gap(2).
								Radius(m.RadiusSM).AlignItems(ui.Center).
								Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
							menuOpen := ui.Local(line, "menu-open", func() bool { return false })
							hovered := line.Hovered() || *menuOpen
							switch {
							case isSel:
								line.Background(tk.BgSelected)
							case hovered:
								line.Background(tk.BgHover)
							}

							line.Children(func() {
								row := ui.ButtonBase(c.Key("wb-hist-item-"+string(it.ID))).
									Grow(1).FillHeight().Gap(6).
									AlignItems(ui.Center).Justify(ui.Start).
									Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(method + " " + displayURL)
								if row.Clicked() {
									*selectedID = it.ID
									if it.RequestID != "" && openRequest != nil {
										openRequest(string(it.RequestID))
									}
								}
								row.Children(func() {
									ui.Text(c, method).FontSize(10).FontWeight(700).
										TextColor(sidebarMethodColor(tk, method)).Width(36).SingleLine()
									weight := 400
									if isSel {
										weight = 500
									}
									ui.Text(c, displayURL).FontSize(m.FontSM).FontWeight(weight).
										TextColor(tk.TextPrimary).SingleLine().Grow(1)
								})

								moreBtn := v2.IconButton(c, v2.IconButtonOpts{
									Key:      "wb-hist-more-" + string(it.ID),
									Label:    "History actions",
									Icon:     icons.Ellipsis,
									Size:     20,
									IconSize: 12,
									Radius:   m.RadiusSM,
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
					})
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
