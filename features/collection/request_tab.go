package collection

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"rddpostman/components/theme"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/curl"
	"rddpostman/internal/domain"
	"rddpostman/internal/transport"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// reqTitleState 维护请求名称的面包屑行内编辑态。
type reqTitleState struct {
	id      string
	editing bool
	fresh   bool
	draft   string
}

// reqSessionState 维护请求 tab 的运行时会话状态（Tab 选中、下拉展开、本地草稿与拆分高度）。
type reqSessionState struct {
	id           string
	reqTab       int     // 0: Params, 1: Authorization, 2: Headers, 3: Body, 4: Scripts, 5: Settings
	respTab      int     // 0: Body, 1: Headers, 2: Cookies, 3: Test Results
	scriptSubTab int     // 0: Pre-request Script, 1: Tests Script
	methodOpen   bool    // 请求方式下拉菜单开关
	authTypeOpen bool    // 认证方式下拉菜单开关
	urlDraft     string  // URL 本地编辑草稿
	bodyRawDraft string  // Raw 请求体本地编辑草稿
	splitHeight  float32 // 请求区与响应区的上下分栏高度

	// 可编辑表单与列表草稿（保证 TextInputBase 在重绘时不被擦除）
	pathVarsDraft         []domain.KV
	queryDraft            []domain.KV
	headersDraft          []domain.KV
	bodyFieldsDraft       []domain.KV
	authTokenDraft        string
	authUserDraft         string
	authPassDraft         string
	authKeyDraft          string
	authValDraft          string
	timeoutDraft          string
	preScriptDraft        string
	testsScriptDraft      string
	proxyDraft            string
	disableProxyDraft     bool
	disableCookieJarDraft bool

	// 发送执行态与取消控制
	sending  bool
	cancelFn func()

	// 响应区展示状态
	hasResponse     bool
	respStatus      int
	respDuration    string
	respSize        string
	respBody        string
	respHeaders     []domain.KV
	respCookies     []transport.ResponseCookie
	respTestResults []domain.TestResult
	respError       string
	respEditor      *editor.State
}

