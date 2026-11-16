package collection

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"rddpostman/components/theme"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// reqTitleState 维护请求名称的面包屑行内编辑态。
type reqTitleState struct {
	id      string
	editing bool
	fresh   bool
	draft   string
}

// reqSessionState 维护请求 tab 的运行时会话状态（Tab 选中、下拉展开、临时输入草稿与拆分高度）。
type reqSessionState struct {
	id           string
	reqTab       int     // 0: Params, 1: Authorization, 2: Headers, 3: Body, 4: Settings
	respTab      int     // 0: Body, 1: Headers, 2: Cookies
	methodOpen   bool    // 请求方式下拉菜单开关
	authTypeOpen bool    // 认证方式下拉菜单开关
	urlDraft     string  // URL 本地编辑草稿
	bodyRawDraft string  // Raw 请求体本地编辑草稿
	splitHeight  float32 // 请求区与响应区的上下分栏高度

	// 响应区展示状态
	hasResponse  bool
	respStatus   int
	respDuration string
	respSize     string
	respBody     string
}

func defaultSessionState(id string, req *domain.Request) reqSessionState {
	url := ""
	rawBody := ""
	if req != nil {
		url = req.URL
		rawBody = req.Body.Raw
	}
	defaultResp := "{\n  \"status\": \"success\",\n  \"code\": 200,\n  \"message\": \"Request processed successfully\",\n  \"data\": {\n    \"requestId\": \"req_88f921a4\",\n    \"timestamp\": 1731750000,\n    \"service\": \"rddPostman Engine v1.0\",\n    \"client\": \"Desktop / Windows\"\n  }\n}"
	return reqSessionState{
		id:           id,
		reqTab:       0,
		respTab:      0,
		urlDraft:     url,
		bodyRawDraft: rawBody,
		splitHeight:  340,
		hasResponse:  true,
		respStatus:   200,
		respDuration: "142 ms",
		respSize:     "1.85 KB",
		respBody:     defaultResp,
	}
}

// requestTab 画完整的请求详情区：顶部可编辑面包屑、请求行、可编辑参数/认证/头/体、响应区与纵向拖拽分栏。
func requestTab(c *ui.Context, props pageProps, coll *domain.Collection, n *domain.Node, activeID string) {
	m, tk := theme.Metrics, theme.Of(c)
	if n == nil || n.Request == nil {
		return
	}
	req := n.Request

	root := ui.Column(c).Grow(1).Background(tk.BgSurface)

	title := ui.Local(root, "req-title-"+activeID, func() reqTitleState {
		return reqTitleState{id: activeID, draft: n.Name}
	})
	if title.id != activeID {
		*title = reqTitleState{id: activeID, draft: n.Name}
	}

	state := ui.Local(root, "req-session-"+activeID, func() reqSessionState {
		return defaultSessionState(activeID, req)
	})
	if state.id != activeID {
		*state = defaultSessionState(activeID, req)
	}

	// 同步外部变更
	if !title.editing && title.draft != n.Name {
		title.draft = n.Name
	}

	root.Children(func() {
		// 5. 请求体和响应体做成可调整大小（SplitVertical）
		ui.SplitVertical(c, &state.splitHeight, func() {
			// 上半部：请求编辑区
			ui.Column(c).Fill().Padding(m.Space2, m.Space4, 0, m.Space4).Children(func() {
				// 1. 顶部面包屑导航（最后一项可点击进入编辑模式）
				reqBreadcrumb(c, props, n, activeID, title, tk, m)

				ui.Box(c).Height(m.Space2)

				// 2. 请求类型（下拉 Dropdown）+ URL 地址输入栏 + 发送按钮
				reqLine(c, props, coll, n, activeID, state, tk, m)

				ui.Box(c).Height(m.Space2)

				// 3. 请求配置 Tab 切换（Params, Auth, Headers, Body, Settings）
				reqConfigTabs(c, props, coll, n, activeID, state, tk, m)
			})
		}, func() {
			// 下半部：响应区（支持 Body, Headers, Cookies Tab 切换）
			responsePane(c, props, n, state, tk, m)
		}).Fill().SplitReset(340)
	})
}

// 1. 面包屑导航与可编辑请求名称
func reqBreadcrumb(c *ui.Context, props pageProps, n *domain.Node, activeID string,
	title *reqTitleState, tk theme.Tokens, m theme.Metric) {
	crumbs := nodeTrail(props.collections, activeID)

	ui.Row(c).Height(26).Gap(m.Space1).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icons.Get("folder")).FontSize(11).TextColor(tk.TextMuted)

		for i, cr := range crumbs {
			cr := cr
			isLast := i == len(crumbs)-1
			if i > 0 {
				ui.Text(c, "/").FontSize(m.FontXS).TextColor(tk.TextMuted).Margin(0, 1)
			}

			if !isLast {
				btn := ui.ButtonBase(c.Key("req-crumb-" + activeID + "-" + cr.ID)).
					Height(20).PaddingX(4).Radius(m.RadiusSM).AlignItems(ui.Center)
				if btn.Hovered() {
					btn.Background(tk.HoverSurface)
				}
				if btn.Clicked() {
					props.actions.openTab(cr.ID)
				}
				btn.Children(func() {
					color := tk.TextMuted
					if btn.Hovered() {
						color = tk.TextPrimary
					}
					ui.Text(c, cr.Name).FontSize(m.FontXS).TextColor(color).SingleLine()
				})
			} else {
				// 最后一项：当前请求名称，支持点击编辑
				if !title.editing {
					nameBtn := ui.ButtonBase(c.Key("req-name-btn-" + activeID)).Height(22).
						PaddingX(6).Radius(m.RadiusSM).AlignItems(ui.Center).Gap(4)
					if nameBtn.Hovered() {
						nameBtn.Background(tk.HoverSurface)
					}
					if nameBtn.Clicked() {
						title.editing = true
						title.fresh = true
						title.draft = n.Name
					}
					nameBtn.Children(func() {
						ui.Text(c, n.Name).FontSize(m.FontSM).Bold().
							TextColor(tk.TextPrimary).SingleLine()
						if nameBtn.Hovered() {
							ui.Icon(c, icons.Get("file-pen")).FontSize(11).
								TextColor(tk.TextMuted)
						}
					})
				} else {
					in := ui.TextInputBase(c, &title.draft).Height(22).Padding(0, 6).
						Radius(m.RadiusSM).Border(m.Border, tk.Accent).
						FontSize(m.FontSM).Bold().TextColor(tk.TextPrimary)
					if title.fresh {
						title.fresh = false
						in.AutoFocus()
					}
					done := func(keep bool) {
						title.editing = false
						if !keep {
							return
						}
						next := strings.TrimSpace(title.draft)
						if next != "" && next != n.Name {
							props.actions.rename(string(n.ID), next)
						}
					}
					if in.Submitted() || in.FocusLeft() {
						done(true)
					} else if in.Shortcut(0, ui.KeyEscape) {
						done(false)
					}
				}
			}
		}

		ui.Spacer(c)
		ui.Text(c, "已保存").FontSize(10).TextColor(tk.TextMuted)
	})
}