// extractPathVariables 从 URL 中提取路径变量（:id 或 {id}），保留已有值
func extractPathVariables(rawURL string, existing []domain.KV) []domain.KV {
	varMap := make(map[string]domain.KV)
	for _, kv := range existing {
		varMap[kv.Key] = kv
	}

	reColon := regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*)`)
	reBrace := regexp.MustCompile(`\{([a-zA-Z0-9_-]+)\}`)

	var out []domain.KV
	seen := make(map[string]bool)

	for _, m := range reColon.FindAllStringSubmatch(rawURL, -1) {
		key := m[1]
		if !seen[key] {
			seen[key] = true
			if ex, ok := varMap[key]; ok {
				out = append(out, ex)
			} else {
				out = append(out, domain.KV{ID: domain.NewID(), Key: key, Enabled: true})
			}
		}
	}
	for _, m := range reBrace.FindAllStringSubmatch(rawURL, -1) {
		key := m[1]
		if !seen[key] {
			seen[key] = true
			if ex, ok := varMap[key]; ok {
				out = append(out, ex)
			} else {
				out = append(out, domain.KV{ID: domain.NewID(), Key: key, Enabled: true})
			}
		}
	}
	return out
}

func defaultSessionState(id string, req *domain.Request, auth *domain.Auth) reqSessionState {
	url := ""
	rawBody := ""
	var pathVars []domain.KV
	var query []domain.KV
	var headers []domain.KV
	var bodyFields []domain.KV
	timeoutStr := "10000"

	preScript := ""
	testsScript := ""
	proxyURL := ""
	disableProxy := false
	disableCookieJar := false

	if req != nil {
		url = req.URL
		rawBody = req.Body.Raw
		preScript = req.PreRequestScript
		testsScript = req.TestsScript
		proxyURL = req.Options.ProxyURL
		disableProxy = req.Options.DisableProxy
		disableCookieJar = req.Options.DisableCookieJar
		if len(req.PathVariables) > 0 {
			pathVars = make([]domain.KV, len(req.PathVariables))
			copy(pathVars, req.PathVariables)
		}
		if len(req.Query) > 0 {
			query = make([]domain.KV, len(req.Query))
			copy(query, req.Query)
		}
		if len(req.Headers) > 0 {
			headers = make([]domain.KV, len(req.Headers))
			copy(headers, req.Headers)
		}
		if len(req.Body.Fields) > 0 {
			bodyFields = make([]domain.KV, len(req.Body.Fields))
			copy(bodyFields, req.Body.Fields)
		}
		if req.Options.TimeoutMS > 0 {
			timeoutStr = strconv.Itoa(req.Options.TimeoutMS)
		}
	}

	// 保证 Params、Headers、Body 默认至少有一行空白行以便直接输入
	pathVars = extractPathVariables(url, pathVars)
	if len(query) == 0 {
		query = parseQueryFromURL(url)
	}
	if len(headers) == 0 {
		headers = []domain.KV{{ID: domain.NewID(), Enabled: true}}
	}
	if len(bodyFields) == 0 {
		bodyFields = []domain.KV{{ID: domain.NewID(), Enabled: true}}
	}

	authToken := ""
	authUser := ""
	authPass := ""
	authKey := ""
	authVal := ""
	if auth != nil {
		for _, p := range auth.Params {
			switch p.Key {
			case "token":
				authToken = p.Value
			case "username":
				authUser = p.Value
			case "password":
				authPass = p.Value
			case "key":
				authKey = p.Value
			case "value":
				authVal = p.Value
			}
		}
	}

	return reqSessionState{
		id:                    id,
		reqTab:                0,
		respTab:               0,
		urlDraft:              url,
		bodyRawDraft:          rawBody,
		splitHeight:           340,
		pathVarsDraft:         pathVars,
		queryDraft:            query,
		headersDraft:          headers,
		bodyFieldsDraft:       bodyFields,
		authTokenDraft:        authToken,
		authUserDraft:         authUser,
		authPassDraft:         authPass,
		authKeyDraft:          authKey,
		authValDraft:          authVal,
		timeoutDraft:          timeoutStr,
		preScriptDraft:        preScript,
		testsScriptDraft:      testsScript,
		proxyDraft:            proxyURL,
		disableProxyDraft:     disableProxy,
		disableCookieJarDraft: disableCookieJar,
		hasResponse:           false,
		respStatus:            0,
		respDuration:          "-",
		respSize:              "-",
		respBody:              "",
		respEditor: func() *editor.State {
			buf := editor.NewBuffer("")
			buf.SetLanguage(editor.LanguageNamed("JSON"))
			return &editor.State{
				Buffer:      buf,
				ReadOnly:    true,
				SoftWrap:    true,
				LineNumbers: editor.LineNumbersOn,
			}
		}(),
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
		return defaultSessionState(activeID, req, &n.Auth)
	})
	if state.id != activeID {
		*state = defaultSessionState(activeID, req, &n.Auth)
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
	title *reqTitleState, tk theme.Tokens, m theme.Metric,
) {
	crumbs := nodeTrail(props.collections, activeID)

	ui.Row(c).Height(26).Gap(m.Space1).AlignItems(ui.Center).Children(func() {
		for i, cr := range crumbs {
			i, cr := i, cr
			isLast := i == len(crumbs)-1

			if i > 0 {
				ui.Text(c, "/").FontSize(m.FontSM).
					TextColor(tk.TextMuted).Margin(0, 2)
			}

			if isLast {
				// 最后一项：进入行内编辑或展示可点击态
				if title.editing {
					in := ui.TextInputBase(c.Key("req-title-input"), &title.draft).
						Height(22).Padding(0, 6).
						Radius(m.RadiusSM).Border(m.Border, tk.FocusRing).
						Background(tk.BgElevated).
						FontSize(m.FontSM).TextColor(tk.TextPrimary)
					if title.fresh {
						in.Focus()
						title.fresh = false
					}
					if in.Submitted() || in.FocusLeft() {
						title.editing = false
						trimmed := strings.TrimSpace(title.draft)
						if trimmed != "" && trimmed != n.Name {
							n.Name = trimmed
							props.actions.rename(activeID, trimmed)
						} else {
							title.draft = n.Name
						}
					}
					if in.Shortcut(0, ui.KeyEscape) {
						title.editing = false
						title.draft = n.Name
					}
				} else {
					btn := ui.ButtonBase(c.Key("req-title-btn-" + activeID)).
						Height(22).PaddingX(6).Radius(m.RadiusSM).
						AlignItems(ui.Center).Gap(4)
					if btn.Hovered() {
						btn.Background(tk.HoverSurface)
					}
					if btn.Clicked() {
						title.editing = true
						title.fresh = true
						title.draft = n.Name
					}
					btn.Children(func() {
						ui.Text(c, n.Name).FontSize(m.FontSM).FontWeight(600).
							TextColor(tk.TextPrimary).SingleLine()
						ui.Icon(c, icons.Get("file-pen")).Size(11, 11).TextColor(tk.TextMuted)
					})
				}
			} else {
				btn := ui.ButtonBase(c.Key("req-crumb-" + cr.ID)).
					Height(22).PaddingX(4).Radius(m.RadiusSM).AlignItems(ui.Center)
				if btn.Hovered() {
					btn.Background(tk.HoverSurface)
				}
				if btn.Clicked() {
					props.actions.openTab(cr.ID)
				}
				btn.Children(func() {
					textColor := tk.TextMuted
					if btn.Hovered() {
						textColor = tk.TextPrimary
					}
					ui.Text(c, cr.Name).FontSize(m.FontSM).
						TextColor(textColor).SingleLine()
				})
			}
		}

		ui.Spacer(c)
		ui.Text(c, "已保存").FontSize(m.FontSM).TextColor(tk.TextMuted)
	})
}

// 2. 请求行：Method 下拉 + URL 输入框 + 发送/取消按钮
func reqLine(c *ui.Context, props pageProps, coll *domain.Collection, n *domain.Node,
	activeID string, state *reqSessionState, tk theme.Tokens, m theme.Metric,
) {
	req := n.Request

	ui.Row(c).Height(36).Gap(m.Space2).AlignItems(ui.Center).Children(func() {
		// 请求类型选择器（点击弹出 Dropdown）
		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
		methodBtn := ui.ButtonBase(c.Key("req-method-btn-"+activeID)).
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
			in := ui.TextInputBase(c.Key("req-url-input"), &state.urlDraft).FillWidth().
				FontSize(m.FontSM).TextColor(tk.TextPrimary).
				Placeholder("输入请求 URL 地址，例如 https://api.example.com/v1/users 或 {{baseUrl}}/users")
			if in.Submitted() || in.FocusLeft() {
				// 同步 URL 到 Query Params 与 Path Variables
				state.queryDraft = parseQueryFromURL(state.urlDraft)
				state.pathVarsDraft = extractPathVariables(state.urlDraft, state.pathVarsDraft)
				if state.urlDraft != req.URL {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.URL = strings.TrimSpace(state.urlDraft)
							node.Request.Query = cleanKVList(state.queryDraft)
							node.Request.PathVariables = cleanKVList(state.pathVarsDraft)
						}
					})
				}
			}
		})

		// 发送 / 取消 按钮
		if state.sending {
			cancelBtn := ui.Button(c, "取消").Height(34).PaddingX(16).FontSize(m.FontSM).
				Background(tk.Danger.Alpha(0.15)).TextColor(tk.Danger).Border(m.Border, tk.Danger)
			if cancelBtn.Clicked() {
				if state.cancelFn != nil {
					state.cancelFn()
				}
				state.sending = false
			}
		} else {
			sendBtn := ui.PrimaryButton(c, "发送").Height(34).PaddingX(16).FontSize(m.FontSM)
			if sendBtn.Clicked() {
				// 1. 同步保存当前 UI 草稿到 node.Request
				state.queryDraft = parseQueryFromURL(state.urlDraft)
				state.pathVarsDraft = extractPathVariables(state.urlDraft, state.pathVarsDraft)
				currReq := *req.Clone()
				currReq.URL = strings.TrimSpace(state.urlDraft)
				currReq.PathVariables = cleanKVList(state.pathVarsDraft)
				currReq.Query = cleanKVList(state.queryDraft)
				currReq.Headers = cleanKVList(state.headersDraft)
				currReq.Body.Raw = state.bodyRawDraft
				currReq.Body.Fields = cleanKVList(state.bodyFieldsDraft)
				currReq.PreRequestScript = state.preScriptDraft
				currReq.TestsScript = state.testsScriptDraft
				currReq.Options.ProxyURL = state.proxyDraft
				currReq.Options.DisableProxy = state.disableProxyDraft
				currReq.Options.DisableCookieJar = state.disableCookieJarDraft

				props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
					if node.Request != nil {
						*node.Request = *currReq.Clone()
					}
				})

				// 2. 标记发送中
				state.sending = true

				// 3. 发送请求（包含环境变量插值、认证继承与异步网络请求）
				if props.actions.sendRequest != nil {
					cancel := props.actions.sendRequest(coll.ID, n.ID, currReq, n.Auth, func(resp *transport.Response) {
						state.sending = false
						if resp == nil {
							return
						}
						state.hasResponse = true
						state.respStatus = resp.StatusCode
						state.respDuration = transport.FormatDuration(resp.Duration)
						state.respSize = transport.FormatSize(resp.Size)
						state.respBody = resp.FormattedBody
						state.respHeaders = resp.Headers
						state.respCookies = resp.Cookies
						state.respTestResults = resp.TestResults
						state.respError = resp.Error
					})
					state.cancelFn = cancel
				} else {
					// 无网络回调时的兜底本地模拟
					state.sending = false
					state.hasResponse = true
					state.respStatus = 200
					state.respDuration = "89 ms"
					state.respSize = "1.42 KB"
					state.respBody = "{\n  \"message\": \"Simulated local response\"\n}"
				}
			}

			curlBtn := ui.Button(c, "cURL").Height(34).PaddingX(12).FontSize(m.FontSM)
			if curlBtn.Clicked() {
				state.queryDraft = parseQueryFromURL(state.urlDraft)
				state.pathVarsDraft = extractPathVariables(state.urlDraft, state.pathVarsDraft)
				currReq := *req.Clone()
				currReq.URL = strings.TrimSpace(state.urlDraft)
				currReq.PathVariables = cleanKVList(state.pathVarsDraft)
				currReq.Query = cleanKVList(state.queryDraft)
				currReq.Headers = cleanKVList(state.headersDraft)
				currReq.Body.Raw = state.bodyRawDraft
				currReq.Body.Fields = cleanKVList(state.bodyFieldsDraft)
				cmd := curl.ToCurl(currReq, n.Auth)
				c.WriteClipboard(cmd)
			}
		}
	})
}

// 3. 请求配置 Tab 切换区
func reqConfigTabs(c *ui.Context, props pageProps, coll *domain.Collection, n *domain.Node,
	activeID string, state *reqSessionState, tk theme.Tokens, m theme.Metric,
) {
	paramCount := countActiveKV(state.queryDraft) + countActiveKV(state.pathVarsDraft)
	headerCount := countActiveKV(state.headersDraft)

	tabLabels := []string{
		fmt.Sprintf("Params (%d)", paramCount),
		"Authorization",
		fmt.Sprintf("Headers (%d)", headerCount),
		"Body",
		"Scripts",
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
			// Params 参数编辑（包含 Query 与 Path 变量，与 URL 保持双向联动）
			ui.Column(c).Grow(1).Gap(10).Children(func() {
				ui.Row(c).Height(20).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Query Parameters").FontSize(11).Bold().TextColor(tk.TextSecondary)
				})
				renderKVEditor(c, tk, m, "query", &state.queryDraft, func(items []domain.KV) {
					state.urlDraft = syncQueryToURL(state.urlDraft, items)
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.Query = cleanKVList(items)
							node.Request.URL = state.urlDraft
						}
					})
				})

				if len(state.pathVarsDraft) > 0 {
					ui.Box(c).Height(8)
					ui.Row(c).Height(20).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "Path Variables").FontSize(11).Bold().TextColor(tk.TextSecondary)
					})
					renderKVEditor(c, tk, m, "pathvars", &state.pathVarsDraft, func(items []domain.KV) {
						props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
							if node.Request != nil {
								node.Request.PathVariables = cleanKVList(items)
							}
						})
					})
				}
			})
		case 1:
			// Authorization 认证配置（带继承链生效计算展示）
			renderAuthEditor(c, props, coll, n, state, tk, m)
		case 2:
			// Headers 请求头编辑
			renderKVEditor(c, tk, m, "headers", &state.headersDraft, func(items []domain.KV) {
				props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
					if node.Request != nil {
						node.Request.Headers = cleanKVList(items)
					}
				})
			})
		case 3:
			// Body 请求体配置
			renderBodyEditor(c, props, coll, n, state, tk, m)
		case 4:
			// Scripts 脚本与测试断言
			renderScriptsEditor(c, props, coll, n, state, tk, m)
		case 5:
			// Settings 传输设置
			renderSettingsEditor(c, props, coll, n, state, tk, m)
		}
	})
}

// 通用键值对表格编辑器（用于 Params、Headers、Body Fields）
func renderKVEditor(c *ui.Context, tk theme.Tokens, m theme.Metric,
	keyPrefix string, items *[]domain.KV, onPersist func([]domain.KV),
) {
	if items == nil {
		return
	}
	if len(*items) == 0 {
		*items = []domain.KV{{ID: domain.NewID(), Enabled: true}}
	}

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

		// 列表行
		list := *items
		for i := 0; i < len(list); i++ {
			i := i
			kv := &(*items)[i]

			ui.Row(c).Height(34).PaddingX(8).AlignItems(ui.Center).Gap(6).
				BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border.Alpha(0.4)).
				Children(func() {
					// 1. 复选框（启用/停用）
					chk := ui.ButtonBase(c.Key(fmt.Sprintf("%s-chk-%d", keyPrefix, i))).
						Size(16, 16).Radius(m.RadiusSM).Justify(ui.Center).AlignItems(ui.Center)
					if kv.Enabled {
						chk.Background(tk.Accent)
						chk.Children(func() {
							ui.Icon(c, icons.Get("check")).Size(11, 11).TextColor(tk.BgSurface)
						})
					} else {
						chk.Border(m.Border, tk.Border)
					}
					if chk.Clicked() {
						kv.Enabled = !kv.Enabled
						onPersist(*items)
					}

					// 2. Key 输入框
					keyIn := ui.TextInputBase(c.Key(fmt.Sprintf("%s-key-%d", keyPrefix, i)), &kv.Key).
						Width(160).Height(26).Padding(2, 6).Radius(m.RadiusSM).
						Border(m.Border, tk.Border.Alpha(0.6)).Background(tk.BgElevated.Alpha(0.3)).
						FontSize(m.FontSM).TextColor(tk.TextPrimary).Placeholder("Key")
					if keyIn.Focused() {
						keyIn.Border(m.Border, tk.FocusRing).Background(tk.BgSurface)
					} else if keyIn.Hovered() {
						keyIn.Border(m.Border, tk.Border)
					}
					if keyIn.Changed() {
						// 若在最后一行输入，自动追加新的一行空白项方便连续输入
						if i == len(*items)-1 && (kv.Key != "" || kv.Value != "") {
							*items = append(*items, domain.KV{ID: domain.NewID(), Enabled: true})
						}
					}
					if keyIn.FocusLeft() || keyIn.Submitted() {
						onPersist(*items)
					}

					// 3. Value 输入框
					valIn := ui.TextInputBase(c.Key(fmt.Sprintf("%s-val-%d", keyPrefix, i)), &kv.Value).
						Grow(1).Height(26).Padding(2, 6).Radius(m.RadiusSM).
						Border(m.Border, tk.Border.Alpha(0.6)).Background(tk.BgElevated.Alpha(0.3)).
						FontSize(m.FontSM).TextColor(tk.Info).Placeholder("Value")
					if valIn.Focused() {
						valIn.Border(m.Border, tk.FocusRing).Background(tk.BgSurface)
					} else if valIn.Hovered() {
						valIn.Border(m.Border, tk.Border)
					}
					if valIn.Changed() {
						if i == len(*items)-1 && (kv.Key != "" || kv.Value != "") {
							*items = append(*items, domain.KV{ID: domain.NewID(), Enabled: true})
						}
					}
					if valIn.FocusLeft() || valIn.Submitted() {
						onPersist(*items)
					}

					// 4. Description 输入框
					descIn := ui.TextInputBase(c.Key(fmt.Sprintf("%s-desc-%d", keyPrefix, i)), &kv.Desc).
						Width(140).Height(26).Padding(2, 6).Radius(m.RadiusSM).
						Border(m.Border, tk.Border.Alpha(0.6)).Background(tk.BgElevated.Alpha(0.3)).
						FontSize(m.FontSM).TextColor(tk.TextMuted).Placeholder("Description")
					if descIn.Focused() {
						descIn.Border(m.Border, tk.FocusRing).Background(tk.BgSurface)
					} else if descIn.Hovered() {
						descIn.Border(m.Border, tk.Border)
					}
					if descIn.FocusLeft() || descIn.Submitted() {
						onPersist(*items)
					}

					// 5. 删除行按钮
					delBtn := ui.ButtonBase(c.Key(fmt.Sprintf("%s-del-%d", keyPrefix, i))).
						Size(22, 22).Radius(m.RadiusSM).Justify(ui.Center).AlignItems(ui.Center)
					if delBtn.Hovered() {
						delBtn.Background(tk.HoverSurface)
					}
					if delBtn.Clicked() {
						cp := append([]domain.KV(nil), (*items)[:i]...)
						cp = append(cp, (*items)[i+1:]...)
						if len(cp) == 0 {
							cp = append(cp, domain.KV{ID: domain.NewID(), Enabled: true})
						}
						*items = cp
						onPersist(*items)
					}
					delBtn.Children(func() {
						ui.Icon(c, icons.Get("trash")).Size(12, 12).TextColor(tk.TextMuted)
					})
				})
		}

		// 底部添加按钮
		ui.Box(c).Height(6)
		addBtn := ui.ButtonBase(c.Key("add-row-" + keyPrefix)).Height(24).PaddingX(8).
			Radius(m.RadiusSM).AlignItems(ui.Center).Gap(4)
		if addBtn.Hovered() {
			addBtn.Background(tk.HoverSurface)
		}
		if addBtn.Clicked() {
			*items = append(*items, domain.KV{ID: domain.NewID(), Enabled: true})
			onPersist(*items)
		}
		addBtn.Children(func() {
			ui.Icon(c, icons.Get("plus")).Size(12, 12).TextColor(tk.Accent)
			ui.Text(c, "添加一行").FontSize(m.FontXS).TextColor(tk.Accent)
		})
	})
}

// 认证配置面板
func renderAuthEditor(c *ui.Context, props pageProps, coll *domain.Collection, n *domain.Node,
	state *reqSessionState, tk theme.Tokens, m theme.Metric,
) {
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

		// 各认证方式的表单字段与继承链计算
		switch currType {
		case "inherit":
			effectiveAuth := domain.Auth{Type: domain.AuthNone}
			if props.actions.resolveAuth != nil {
				effectiveAuth = props.actions.resolveAuth(coll.ID, n.ID)
			}
			authLabel := "无认证"
			switch effectiveAuth.Type {
			case domain.AuthBearer:
				authLabel = "Bearer Token"
			case domain.AuthBasic:
				authLabel = "Basic Auth"
			case domain.AuthAPIKey:
				authLabel = "API Key"
			}

			ui.Column(c).Gap(6).Children(func() {
				ui.Row(c).Height(32).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Icon(c, icons.Get("circle-help")).Size(14, 14).TextColor(tk.Info)
					ui.Text(c, "该请求将自动继承上级目录或集合配置的认证令牌与授权头。").
						FontSize(m.FontSM).TextColor(tk.TextMuted)
				})
				ui.Row(c).Height(28).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Text(c, "当前沿继承链生效的认证方式:").FontSize(m.FontSM).TextColor(tk.TextSecondary)
					ui.Box(c).Padding(2, 8).Radius(m.RadiusSM).Background(tk.HoverSurface).Children(func() {
						ui.Text(c, authLabel).FontSize(m.FontXS).FontWeight(600).TextColor(tk.FocusRing)
					})
				})
			})
		case "bearer":
			ui.Row(c).Height(32).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "Token:").FontSize(m.FontSM).TextColor(tk.TextSecondary).Width(60)
				in := ui.TextInputBase(c.Key("auth-bearer-token"), &state.authTokenDraft).
					Grow(1).Height(28).PaddingX(8).
					Radius(m.RadiusSM).Border(m.Border, tk.Border).
					FontSize(m.FontSM).TextColor(tk.TextPrimary).Placeholder("输入 Bearer Token 或 {{token}}")
				if in.Focused() {
					in.Border(m.Border, tk.FocusRing)
				}
				if in.FocusLeft() || in.Submitted() {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						node.Auth.Params = []domain.KV{{Key: "token", Value: state.authTokenDraft, Enabled: true}}
					})
				}
			})
		case "basic":
			ui.Column(c).Gap(6).Children(func() {
				ui.Row(c).Height(30).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "用户名:").FontSize(m.FontSM).TextColor(tk.TextSecondary).Width(60)
					in := ui.TextInputBase(c.Key("auth-basic-user"), &state.authUserDraft).
						Width(240).Height(28).PaddingX(8).
						Radius(m.RadiusSM).Border(m.Border, tk.Border).
						FontSize(m.FontSM).TextColor(tk.TextPrimary)
					if in.Focused() {
						in.Border(m.Border, tk.FocusRing)
					}
					if in.FocusLeft() || in.Submitted() {
						props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
							node.Auth.Params = []domain.KV{
								{Key: "username", Value: state.authUserDraft, Enabled: true},
								{Key: "password", Value: state.authPassDraft, Enabled: true},
							}
						})
					}
				})
				ui.Row(c).Height(30).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "密码:").FontSize(m.FontSM).TextColor(tk.TextSecondary).Width(60)
					in := ui.TextInputBase(c.Key("auth-basic-pass"), &state.authPassDraft).
						Width(240).Height(28).PaddingX(8).
						Radius(m.RadiusSM).Border(m.Border, tk.Border).
						FontSize(m.FontSM).TextColor(tk.TextPrimary)
					if in.Focused() {
						in.Border(m.Border, tk.FocusRing)
					}
					if in.FocusLeft() || in.Submitted() {
						props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
							node.Auth.Params = []domain.KV{
								{Key: "username", Value: state.authUserDraft, Enabled: true},
								{Key: "password", Value: state.authPassDraft, Enabled: true},
							}
						})
					}
				})
			})
		case "apikey":
			ui.Row(c).Height(30).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "Key:").FontSize(m.FontSM).TextColor(tk.TextSecondary).Width(50)
				keyIn := ui.TextInputBase(c.Key("auth-apikey-key"), &state.authKeyDraft).
					Width(180).Height(28).PaddingX(8).
					Radius(m.RadiusSM).Border(m.Border, tk.Border).
					FontSize(m.FontSM).TextColor(tk.TextPrimary).Placeholder("Header/Query 名")
				if keyIn.Focused() {
					keyIn.Border(m.Border, tk.FocusRing)
				}
				if keyIn.FocusLeft() || keyIn.Submitted() {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						node.Auth.Params = []domain.KV{
							{Key: "key", Value: state.authKeyDraft, Enabled: true},
							{Key: "value", Value: state.authValDraft, Enabled: true},
						}
					})
				}

				ui.Text(c, "Value:").FontSize(m.FontSM).TextColor(tk.TextSecondary).Width(50)
				valIn := ui.TextInputBase(c.Key("auth-apikey-val"), &state.authValDraft).
					Width(220).Height(28).PaddingX(8).
					Radius(m.RadiusSM).Border(m.Border, tk.Border).
					FontSize(m.FontSM).TextColor(tk.TextPrimary).Placeholder("Key 值")
				if valIn.Focused() {
					valIn.Border(m.Border, tk.FocusRing)
				}
				if valIn.FocusLeft() || valIn.Submitted() {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						node.Auth.Params = []domain.KV{
							{Key: "key", Value: state.authKeyDraft, Enabled: true},
							{Key: "value", Value: state.authValDraft, Enabled: true},
						}
					})
				}
			})
		case "none":
			ui.Text(c, "当前请求不使用任何认证凭证。").FontSize(m.FontSM).
				TextColor(tk.TextMuted)
		}
	})
}

// 请求体配置面板
func renderBodyEditor(c *ui.Context, props pageProps, coll *domain.Collection, n *domain.Node,
	state *reqSessionState, tk theme.Tokens, m theme.Metric,
) {
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
			renderKVEditor(c, tk, m, "body-fields", &state.bodyFieldsDraft, func(items []domain.KV) {
				props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
					if node.Request != nil {
						node.Request.Body.Fields = cleanKVList(items)
					}
				})
			})
		}
	})
}

// 脚本与测试断言面板
func renderScriptsEditor(c *ui.Context, props pageProps, coll *domain.Collection,
	n *domain.Node, state *reqSessionState, tk theme.Tokens, m theme.Metric,
) {
	subTabs := []string{"前置脚本 (Pre-request Script)", "测试断言 (Tests Script)"}
	tabsRowWithAction(c, subTabs, state.scriptSubTab, func(idx int) {
		state.scriptSubTab = idx
	}, nil)

	ui.Divider(c).BorderColor(tk.Border)

	ui.Column(c).Grow(1).Padding(m.Space2, 0).Gap(8).Children(func() {
		if state.scriptSubTab == 0 {
			// Pre-request Script
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "快速插入代码片段:").FontSize(m.FontXS).TextColor(tk.TextMuted)
				if ui.Button(c, "设置变量").Height(22).FontSize(m.FontXS).Clicked() {
					if state.preScriptDraft != "" && !strings.HasSuffix(state.preScriptDraft, "\n") {
						state.preScriptDraft += "\n"
					}
					state.preScriptDraft += "pm.variables.set(\"my_var\", \"value\");\n"
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.PreRequestScript = state.preScriptDraft
						}
					})
				}
				if ui.Button(c, "设置环境变量").Height(22).FontSize(m.FontXS).Clicked() {
					if state.preScriptDraft != "" && !strings.HasSuffix(state.preScriptDraft, "\n") {
						state.preScriptDraft += "\n"
					}
					state.preScriptDraft += "pm.environment.set(\"token\", \"secret\");\n"
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.PreRequestScript = state.preScriptDraft
						}
					})
				}
			})

			ui.TextArea(c, &state.preScriptDraft).Fill().MinHeight(180).
				Padding(m.Space2, m.Space2).Radius(m.RadiusSM).
				FontSize(m.FontSM).FixedLineHeight(20).
				Placeholder("// 在发送请求前执行的 JavaScript 脚本\npm.variables.set(\"timestamp\", \"1609459200\");\npm.environment.set(\"auth_token\", \"secret\");").
				OnChange(func() {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.PreRequestScript = state.preScriptDraft
						}
					})
				})
		} else {
			// Tests Script
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "快速插入测试断言:").FontSize(m.FontXS).TextColor(tk.TextMuted)
				if ui.Button(c, "状态码 200").Height(22).FontSize(m.FontXS).Clicked() {
					if state.testsScriptDraft != "" && !strings.HasSuffix(state.testsScriptDraft, "\n") {
						state.testsScriptDraft += "\n"
					}
					state.testsScriptDraft += "pm.test(\"Status code is 200\", function () {\n    pm.response.to.have.status(200);\n});\n"
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.TestsScript = state.testsScriptDraft
						}
					})
				}
				if ui.Button(c, "耗时 < 500ms").Height(22).FontSize(m.FontXS).Clicked() {
					if state.testsScriptDraft != "" && !strings.HasSuffix(state.testsScriptDraft, "\n") {
						state.testsScriptDraft += "\n"
					}
					state.testsScriptDraft += "pm.test(\"Response time is below 500ms\", function () {\n    pm.response.to.have.responseTime.below(500);\n});\n"
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.TestsScript = state.testsScriptDraft
						}
					})
				}
				if ui.Button(c, "JSON 字段校验").Height(22).FontSize(m.FontXS).Clicked() {
					if state.testsScriptDraft != "" && !strings.HasSuffix(state.testsScriptDraft, "\n") {
						state.testsScriptDraft += "\n"
					}
					state.testsScriptDraft += "pm.test(\"Check JSON field\", function () {\n    var jsonData = pm.response.json();\n    pm.expect(jsonData.code).to.eq(0);\n});\n"
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.TestsScript = state.testsScriptDraft
						}
					})
				}
			})

			ui.TextArea(c, &state.testsScriptDraft).Fill().MinHeight(180).
				Padding(m.Space2, m.Space2).Radius(m.RadiusSM).
				FontSize(m.FontSM).FixedLineHeight(20).
				Placeholder("// 请求收到响应后执行的测试断言脚本\npm.test(\"Status code is 200\", function () {\n    pm.response.to.have.status(200);\n});\npm.test(\"Check code is 0\", function () {\n    pm.expect(pm.response.json().code).to.eql(0);\n});").
				OnChange(func() {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.TestsScript = state.testsScriptDraft
						}
					})
				})
		}
	})
}

// 传输设置面板
func renderSettingsEditor(c *ui.Context, props pageProps, coll *domain.Collection,
	n *domain.Node, state *reqSessionState, tk theme.Tokens, m theme.Metric,
) {
	req := n.Request

	followRedirects := true
	if req.Options.FollowRedirects != nil {
		followRedirects = *req.Options.FollowRedirects
	}
	verifySSL := true
	if req.Options.VerifyTLS != nil {
		verifySSL = *req.Options.VerifyTLS
	}

	ui.Column(c).Grow(1).Gap(12).Padding(4, 0).Children(func() {
		ui.Row(c).Height(32).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "请求超时时间 (ms):").FontSize(m.FontSM).
				TextColor(tk.TextSecondary).Width(160)
			in := ui.TextInputBase(c.Key("settings-timeout-input"), &state.timeoutDraft).
				Width(120).Height(26).PaddingX(6).
				Radius(m.RadiusSM).Border(m.Border, tk.Border).
				FontSize(m.FontSM).TextColor(tk.TextPrimary)
			if in.Focused() {
				in.Border(m.Border, tk.FocusRing)
			}
			if in.FocusLeft() || in.Submitted() {
				timeout, _ := strconv.Atoi(state.timeoutDraft)
				if timeout <= 0 {
					timeout = 10000
					state.timeoutDraft = "10000"
				}
				props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
					if node.Request != nil {
						node.Request.Options.TimeoutMS = timeout
					}
				})
			}
		})

		ui.Row(c).Height(28).AlignItems(ui.Center).Gap(8).Children(func() {
			chk := ui.ButtonBase(c.Key("settings-follow-redirects")).
				Size(16, 16).Radius(m.RadiusSM).Justify(ui.Center).AlignItems(ui.Center)
			if followRedirects {
				chk.Background(tk.Accent)
				chk.Children(func() {
					ui.Icon(c, icons.Get("check")).Size(11, 11).TextColor(tk.BgSurface)
				})
			} else {
				chk.Border(m.Border, tk.Border)
			}
			if chk.Clicked() {
				val := !followRedirects
				props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
					if node.Request != nil {
						node.Request.Options.FollowRedirects = &val
					}
				})
			}
			ui.Text(c, "自动跟随 HTTP 重定向 (Follow Redirects)").FontSize(m.FontSM).
				TextColor(tk.TextSecondary)
		})

		ui.Row(c).Height(28).AlignItems(ui.Center).Gap(8).Children(func() {
			chk := ui.ButtonBase(c.Key("settings-verify-ssl")).
				Size(16, 16).Radius(m.RadiusSM).Justify(ui.Center).AlignItems(ui.Center)
			if verifySSL {
				chk.Background(tk.Accent)
				chk.Children(func() {
					ui.Icon(c, icons.Get("check")).Size(11, 11).TextColor(tk.BgSurface)
				})
			} else {
				chk.Border(m.Border, tk.Border)
			}
			if chk.Clicked() {
				val := !verifySSL
				props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
					if node.Request != nil {
						node.Request.Options.VerifyTLS = &val
					}
				})
			}
			ui.Text(c, "开启 SSL/TLS 证书有效性校验 (Verify SSL)").FontSize(m.FontSM).
				TextColor(tk.TextSecondary)
		})

		useCookieJar := !state.disableCookieJarDraft
		ui.Row(c).Height(28).AlignItems(ui.Center).Gap(8).Children(func() {
			chk := ui.ButtonBase(c.Key("settings-cookie-jar")).
				Size(16, 16).Radius(m.RadiusSM).Justify(ui.Center).AlignItems(ui.Center)
			if useCookieJar {
				chk.Background(tk.Accent)
				chk.Children(func() {
					ui.Icon(c, icons.Get("check")).Size(11, 11).TextColor(tk.BgSurface)
				})
			} else {
				chk.Border(m.Border, tk.Border)
			}
			if chk.Clicked() {
				state.disableCookieJarDraft = !state.disableCookieJarDraft
				val := state.disableCookieJarDraft
				props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
					if node.Request != nil {
						node.Request.Options.DisableCookieJar = val
					}
				})
			}
			ui.Text(c, "启用会话 Cookie 管理 (CookieJar 自动持久化并在后续请求中携带)").FontSize(m.FontSM).
				TextColor(tk.TextSecondary)
		})

		ui.Row(c).Height(32).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "自定义代理 (Proxy URL):").FontSize(m.FontSM).
				TextColor(tk.TextSecondary).Width(160)
			in := ui.TextInputBase(c.Key("settings-proxy-input"), &state.proxyDraft).
				Fill().Height(26).PaddingX(6).
				Radius(m.RadiusSM).Border(m.Border, tk.Border).
				Placeholder("例如 http://127.0.0.1:7890 或 socks5://127.0.0.1:1080").
				FontSize(m.FontSM).TextColor(tk.TextPrimary)
			if in.Focused() {
				in.Border(m.Border, tk.FocusRing)
			}
			if in.FocusLeft() || in.Submitted() {
				val := strings.TrimSpace(state.proxyDraft)
				props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
					if node.Request != nil {
						node.Request.Options.ProxyURL = val
					}
				})
			}
		})

		ui.Row(c).Height(28).AlignItems(ui.Center).Gap(8).Children(func() {
			chk := ui.ButtonBase(c.Key("settings-disable-proxy")).
				Size(16, 16).Radius(m.RadiusSM).Justify(ui.Center).AlignItems(ui.Center)
			if state.disableProxyDraft {
				chk.Background(tk.Accent)
				chk.Children(func() {
					ui.Icon(c, icons.Get("check")).Size(11, 11).TextColor(tk.BgSurface)
				})
			} else {
				chk.Border(m.Border, tk.Border)
			}
			if chk.Clicked() {
				state.disableProxyDraft = !state.disableProxyDraft
				val := state.disableProxyDraft
				props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
					if node.Request != nil {
						node.Request.Options.DisableProxy = val
					}
				})
			}
			ui.Text(c, "禁用代理 (直连模式 Direct Connection，绕过系统代理)").FontSize(m.FontSM).
				TextColor(tk.TextSecondary)
		})
	})
}

// 4. 底部响应区：支持 Body, Headers, Cookies 的 Tab 切换
func responsePane(c *ui.Context, props pageProps, n *domain.Node,
	state *reqSessionState, tk theme.Tokens, m theme.Metric,
) {
	ui.Column(c).Fill().Background(tk.BgSurface).Children(func() {
		// 响应头状态行
		ui.Row(c).Height(38).PaddingX(m.Space4).Gap(m.Space2).
			AlignItems(ui.Center).BorderWidth(m.Border, 0, m.Border, 0).
			BorderColor(tk.Border).Background(tk.BgElevated.Alpha(0.3)).
			Children(func() {
				if state.sending {
					ui.Box(c).Padding(2, 8).Radius(m.RadiusSM).
						Background(tk.Warning.Alpha(0.15)).Children(func() {
						ui.Text(c, "发送中...").FontSize(m.FontXS).Bold().TextColor(tk.Warning)
					})
					ui.Text(c, "正在等待服务器响应").FontSize(m.FontXS).TextColor(tk.TextMuted)
				} else if state.hasResponse {
					// 状态码 Badge
					statusBg := tk.Success.Alpha(0.15)
					statusColor := tk.Success
					if state.respStatus >= 400 || state.respStatus == 0 {
						statusBg = tk.Danger.Alpha(0.15)
						statusColor = tk.Danger
					} else if state.respStatus >= 300 {
						statusBg = tk.Info.Alpha(0.15)
						statusColor = tk.Info
					}
					statusLabel := fmt.Sprintf("%d OK", state.respStatus)
					if state.respStatus == 0 {
						statusLabel = "ERROR"
					} else if state.respStatus != 200 {
						statusLabel = fmt.Sprintf("%d", state.respStatus)
					}

					ui.Box(c).Padding(2, 8).Radius(m.RadiusSM).
						Background(statusBg).Children(func() {
						ui.Text(c, statusLabel).FontSize(m.FontXS).Bold().TextColor(statusColor)
					})

					// 耗时与响应大小
					ui.Text(c, state.respDuration).FontSize(m.FontXS).TextColor(tk.TextMuted)
					ui.Box(c).Size(3, 3).Radius(m.RadiusFull).Background(tk.TextMuted)
					ui.Text(c, state.respSize).FontSize(m.FontXS).TextColor(tk.TextMuted)
				} else {
					ui.Text(c, "尚未发送请求").FontSize(m.FontXS).TextColor(tk.TextMuted)
				}

				ui.Spacer(c)

				// 复制响应内容
				if state.hasResponse && len(state.respBody) > 0 {
					copyBtn := ui.ButtonBase(c.Key("resp-copy-btn")).Height(24).PaddingX(8).
						Radius(m.RadiusSM).AlignItems(ui.Center).Gap(4)
					if copyBtn.Hovered() {
						copyBtn.Background(tk.HoverSurface)
					}
					if copyBtn.Clicked() {
						c.Services().WriteClipboard(state.respBody)
					}
					copyBtn.Children(func() {
						ui.Icon(c, icons.Get("copy")).Size(11, 11).TextColor(tk.TextMuted)
						ui.Text(c, "复制").FontSize(m.FontXS).TextColor(tk.TextMuted)
					})
				}
			})

		headerCount := len(state.respHeaders)
		cookieCount := len(state.respCookies)
		totalTests := len(state.respTestResults)
		passedTests := 0
		for _, tr := range state.respTestResults {
			if tr.Passed {
				passedTests++
			}
		}
		testTabLabel := "Test Results"
		if totalTests > 0 {
			testTabLabel = fmt.Sprintf("Test Results (%d/%d)", passedTests, totalTests)
		}

		// 响应 Tab 栏（Body, Headers, Cookies, Test Results）
		ui.Row(c).PaddingX(m.Space4).Children(func() {
			respTabs := []string{
				"Body",
				fmt.Sprintf("Headers (%d)", headerCount),
				fmt.Sprintf("Cookies (%d)", cookieCount),
				testTabLabel,
			}
			tabsRowWithAction(c, respTabs, state.respTab, func(idx int) {
				state.respTab = idx
			}, nil)
		})

		ui.Divider(c).BorderColor(tk.Border)

		// 响应面板内容
		ui.Column(c).Grow(1).Padding(m.Space2, m.Space4).Children(func() {
			if state.sending {
				ui.Row(c).Height(80).AlignItems(ui.Center).Justify(ui.Center).Gap(8).Children(func() {
					ui.Icon(c, icons.Get("clock")).Size(16, 16).TextColor(tk.FocusRing)
					ui.Text(c, "正在向服务器发送请求，请稍候...").FontSize(m.FontSM).
						TextColor(tk.TextSecondary)
				})
				return
			}

			if !state.hasResponse {
				ui.Row(c).Height(80).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Text(c, "点击上方「发送」按钮发起网络请求，在此查看真实响应。").
						FontSize(m.FontSM).TextColor(tk.TextMuted)
				})
				return
			}

			if state.respError != "" {
				ui.Column(c).Padding(8, 0).Gap(6).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Icon(c, icons.Get("triangle-alert")).Size(16, 16).TextColor(tk.Danger)
						ui.Text(c, "请求遇到错误:").FontSize(m.FontSM).FontWeight(600).TextColor(tk.Danger)
					})
					ui.Text(c, state.respError).FontSize(m.FontSM).TextColor(tk.TextPrimary)
				})
			}

			switch state.respTab {
			case 0:
				// 使用 MyGo 原生内置代码编辑器展示响应体
				if len(state.respBody) > 0 {
					renderResponseBody(c, state, tk, m)
				} else if state.respError == "" {
					ui.Row(c).Height(60).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						ui.Text(c, "响应体内容为空").FontSize(m.FontSM).TextColor(tk.TextMuted)
					})
				}
			case 1:
				// 真实响应头列表
				renderResponseHeaders(c, state.respHeaders, tk, m)
			case 2:
				// 真实 Cookie 列表
				renderResponseCookies(c, state.respCookies, tk, m)
			case 3:
				// 测试断言执行结果
				renderTestResults(c, state.respTestResults, tk, m)
			}
		})
	})
	_ = props
}

// 测试断言列表展示
func renderTestResults(c *ui.Context, results []domain.TestResult, tk theme.Tokens, m theme.Metric) {
	if len(results) == 0 {
		ui.Row(c).Height(80).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, "本次请求未执行测试脚本。您可在上方「Scripts」Tab 中编写 pm.test(...) 测试断言。").
				FontSize(m.FontSM).TextColor(tk.TextMuted)
		})
		return
	}

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Grow(1).Gap(8).Padding(m.Space2, 0).Children(func() {
			for _, tr := range results {
				tr := tr
				ui.Row(c).Padding(8, 12).Radius(m.RadiusSM).AlignItems(ui.Center).Gap(10).
					Background(tk.HoverSurface.Alpha(0.4)).Children(func() {
					if tr.Passed {
						ui.Box(c).Padding(2, 6).Radius(m.RadiusSM).Background(tk.Success.Alpha(0.15)).Children(func() {
							ui.Text(c, "PASS").FontSize(m.FontXS).Bold().TextColor(tk.Success)
						})
						ui.Icon(c, icons.Get("check")).Size(14, 14).TextColor(tk.Success)
						ui.Text(c, tr.Name).FontSize(m.FontSM).Bold().TextColor(tk.TextPrimary)
					} else {
						ui.Box(c).Padding(2, 6).Radius(m.RadiusSM).Background(tk.Danger.Alpha(0.15)).Children(func() {
							ui.Text(c, "FAIL").FontSize(m.FontXS).Bold().TextColor(tk.Danger)
						})
						ui.Icon(c, icons.Get("triangle-alert")).Size(14, 14).TextColor(tk.Danger)
						ui.Column(c).Gap(4).Children(func() {
							ui.Text(c, tr.Name).FontSize(m.FontSM).Bold().TextColor(tk.TextPrimary)
							if tr.Error != "" {
								ui.Text(c, tr.Error).FontSize(m.FontXS).TextColor(tk.Danger)
							}
						})
					}
				})
			}
		})
	})
}

// 使用 MyGo 原生内置的 code editor 组件展示响应体
func renderResponseBody(c *ui.Context, state *reqSessionState, tk theme.Tokens, m theme.Metric) {
	if state.respEditor == nil {
		buf := editor.NewBuffer(state.respBody)
		buf.SetLanguage(editor.LanguageNamed("JSON"))
		state.respEditor = &editor.State{
			Buffer:      buf,
			ReadOnly:    true,
			SoftWrap:    true,
			LineNumbers: editor.LineNumbersOn,
		}
	} else if state.respEditor.Buffer.String() != state.respBody {
		state.respEditor.Buffer.Reset(state.respBody)
	}

	editor.View(c, state.respEditor).Fill()
}

// 响应头列表展示
func renderResponseHeaders(c *ui.Context, headers []domain.KV, tk theme.Tokens, m theme.Metric) {
	if len(headers) == 0 {
		ui.Row(c).Height(40).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, "响应中无返回标头").FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
		return
	}

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Grow(1).Children(func() {
			for i, h := range headers {
				i, h := i, h
				ui.Row(c).Height(26).PaddingX(8).AlignItems(ui.Center).
					BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border.Alpha(0.3)).
					Children(func() {
						ui.Text(c, h.Key).FontSize(m.FontXS).FontWeight(600).
							TextColor(tk.TextSecondary).Width(200)
						ui.Text(c, h.Value).FontSize(m.FontXS).
							TextColor(tk.TextPrimary).Grow(1)
					})
				_ = i
			}
		})
	})
}

// Cookie 列表展示
func renderResponseCookies(c *ui.Context, cookies []transport.ResponseCookie, tk theme.Tokens, m theme.Metric) {
	if len(cookies) == 0 {
		ui.Row(c).Height(40).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, "响应中未包含 Cookie").FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
		return
	}

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Grow(1).Children(func() {
			ui.Row(c).Height(26).PaddingX(8).AlignItems(ui.Center).
				Background(tk.HoverSurface.Alpha(0.4)).Children(func() {
				ui.Text(c, "NAME").FontSize(10).FontWeight(600).TextColor(tk.TextMuted).Width(120)
				ui.Text(c, "VALUE").FontSize(10).FontWeight(600).TextColor(tk.TextMuted).Width(140)
				ui.Text(c, "DOMAIN").FontSize(10).FontWeight(600).TextColor(tk.TextMuted).Width(120)
				ui.Text(c, "PATH").FontSize(10).FontWeight(600).TextColor(tk.TextMuted).Width(60)
				ui.Text(c, "EXPIRES").FontSize(10).FontWeight(600).TextColor(tk.TextMuted).Width(150)
				ui.Text(c, "HTTPONLY").FontSize(10).FontWeight(600).TextColor(tk.TextMuted).Width(80)
			})

			for _, ck := range cookies {
				ck := ck
				ui.Row(c).Height(28).PaddingX(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, ck.Name).FontSize(m.FontXS).TextColor(tk.TextPrimary).Width(120)
					ui.Text(c, ck.Value).FontSize(m.FontXS).TextColor(tk.Info).Width(140)
					ui.Text(c, ck.Domain).FontSize(m.FontXS).TextColor(tk.TextMuted).Width(120)
					ui.Text(c, ck.Path).FontSize(m.FontXS).TextColor(tk.TextMuted).Width(60)
					ui.Text(c, ck.Expires).FontSize(m.FontXS).TextColor(tk.TextMuted).Width(150)
					httpOnlyStr := ""
					if ck.HttpOnly {
						httpOnlyStr = "✓"
					}
					ui.Text(c, httpOnlyStr).FontSize(m.FontXS).TextColor(tk.Success).Width(80)
				})
			}
		})
	})
}

// tabsRowWithAction 是支持点击切换的下划线 Tab 行
func tabsRowWithAction(c *ui.Context, labels []string, active int,
	onSelect func(int), trailing func(),
) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Row(c).Height(32).Gap(m.Space5).
		AlignItems(ui.Center).BorderWidth(0, 0, m.Border, 0).
		BorderColor(tk.Border).Children(func() {
		for i, label := range labels {
			i := i
			isActive := i == active
			tab := ui.ButtonBase(c.Key(fmt.Sprintf("tab-btn-%s-%d", label, i))).
				Height(32).PaddingX(m.Space1).Justify(ui.Center).AlignItems(ui.Center)
			if isActive {
				tab.BorderWidth(0, 0, 2, 0).BorderColor(tk.FocusRing)
			}
			if tab.Clicked() && onSelect != nil {
				onSelect(i)
			}
			tab.Children(func() {
				color := tk.TextSecondary
				if isActive {
					color = tk.TextPrimary
				}
				text := ui.Text(c, label).FontSize(m.FontSM).TextColor(color)
				if isActive {
					text.FontWeight(600)
				}
			})
		}
		if trailing != nil {
			ui.Spacer(c)
			trailing()
		}
	})
}

// syncQueryToURL 将 Query 键值列表同步格式化回 URL 的 query string
func syncQueryToURL(rawURL string, query []domain.KV) string {
	baseURL := rawURL
	if idx := strings.Index(rawURL, "?"); idx >= 0 {
		baseURL = rawURL[:idx]
	}
	var pairs []string
	for _, q := range query {
		if !q.Enabled || strings.TrimSpace(q.Key) == "" {
			continue
		}
		if q.Value != "" {
			pairs = append(pairs, q.Key+"="+q.Value)
		} else {
			pairs = append(pairs, q.Key)
		}
	}
	if len(pairs) == 0 {
		return baseURL
	}
	return baseURL + "?" + strings.Join(pairs, "&")
}

// parseQueryFromURL 从 URL 的 query string 中解析出键值列表
func parseQueryFromURL(rawURL string) []domain.KV {
	idx := strings.Index(rawURL, "?")
	if idx < 0 || idx == len(rawURL)-1 {
		return []domain.KV{{ID: domain.NewID(), Enabled: true}}
	}
	queryStr := rawURL[idx+1:]
	parts := strings.Split(queryStr, "&")
	var res []domain.KV
	for _, part := range parts {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		res = append(res, domain.KV{
			ID:      domain.NewID(),
			Key:     k,
			Value:   v,
			Enabled: true,
		})
	}
	if len(res) == 0 {
		res = append(res, domain.KV{ID: domain.NewID(), Enabled: true})
	}
	return res
}

// cleanKVList 过滤掉完全空白的键值项
func cleanKVList(items []domain.KV) []domain.KV {
	var out []domain.KV
	for _, it := range items {
		if strings.TrimSpace(it.Key) != "" || strings.TrimSpace(it.Value) != "" {
			out = append(out, it)
		}
	}
	return out
}

// countActiveKV 计算非空项数量
func countActiveKV(items []domain.KV) int {
	cnt := 0
	for _, it := range items {
		if strings.TrimSpace(it.Key) != "" || strings.TrimSpace(it.Value) != "" {
			cnt++
		}
	}
	return cnt
}