// 2. 请求行：Method 下拉 + URL 输入框 + 发送按钮
func reqLine(c *ui.Context, props pageProps, coll *domain.Collection, n *domain.Node,
	activeID string, state *reqSessionState, tk theme.Tokens, m theme.Metric) {
	req := n.Request

	ui.Row(c).Height(36).Gap(m.Space2).AlignItems(ui.Center).Children(func() {
		// 请求类型选择器（点击弹出 Dropdown）
		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
		methodBtn := ui.ButtonBase(c.Key("req-method-btn-" + activeID)).
			Height(34).PaddingX(10).Gap(6).AlignItems(ui.Center).
			Radius(m.RadiusMD).Background(tk.BgElevated).Border(m.Border, tk.Border)
		if methodBtn.Hovered() {
			methodBtn.Background(tk.HoverSurface)
		}
		if methodBtn.Clicked() {
			state.methodOpen = !state.methodOpen
		}
		methodBtn.Children(func() {
			ui.Text(c, req.Method).FontSize(m.FontSM).Bold().
				TextColor(methodColor(tk, req.Method))
			ui.Icon(c, icons.Get("chevron-down")).FontSize(11).TextColor(tk.TextMuted)
		})

		ui.PopoverBase(c, methodBtn, &state.methodOpen, func(panel ui.Element) {
			panel.Margin(4, 0, 0, 0).Padding(4).Width(110).Gap(2)
			theme.PopoverPanel(c, panel)
			for _, mth := range methods {
				mth := mth
				row := ui.ButtonBase(c.Key("method-opt-" + mth)).FillWidth().Height(26).
					PaddingX(8).Radius(m.RadiusSM).AlignItems(ui.Center)
				if row.Hovered() {
					row.Background(tk.PopoverItemHover)
				}
				if row.Clicked() {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.Method = mth
						}
					})
					state.methodOpen = false
				}
				row.Children(func() {
					ui.Text(c, mth).FontSize(m.FontXS).Bold().
						TextColor(methodColor(tk, mth))
				})
			}
		})

		// URL 输入栏（支持双向同步与回车自动更新）
		urlBox := ui.Row(c).Grow(1).Height(34).PaddingX(10).AlignItems(ui.Center).
			Radius(m.RadiusMD).Background(tk.BgSurface).Border(m.Border, tk.Border)
		urlBox.Children(func() {
			in := ui.TextInputBase(c, &state.urlDraft).FillWidth().
				FontSize(m.FontSM).TextColor(tk.TextPrimary).
				Placeholder("输入请求 URL 地址，例如 https://api.example.com/v1/users")
			if in.Submitted() || in.FocusLeft() {
				if state.urlDraft != req.URL {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.URL = strings.TrimSpace(state.urlDraft)
						}
					})
				}
			}
		})

		// 发送按钮
		sendBtn := ui.PrimaryButton(c, "发送").Height(34).PaddingX(16).FontSize(m.FontSM)
		if sendBtn.Clicked() {
			// 保存最新状态并模拟触发请求响应
			props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
				if node.Request != nil {
					node.Request.URL = strings.TrimSpace(state.urlDraft)
				}
			})
			state.hasResponse = true
			state.respStatus = 200
			state.respDuration = "96 ms"
			state.respSize = "1.42 KB"
		}
	})
}

// 3. 请求配置 Tab 切换区
func reqConfigTabs(c *ui.Context, props pageProps, coll *domain.Collection, n *domain.Node,
	activeID string, state *reqSessionState, tk theme.Tokens, m theme.Metric) {
	req := n.Request

	tabLabels := []string{
		fmt.Sprintf("Params (%d)", len(req.Query)),
		"Authorization",
		fmt.Sprintf("Headers (%d)", len(req.Headers)),
		"Body",
		"Settings",
	}

	tabsRowWithAction(c, tabLabels, state.reqTab, func(idx int) {
		state.reqTab = idx
	}, nil)

	ui.Divider(c).BorderColor(tk.Border)

	// 根据当前选中的配置 Tab 渲染可编辑面板
	ui.Column(c).Grow(1).Padding(m.Space2, 0).Children(func() {
		switch state.reqTab {
		case 0:
			// Params 参数编辑
			renderKVEditor(c, tk, m, "Query Parameters", req.Query, func(items []domain.KV) {
				props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
					if node.Request != nil {
						node.Request.Query = items
					}
				})
			})
		case 1:
			// Authorization 认证配置
			renderAuthEditor(c, props, coll, n, state, tk, m)
		case 2:
			// Headers 请求头编辑
			renderKVEditor(c, tk, m, "Headers", req.Headers, func(items []domain.KV) {
				props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
					if node.Request != nil {
						node.Request.Headers = items
					}
				})
			})
		case 3:
			// Body 请求体配置
			renderBodyEditor(c, props, coll, n, state, tk, m)
		case 4:
			// Settings 传输设置
			renderSettingsEditor(c, props, coll, n, tk, m)
		}
	})
}

// 通用键值对表格编辑器（用于 Params 和 Headers）
func renderKVEditor(c *ui.Context, tk theme.Tokens, m theme.Metric,
	title string, items []domain.KV, onUpdate func([]domain.KV)) {
	ui.Column(c).Grow(1).Children(func() {
		// 表头
		ui.Row(c).Height(26).PaddingX(8).AlignItems(ui.Center).
			Background(tk.HoverSurface.Alpha(0.5)).Radius(m.RadiusSM).
			Children(func() {
				ui.Box(c).Width(24) // Checkbox 占位
				ui.Text(c, "KEY").FontSize(10).FontWeight(600).
					TextColor(tk.TextMuted).Width(160)
				ui.Text(c, "VALUE").FontSize(10).FontWeight(600).
					TextColor(tk.TextMuted).Grow(1)
				ui.Text(c, "DESCRIPTION").FontSize(10).FontWeight(600).
					TextColor(tk.TextMuted).Width(140)
				ui.Box(c).Width(28) // 操作占位
			})

		// 列表项
		if len(items) == 0 {
			ui.Row(c).Height(40).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(c, "暂无配置项，点击下方按钮添加").FontSize(m.FontXS).
					TextColor(tk.TextMuted)
			})
		} else {
			for i, kv := range items {
				i, kv := i, kv
				ui.Row(c).Height(32).PaddingX(8).AlignItems(ui.Center).Gap(6).
					BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border.Alpha(0.5)).
					Children(func() {
						// 复选框
						chk := ui.ButtonBase(c.Key(fmt.Sprintf("chk-%s-%d", title, i))).
							Size(14, 14).Radius(m.RadiusSM).Justify(ui.Center).AlignItems(ui.Center)
						if kv.Enabled {
							chk.Background(tk.Accent)
							chk.Children(func() {
								ui.Icon(c, icons.Get("check")).Size(10, 10).TextColor(tk.BgSurface)
							})
						} else {
							chk.Border(m.Border, tk.Border)
						}
						if chk.Clicked() {
							cp := append([]domain.KV(nil), items...)
							cp[i].Enabled = !cp[i].Enabled
							onUpdate(cp)
						}

						// Key 输入
						keyIn := ui.TextInputBase(c, &kv.Key).Width(160).FontSize(m.FontSM).
							TextColor(tk.TextPrimary).Placeholder("Key")
						if keyIn.FocusLeft() || keyIn.Submitted() {
							cp := append([]domain.KV(nil), items...)
							cp[i].Key = kv.Key
							onUpdate(cp)
						}

						// Value 输入
						valIn := ui.TextInputBase(c, &kv.Value).Grow(1).FontSize(m.FontSM).
							TextColor(tk.Info).Placeholder("Value")
						if valIn.FocusLeft() || valIn.Submitted() {
							cp := append([]domain.KV(nil), items...)
							cp[i].Value = kv.Value
							onUpdate(cp)
						}

						// Desc 输入
						descIn := ui.TextInputBase(c, &kv.Desc).Width(140).FontSize(m.FontSM).
							TextColor(tk.TextMuted).Placeholder("Description")
						if descIn.FocusLeft() || descIn.Submitted() {
							cp := append([]domain.KV(nil), items...)
							cp[i].Desc = kv.Desc
							onUpdate(cp)
						}

						// 删除按钮
						delBtn := ui.ButtonBase(c.Key(fmt.Sprintf("del-%s-%d", title, i))).
							Size(22, 22).Radius(m.RadiusSM).Justify(ui.Center).AlignItems(ui.Center)
						if delBtn.Hovered() {
							delBtn.Background(tk.HoverSurface)
						}
						if delBtn.Clicked() {
							cp := append([]domain.KV(nil), items[:i]...)
							cp = append(cp, items[i+1:]...)
							onUpdate(cp)
						}
						delBtn.Children(func() {
							ui.Icon(c, icons.Get("trash")).Size(12, 12).TextColor(tk.TextMuted)
						})
					})
			}
		}

		// 底部添加按钮
		ui.Box(c).Height(4)
		addBtn := ui.ButtonBase(c.Key("add-row-" + title)).Height(24).PaddingX(8).
			Radius(m.RadiusSM).AlignItems(ui.Center).Gap(4)
		if addBtn.Hovered() {
			addBtn.Background(tk.HoverSurface)
		}
		if addBtn.Clicked() {
			cp := append([]domain.KV(nil), items...)
			cp = append(cp, domain.KV{ID: domain.NewID(), Enabled: true})
			onUpdate(cp)
		}
		addBtn.Children(func() {
			ui.Icon(c, icons.Get("plus")).Size(12, 12).TextColor(tk.Accent)
			ui.Text(c, "添加一行").FontSize(m.FontXS).TextColor(tk.Accent)
		})
	})
}

// 认证配置面板
func renderAuthEditor(c *ui.Context, props pageProps, coll *domain.Collection, n *domain.Node,
	state *reqSessionState, tk theme.Tokens, m theme.Metric) {
	req := n.Request
	currType := string(n.Auth.Type)
	if currType == "" {
		currType = "inherit"
	}

	ui.Column(c).Grow(1).Gap(10).Children(func() {
		ui.Row(c).Height(30).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "认证类型 (Type):").FontSize(m.FontSM).TextColor(tk.TextSecondary)

			for _, t := range []struct{ key, label string }{
				{"inherit", "继承父级"},
				{"bearer", "Bearer Token"},
				{"basic", "Basic Auth"},
				{"apikey", "API Key"},
				{"none", "无认证"},
			} {
				t := t
				btn := ui.ButtonBase(c.Key("auth-type-" + t.key)).Height(24).PaddingX(8).
					Radius(m.RadiusSM).AlignItems(ui.Center)
				if currType == t.key {
					btn.Background(tk.FocusRing).TextColor(tk.BgSurface)
				} else if btn.Hovered() {
					btn.Background(tk.HoverSurface)
				}
				if btn.Clicked() {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						node.Auth.Type = domain.AuthType(t.key)
					})
				}
				btn.Children(func() {
					color := tk.TextSecondary
					if currType == t.key {
						color = tk.BgSurface
					}
					ui.Text(c, t.label).FontSize(m.FontXS).TextColor(color)
				})
			}
		})

		ui.Box(c).Height(1).Background(tk.Border.Alpha(0.6))

		// 各认证方式的表单字段
		switch currType {
		case "inherit":
			ui.Row(c).Height(40).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Icon(c, icons.Get("circle-help")).Size(14, 14).TextColor(tk.Info)
				ui.Text(c, "该请求自动继承目录或集合层级配置的认证令牌与授权头。").
					FontSize(m.FontSM).TextColor(tk.TextMuted)
			})
		case "bearer":
			tokenVal := ""
			for _, p := range n.Auth.Params {
				if p.Key == "token" {
					tokenVal = p.Value
				}
			}
			ui.Row(c).Height(32).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "Token:").FontSize(m.FontSM).TextColor(tk.TextSecondary).Width(60)
				in := ui.TextInputBase(c, &tokenVal).Grow(1).Height(28).PaddingX(8).
					Radius(m.RadiusSM).Border(m.Border, tk.Border).
					FontSize(m.FontSM).TextColor(tk.TextPrimary).Placeholder("输入 Bearer Token")
				if in.FocusLeft() || in.Submitted() {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						node.Auth.Params = []domain.KV{{Key: "token", Value: tokenVal, Enabled: true}}
					})
				}
			})
		case "basic":
			userVal, passVal := "", ""
			for _, p := range n.Auth.Params {
				if p.Key == "username" {
					userVal = p.Value
				}
				if p.Key == "password" {
					passVal = p.Value
				}
			}
			ui.Column(c).Gap(6).Children(func() {
				ui.Row(c).Height(30).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "用户名:").FontSize(m.FontSM).TextColor(tk.TextSecondary).Width(60)
					in := ui.TextInputBase(c, &userVal).Width(240).Height(28).PaddingX(8).
						Radius(m.RadiusSM).Border(m.Border, tk.Border).
						FontSize(m.FontSM).TextColor(tk.TextPrimary)
					if in.FocusLeft() || in.Submitted() {
						props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
							node.Auth.Params = []domain.KV{
								{Key: "username", Value: userVal, Enabled: true},
								{Key: "password", Value: passVal, Enabled: true},
							}
						})
					}
				})
				ui.Row(c).Height(30).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "密码:").FontSize(m.FontSM).TextColor(tk.TextSecondary).Width(60)
					in := ui.TextInputBase(c, &passVal).Width(240).Height(28).PaddingX(8).
						Radius(m.RadiusSM).Border(m.Border, tk.Border).
						FontSize(m.FontSM).TextColor(tk.TextPrimary)
					if in.FocusLeft() || in.Submitted() {
						props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
							node.Auth.Params = []domain.KV{
								{Key: "username", Value: userVal, Enabled: true},
								{Key: "password", Value: passVal, Enabled: true},
							}
						})
					}
				})
			})
		case "none":
			ui.Text(c, "当前请求不使用任何认证凭证。").FontSize(m.FontSM).
				TextColor(tk.TextMuted)
		}
		_ = req
	})
}

// 请求体配置面板
func renderBodyEditor(c *ui.Context, props pageProps, coll *domain.Collection, n *domain.Node,
	state *reqSessionState, tk theme.Tokens, m theme.Metric) {
	req := n.Request
	mode := string(req.Body.Mode)
	if mode == "" {
		mode = "none"
	}

	ui.Column(c).Grow(1).Gap(8).Children(func() {
		// 模式切换行
		ui.Row(c).Height(28).AlignItems(ui.Center).Gap(6).Children(func() {
			for _, item := range []struct{ key, label string }{
				{"none", "none"},
				{"raw", "raw (JSON)"},
				{"form-data", "form-data"},
				{"urlencoded", "x-www-form-urlencoded"},
			} {
				item := item
				btn := ui.ButtonBase(c.Key("body-mode-" + item.key)).Height(22).PaddingX(8).
					Radius(m.RadiusSM).AlignItems(ui.Center)
				if mode == item.key {
					btn.Background(tk.FocusRing).TextColor(tk.BgSurface)
				} else if btn.Hovered() {
					btn.Background(tk.HoverSurface)
				}
				if btn.Clicked() {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.Body.Mode = domain.BodyMode(item.key)
						}
					})
				}
				btn.Children(func() {
					color := tk.TextSecondary
					if mode == item.key {
						color = tk.BgSurface
					}
					ui.Text(c, item.label).FontSize(m.FontXS).TextColor(color)
				})
			}
		})

		ui.Box(c).Height(1).Background(tk.Border.Alpha(0.6))

		// 模式内容展示
		switch mode {
		case "none":
			ui.Row(c).Height(60).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(c, "该请求不携带请求体内容").FontSize(m.FontSM).
					TextColor(tk.TextMuted)
			})
		case "raw":
			in := ui.TextAreaBase(c, &state.bodyRawDraft).FillWidth().MinHeight(120).
				MaxHeight(260).Padding(8, 10).FontSize(m.FontSM).
				FixedLineHeight(20).TextColor(tk.TextPrimary)
			if in.FocusLeft() {
				if state.bodyRawDraft != req.Body.Raw {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.Body.Raw = state.bodyRawDraft
						}
					})
				}
			}
		case "form-data", "urlencoded":
			renderKVEditor(c, tk, m, "Body Fields", req.Body.Fields, func(items []domain.KV) {
				props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
					if node.Request != nil {
						node.Request.Body.Fields = items
					}
				})
			})
		}
	})
}

// 传输设置面板
func renderSettingsEditor(c *ui.Context, props pageProps, coll *domain.Collection,
	n *domain.Node, tk theme.Tokens, m theme.Metric) {
	req := n.Request

	timeoutStr := "10000"
	if req.Options.TimeoutMS > 0 {
		timeoutStr = strconv.Itoa(req.Options.TimeoutMS)
	}

	ui.Column(c).Grow(1).Gap(12).Children(func() {
		ui.Row(c).Height(30).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, "超时时间 (Timeout ms):").FontSize(m.FontSM).
				TextColor(tk.TextSecondary).Width(160)
			in := ui.TextInputBase(c, &timeoutStr).Width(120).Height(26).PaddingX(6).
				Radius(m.RadiusSM).Border(m.Border, tk.Border).
				FontSize(m.FontSM).TextColor(tk.TextPrimary)
			if in.FocusLeft() || in.Submitted() {
				if val, err := strconv.Atoi(timeoutStr); err == nil && val > 0 {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.Options.TimeoutMS = val
						}
					})
				}
			}
		})

		ui.Text(c, "更多传输策略遵循工作区默认网络配置。").FontSize(10).
			TextColor(tk.TextMuted)
	})
}

// 4. 底部响应区：支持 Body, Headers, Cookies 的 Tab 切换
func responsePane(c *ui.Context, props pageProps, n *domain.Node,
	state *reqSessionState, tk theme.Tokens, m theme.Metric) {
	ui.Column(c).Fill().Background(tk.BgSurface).Children(func() {
		// 响应头状态行
		ui.Row(c).Height(38).PaddingX(m.Space4).Gap(m.Space2).
			AlignItems(ui.Center).BorderWidth(m.Border, 0, 0, 0).
			BorderColor(tk.Border).Children(func() {
			ui.Text(c, locales.Response()).FontSize(m.FontSM).Bold().
				TextColor(tk.TextPrimary)

			if state.hasResponse {
				// 状态码 Badge
				badgeBg := tk.Success.Alpha(0.12)
				badgeColor := tk.Success
				if state.respStatus >= 400 {
					badgeBg = tk.Danger.Alpha(0.12)
					badgeColor = tk.Danger
				}
				ui.Text(c, fmt.Sprintf("%d OK", state.respStatus)).FontSize(11).Bold().
					TextColor(badgeColor).Padding(2, 6).Radius(m.RadiusSM).
					Background(badgeBg)

				ui.Text(c, fmt.Sprintf("%s · %s", state.respDuration, state.respSize)).
					FontSize(10).TextColor(tk.TextMuted)
			} else {
				ui.Text(c, "未发送请求").FontSize(10).TextColor(tk.TextMuted)
			}

			ui.Spacer(c)
			ui.Text(c, "Network Ready").FontSize(10).TextColor(tk.TextMuted)
		})

		// 响应内容 Tab 栏
		respTabs := []string{"Body", "Headers (6)", "Cookies (2)"}
		tabsRowWithAction(c, respTabs, state.respTab, func(idx int) {
			state.respTab = idx
		}, nil)

		ui.Divider(c).BorderColor(tk.Border)

		// 响应 Tab 内容
		ui.Scroll(c).Grow(1).Padding(m.Space2, m.Space4).Children(func() {
			switch state.respTab {
			case 0:
				// Response Body 展示
				renderResponseBody(c, state.respBody, tk, m)
			case 1:
				// Response Headers 展示
				headers := [][2]string{
					{"Content-Type", "application/json; charset=utf-8"},
					{"Server", "rddPostman-Core/1.0"},
					{"Date", "Mon, 16 Nov 2026 09:20:00 GMT"},
					{"Cache-Control", "no-cache, private"},
					{"X-Request-Id", "req_9f2c81b4d3"},
					{"Content-Length", strconv.Itoa(len(state.respBody))},
				}
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					for _, h := range headers {
						ui.Row(c).Height(24).Children(func() {
							ui.Text(c, h[0]).FontSize(11).Bold().TextColor(tk.TextSecondary).Width(160)
							ui.Text(c, h[1]).FontSize(11).TextColor(tk.TextPrimary).Grow(1)
						})
					}
				})
			case 2:
				// Cookies 展示
				cookies := [][4]string{
					{"session_token", "sess_910481bc92a", ".example.com", "/"},
					{"csrf_token", "csrf_58392100", ".example.com", "/"},
				}
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					ui.Row(c).Height(22).Background(tk.HoverSurface.Alpha(0.4)).Children(func() {
						ui.Text(c, "NAME").FontSize(10).Bold().TextColor(tk.TextMuted).Width(140)
						ui.Text(c, "VALUE").FontSize(10).Bold().TextColor(tk.TextMuted).Grow(1)
						ui.Text(c, "DOMAIN").FontSize(10).Bold().TextColor(tk.TextMuted).Width(120)
						ui.Text(c, "PATH").FontSize(10).Bold().TextColor(tk.TextMuted).Width(60)
					})
					for _, ck := range cookies {
						ui.Row(c).Height(24).Children(func() {
							ui.Text(c, ck[0]).FontSize(11).Bold().TextColor(tk.TextPrimary).Width(140)
							ui.Text(c, ck[1]).FontSize(11).TextColor(tk.Info).Grow(1)
							ui.Text(c, ck[2]).FontSize(11).TextColor(tk.TextMuted).Width(120)
							ui.Text(c, ck[3]).FontSize(11).TextColor(tk.TextMuted).Width(60)
						})
					}
				})
			}
		})
	})
}

// 格式化展示响应体
func renderResponseBody(c *ui.Context, body string, tk theme.Tokens, m theme.Metric) {
	var pretty strings.Builder
	var obj any
	if err := json.Unmarshal([]byte(body), &obj); err == nil {
		if formatted, err := json.MarshalIndent(obj, "", "  "); err == nil {
			pretty.Write(formatted)
		} else {
			pretty.WriteString(body)
		}
	} else {
		pretty.WriteString(body)
	}

	lines := strings.Split(pretty.String(), "\n")

	ui.Row(c).Grow(1).AlignItems(ui.Start).Clip().Radius(m.RadiusMD).
		Background(tk.BgElevated.Alpha(0.4)).Border(m.Border, tk.Border).
		Children(func() {
			// 行号列
			ui.Column(c).Width(32).Padding(m.Space2, m.Space1).Gap(1).Children(func() {
				for i := range lines {
					ui.Text(c, strconv.Itoa(i+1)).FontSize(10).
						TextColor(tk.TextMuted).TextAlign(ui.End)
				}
			})

			// 内容列
			ui.Column(c).Grow(1).Padding(m.Space2, m.Space2).Gap(1).Children(func() {
				for _, line := range lines {
					jsonLine(c, tk, line)
				}
			})
		})
}

// tabsRowWithAction 是支持点击切换的下划线 Tab 行
func tabsRowWithAction(c *ui.Context, labels []string, active int,
	onSelect func(int), trailing func()) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Row(c).Height(32).Gap(m.Space5).
		Children(func() {
			for i, label := range labels {
				i, label := i, label
				btn := ui.ButtonBase(c.Key("tab-" + label)).Height(32).
					AlignItems(ui.Center).Justify(ui.Center)
				if btn.Clicked() {
					onSelect(i)
				}
				btn.Children(func() {
					ui.Column(c).Height(32).Children(func() {
						ui.Box(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).
							Children(func() {
								txt := ui.Text(c, label).FontSize(m.FontSM)
								if i == active {
									txt.TextColor(tk.Accent).Bold()
								} else {
									txt.TextColor(tk.TextMuted)
								}
							})
						bar := ui.Box(c).Height(2)
						if i == active {
							bar.Background(tk.Accent)
						}
					})
				})
			}
			if trailing != nil {
				ui.Spacer(c)
				trailing()
			}
		})
}

// jsonLine 把一行 JSON 拆成键和值分别着色
func jsonLine(c *ui.Context, tk theme.Tokens, line string) {
	t := strings.TrimSpace(line)
	if t == "{" || t == "}" || t == "}," || strings.HasSuffix(t, "{") || strings.HasSuffix(t, "[") || t == "]" {
		ui.Text(c, line).FontSize(11).Font("monospace").
			TextColor(tk.TextMuted).SingleLine()
		return
	}
	if i := strings.Index(line, `": `); i >= 0 {
		key, val := line[:i+1]+`": `, line[i+3:]
		color := tk.Success
		if val == "{" || val == "}" || val == "}," {
			color = tk.TextMuted
		} else if val != "" && (val[0] >= '0' && val[0] <= '9' ||
			strings.HasPrefix(val, "true") || strings.HasPrefix(val, "false") || val == "null") {
			color = tk.Warning
		}
		ui.Row(c).Children(func() {
			ui.Text(c, key).FontSize(11).Font("monospace").TextColor(tk.Info)
			ui.Text(c, val).FontSize(11).Font("monospace").TextColor(color)
		})
		return
	}
	ui.Text(c, line).FontSize(11).Font("monospace").TextColor(tk.TextSecondary)
}
