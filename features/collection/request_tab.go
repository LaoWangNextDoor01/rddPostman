package collection

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"rddpostman/components/theme"
	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
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
	urlDraft              string        // URL 本地编辑草稿
	urlEditing            bool          // 是否正处于 URL 文本输入聚焦编辑态
	urlFocusReq           bool          // 请求获得 URL 输入框焦点
	autocompleteOpen      bool          // 变量输入自动补全下拉开关
	autocompleteIndex     int           // 自动补全当前键盘选中索引
	autocompletePrefix    string        // 触发自动补全时已键入的前缀
	bodyRawDraft string        // Raw 请求体本地编辑草稿
	bodyEditor   *editor.State // Raw 请求体代码编辑器状态
	splitHeight  float32       // 请求区与响应区的上下分栏高度

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
	if len(headers) == 0 && (req == nil || req.Headers == nil) {
		headers = domain.DefaultRequestHeaders()
	}
	if len(headers) == 0 || strings.TrimSpace(headers[len(headers)-1].Key) != "" || strings.TrimSpace(headers[len(headers)-1].Value) != "" {
		headers = append(headers, domain.KV{ID: domain.NewID(), Enabled: true})
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
		reqTab:                1,
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
		bodyEditor: func() *editor.State {
			rawLang := "json"
			if req != nil && req.Body.Language != "" {
				rawLang = req.Body.Language
			}
			buf := editor.NewBuffer(rawBody)
			buf.SetLanguage(editor.LanguageNamed(rawLang))
			return &editor.State{
				Buffer:      buf,
				ReadOnly:    false,
				SoftWrap:    true,
				LineNumbers: editor.LineNumbersOn,
			}
		}(),
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

	root := ui.Column(c).Grow(1).MinHeight(0).Background(tk.BgSurface)

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

	// 触发手动持久化保存
	triggerSave := func() {
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

		reqName := strings.TrimSpace(title.draft)
		if reqName == "" {
			reqName = n.Name
		}
		if props.actions.saveRequest != nil {
			props.actions.saveRequest(activeID, currReq, n.Auth, reqName)
		}
	}

	if root.Shortcut(ui.Cmd, ui.KeyS) {
		triggerSave()
	}

	root.Children(func() {
		// 请求区与响应区纵向可拖拽分栏（对齐 Pixso 8-请求-Body 内边距 [10,12,0,12]）
		ui.SplitVertical(c, &state.splitHeight, func() {
			// 上半部：请求编辑区（顶部固定区域 + 下方内容独立滚动）
			ui.Column(c).Fill().MinHeight(0).Background(thV2.Of(c).BgBase).Children(func() {
				// 1. 顶部固定区域：面包屑导航 + URL 栏 + Tab 切换栏（固定在顶部，不随内容滚动）
				ui.Column(c).FillWidth().Padding(10, 12, 0, 12).Children(func() {
					reqBreadcrumb(c, props, n, activeID, title, triggerSave, tk, m)
					ui.Box(c).Height(6)
					reqLine(c, props, coll, n, activeID, state, tk, m)
					ui.Box(c).Height(8)
					reqConfigTabBar(c, state, thV2.Of(c))
				})

				// 2. Tab 下方具体配置面板内容（仅此处独立纵向滚动）
				ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
					ui.Column(c).FillWidth().Padding(8, 12, 10, 12).Children(func() {
						reqConfigTabContent(c, props, coll, n, activeID, state, tk, m)
					})
				})
			})
		}, func() {
			// 下半部：响应区（胶囊分段 Tab + 状态指标同行展示）
			responsePane(c, props, n, state, tk, m)
		}).Fill().MinHeight(0).SplitReset(340)
	})
}

// 1. 面包屑导航与可编辑请求名称（对齐 Pixso inst / row / breadcrumb）
func reqBreadcrumb(c *ui.Context, props pageProps, n *domain.Node, activeID string,
	title *reqTitleState, onSave func(), tk theme.Tokens, m theme.Metric,
) {
	tk2 := thV2.Of(c)
	crumbs := nodeTrailInWorkspace(props.collections, props.state, activeID)

	ui.Row(c).FillWidth().Height(22).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icons.Folder).FontSize(13).TextColor(tk2.TextSecondary)

		for i, cr := range crumbs {
			i, cr := i, cr
			isLast := i == len(crumbs)-1

			if i > 0 {
				ui.Text(c, "›").FontSize(12).TextColor(tk2.TextMuted).Margin(0, 2)
			}

			if isLast {
				if title.editing {
					in := ui.TextInputBase(c.Key("req-title-input"), &title.draft).
						Height(22).Padding(0, 6).
						Radius(4).Border(1, tk2.Focus).
						Background(tk2.BgSurface).
						FontSize(12).TextColor(tk2.TextPrimary)
					if title.fresh {
						in.Focus()
						title.fresh = false
					}
					if in.Submitted() || in.FocusLeft() {
						title.editing = false
						trimmed := strings.TrimSpace(title.draft)
						if trimmed != "" && trimmed != n.Name {
							n.Name = trimmed
							if props.state.isDraft(activeID) {
								props.state.setUnsaved(activeID, true)
							} else {
								props.actions.rename(activeID, trimmed)
							}
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
						Height(22).PaddingX(4).Radius(4).
						AlignItems(ui.Center).Gap(4).Cursor(ui.CursorPointer)
					switch {
					case btn.Pressed():
						btn.Background(tk2.BgSelected)
					case btn.Hovered():
						btn.Background(tk2.BgHover)
					}
					if btn.Clicked() {
						title.editing = true
						title.fresh = true
						title.draft = n.Name
					}
					btn.Children(func() {
						ui.Text(c, n.Name).FontSize(12).FontWeight(600).
							TextColor(tk2.TextPrimary).SingleLine()
					})
				}
			} else {
				btn := ui.ButtonBase(c.Key("req-crumb-" + cr.ID)).
					Height(22).PaddingX(4).Radius(4).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				switch {
				case btn.Pressed():
					btn.Background(tk2.BgSelected)
				case btn.Hovered():
					btn.Background(tk2.BgHover)
				}
				if btn.Clicked() {
					props.actions.openTab(cr.ID)
				}
				btn.Children(func() {
					textColor := tk2.TextSecondary
					if btn.Hovered() {
						textColor = tk2.TextPrimary
					}
					ui.Text(c, cr.Name).FontSize(12).TextColor(textColor).SingleLine()
				})
			}
		}

		ui.Spacer(c)

		// 右侧：Save 按钮（支持交互式保存）
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			isUnsaved := props.state.isUnsaved(activeID)
			saveBtn := ui.ButtonBase(c.Key("req-save-btn-" + activeID)).
				Height(24).Padding(0, 8).Gap(4).Radius(4).
				AlignItems(ui.Center).Cursor(ui.CursorPointer).
				Role(ui.RoleButton).Label("Save").
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			switch {
			case saveBtn.Pressed():
				saveBtn.Background(tk2.BgSelected)
			case saveBtn.Hovered():
				saveBtn.Background(tk2.BgHover)
			default:
				if isUnsaved {
					saveBtn.Background(tk2.Accent.Alpha(0.1)).Border(1, tk2.Accent.Alpha(0.4))
				} else {
					saveBtn.Background(ui.Transparent)
				}
			}
			if saveBtn.Clicked() && onSave != nil {
				onSave()
			}
			saveBtn.Children(func() {
				iconColor := tk2.TextSecondary
				textColor := tk2.TextSecondary
				if isUnsaved {
					iconColor = tk2.Accent
					textColor = tk2.Accent
				}
				ui.Icon(c, icons.Copy).FontSize(12).TextColor(iconColor)
				ui.Text(c, "Save").FontSize(12).FontWeight(500).TextColor(textColor).SingleLine()
				ui.Icon(c, icons.ChevronDown).FontSize(10).TextColor(tk2.TextMuted)
			})
		})
	})
}

// 2. 请求行：一体化边框 URL 栏（内嵌 Method 下拉）+ 科技蓝分裂 Send | v 按钮（对齐 Pixso inst / row / url bar）
func reqLine(c *ui.Context, props pageProps, coll *domain.Collection, n *domain.Node,
	activeID string, state *reqSessionState, tk theme.Tokens, m theme.Metric,
) {
	tk2 := thV2.Of(c)
	req := n.Request

	var allVars map[string]domain.Variable
	if props.actions.resolveVariables != nil {
		var collID, nodeID domain.ID
		if coll != nil {
			collID = coll.ID
		}
		if n != nil {
			nodeID = n.ID
		}
		allVars = props.actions.resolveVariables(collID, nodeID)
	}

	activeEnvName := "No Environment"
	for _, env := range props.environments {
		if env.ID == props.selectedEnv {
			activeEnvName = env.Name
			break
		}
	}

	urlVars := extractURLVariables(state.urlDraft, allVars)


	copyAsCurl := func() {
		state.queryDraft = parseQueryFromURL(state.urlDraft)
		state.pathVarsDraft = extractPathVariables(state.urlDraft, state.pathVarsDraft)
		currReq := *req.Clone()
		currReq.URL = strings.TrimSpace(state.urlDraft)
		currReq.PathVariables = cleanKVList(state.pathVarsDraft)
		currReq.Query = cleanKVList(state.queryDraft)
		currReq.Headers = cleanKVList(state.headersDraft)
		currReq.Body.Raw = state.bodyRawDraft
		currReq.Body.Fields = cleanKVList(state.bodyFieldsDraft)

		prepReq, _ := domain.Prepare(currReq, allVars)
		cmd := curl.ToCurl(prepReq, n.Auth)
		c.Services().WriteClipboard(cmd)
	}

	triggerSend := func() {
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

		if props.state.isDraft(string(n.ID)) {
			if n.Request != nil {
				*n.Request = *currReq.Clone()
			}
		} else if props.actions.updateNode != nil {
			props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
				if node.Request != nil {
					*node.Request = *currReq.Clone()
				}
			})
		}

		state.sending = true
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
			state.sending = false
			state.hasResponse = true
			state.respStatus = 200
			state.respDuration = "89 ms"
			state.respSize = "1.42 KB"
			state.respBody = "{\n  \"message\": \"Simulated local response\"\n}"
		}
	}

	ui.Column(c).FillWidth().Gap(2).Children(func() {
		ui.Row(c).FillWidth().Height(28).Gap(6).AlignItems(ui.Center).Children(func() {
			// 一体化 URL 容器（高 28、圆角 4、1px 描边，左侧内嵌 Method 下拉）
			urlBox := ui.Row(c.Key("req-url-box-" + activeID)).Grow(1).Height(28).AlignItems(ui.Center).
				Radius(4).Background(tk2.BgSurface).Border(1, tk2.Border)
			var urlFocused bool

			urlBox.Children(func() {
				methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
				methodBtn := ui.ButtonBase(c.Key("req-method-btn-"+activeID)).
					Height(26).Padding(0, 8, 0, 10).Gap(4).AlignItems(ui.Center).
					Radius(3).Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(req.Method)
				switch {
				case methodBtn.Pressed():
					methodBtn.Background(tk2.BgSelected)
				case methodBtn.Hovered():
					methodBtn.Background(tk2.BgHover)
				}
				if methodBtn.Clicked() {
					state.methodOpen = !state.methodOpen
				}
				methodBtn.Children(func() {
					ui.Text(c, req.Method).FontSize(12).Bold().
						TextColor(methodColor(tk, req.Method)).SingleLine()
					ui.Icon(c, icons.ChevronDown).FontSize(10).TextColor(tk2.TextMuted)
				})

				v2.Popover(c, methodBtn, &state.methodOpen, v2.PopoverOpts{Width: 112}, func(panel ui.Element) {
					ui.Column(c).Padding(4).Gap(2).Children(func() {
						for _, mth := range methods {
							mth := mth
							row := ui.ButtonBase(c.Key("method-opt-" + mth)).FillWidth().Height(26).
								PaddingX(8).Radius(4).AlignItems(ui.Center).Justify(ui.Start).
								Cursor(ui.CursorPointer).Label(mth)
							switch {
							case row.Pressed():
								row.Background(tk2.BgSelected)
							case row.Hovered():
								row.Background(tk2.BgHover)
							}
							if row.Clicked() {
								if req.Method != mth {
									req.Method = mth
									props.state.setUnsaved(activeID, true)
								}
								state.methodOpen = false
							}
							row.Children(func() {
								ui.Text(c, mth).FontSize(11).Bold().TextColor(methodColor(tk, mth))
							})
						}
					})
				})

				// 细竖分隔线
				ui.Box(c).Width(1).Height(16).Background(tk2.Border)

				// URL 编辑区：非聚焦且含完整变量时使用行内交互胶囊（画板 47/48/49），聚焦、正在输入未闭合变量或无变量时直接输入
				prefix, hasUnclosed := detectUnclosedVariable(state.urlDraft)
				hasCompleteVars := strings.Contains(state.urlDraft, "}}") && !hasUnclosed
				if !state.urlEditing && hasCompleteVars {
					renderTokenizedURLBar(c, props, activeID, state, allVars, activeEnvName)
				} else {
					in := ui.TextInputBase(c.Key("req-url-input"), &state.urlDraft).Grow(1).FillHeight().
						Padding(0, 8).FontSize(12).TextColor(tk2.TextPrimary).
						Placeholder("Enter URL or paste text").PlaceholderColor(tk2.TextMuted).
						Label("Request URL")
					if state.urlFocusReq {
						in.Focus()
						state.urlFocusReq = false
					}
					urlFocused = in.Focused()

					// 输入框内实时变量语法着色
					var ranges []ui.TextRange
					for _, v := range urlVars {
						token := "{{" + v.Name + "}}"
						start := 0
						u := state.urlDraft
						for {
							pos := strings.Index(u[start:], token)
							if pos < 0 {
								break
							}
							idx := start + pos
							clr := tk2.Accent
							if !v.Resolved {
								clr = tk2.Danger
							}
							ranges = append(ranges, ui.TextRange{
								Start:  idx,
								End:    idx + len(token),
								Color:  clr,
								Weight: 600,
							})
							start = idx + len(token)
						}
					}
					if len(ranges) > 0 {
						in.TextRanges(ranges...)
					}

					// 输入 {{ 时的智能下拉补全检测与快捷键监听（对齐画板 50）
					if hasUnclosed {
						state.autocompleteOpen = true
						state.autocompletePrefix = prefix
						allList := allAvailableVariables(allVars)
						filtered := filterVariables(allList, prefix)
						if len(filtered) > 0 {
							if in.Shortcut(0, ui.KeyDown) {
								state.autocompleteIndex = (state.autocompleteIndex + 1) % len(filtered)
							}
							if in.Shortcut(0, ui.KeyUp) {
								state.autocompleteIndex = (state.autocompleteIndex - 1 + len(filtered)) % len(filtered)
							}
							if in.Shortcut(0, ui.KeyEnter) {
								if state.autocompleteIndex >= 0 && state.autocompleteIndex < len(filtered) {
									completeVariableInURL(state, filtered[state.autocompleteIndex].Name)
									props.state.setUnsaved(activeID, true)
									state.urlEditing = false
								}
							}
						}
						if in.Shortcut(0, ui.KeyEscape) {
							state.autocompleteOpen = false
						}
					} else {
						state.autocompleteOpen = false
					}

					if in.Submitted() || in.FocusLeft() {
						state.queryDraft = parseQueryFromURL(state.urlDraft)
						state.pathVarsDraft = extractPathVariables(state.urlDraft, state.pathVarsDraft)
						trimmedURL := strings.TrimSpace(state.urlDraft)
						if trimmedURL != req.URL {
							props.state.setUnsaved(activeID, true)
						}
						state.urlEditing = false
						state.autocompleteOpen = false
					}
				}
			})
			if urlFocused {
				urlBox.Border(1, tk2.Focus).Shadow(0, 0, 0, 2, tk2.FocusRing)
			}

			// 如果正处于自动补全态，呼出画板 50 补全下拉浮层
			if state.autocompleteOpen {
				renderVariableAutocompleteDropdown(c, urlBox, state, allVars, activeEnvName, func(varName string) {
					completeVariableInURL(state, varName)
					props.state.setUnsaved(activeID, true)
					state.urlFocusReq = false
					state.urlEditing = false
				})
			}

			// 发送分裂主按钮（高 28、圆角 4、#336FFF 主色 + 30% 白叠主色分裂线 + 下拉箭头）
			if state.sending {
				cancelBtn := ui.ButtonBase(c.Key("req-cancel-btn")).
					Height(28).PaddingX(14).Radius(4).Center().
					Background(tk2.DangerSoft).Border(1, tk2.Danger).
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Cancel")
				if cancelBtn.Clicked() {
					if state.cancelFn != nil {
						state.cancelFn()
					}
					state.sending = false
				}
				cancelBtn.Children(func() {
					ui.Text(c, "Cancel").FontSize(12).FontWeight(500).TextColor(tk2.Danger).SingleLine()
				})
			} else {
				splitWrap := ui.Row(c.Key("req-send-split")).Height(28).Radius(4).Clip().
					Background(tk2.Accent).AlignItems(ui.Center)
				menuOpen := ui.Local(splitWrap, "send-menu-open", func() bool { return false })

				splitWrap.Children(func() {
					sendBtn := ui.ButtonBase(c.Key("req-send-btn")).
						Height(28).PaddingX(16).Center().
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Send").
						Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
					switch {
					case sendBtn.Pressed():
						sendBtn.Background(tk2.AccentPressed)
					case sendBtn.Hovered():
						sendBtn.Background(tk2.AccentHover)
					default:
						sendBtn.Background(tk2.Accent)
					}
					if sendBtn.Clicked() {
						triggerSend()
					}
					sendBtn.Children(func() {
						ui.Text(c, "Send").FontSize(12).FontWeight(500).TextColor(tk2.OnAccent).SingleLine()
					})

					// 30% 白叠主色分裂线（对齐 Pixso 组件库 · Button 06）
					ui.Box(c).Width(1).Height(28).Background(tk2.OnAccent.Alpha(0.3))

					dropBtn := ui.ButtonBase(c.Key("req-send-more")).
						Width(24).Height(28).Center().
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Send options").
						Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
					switch {
					case dropBtn.Pressed():
						dropBtn.Background(tk2.AccentPressed)
					case dropBtn.Hovered():
						dropBtn.Background(tk2.AccentHover)
					default:
						dropBtn.Background(tk2.Accent)
					}
					if dropBtn.Clicked() {
						*menuOpen = !*menuOpen
					}
					dropBtn.Children(func() {
						ui.Icon(c, icons.ChevronDown).FontSize(11).TextColor(tk2.OnAccent)
					})

					v2.Popover(c, dropBtn, menuOpen, v2.PopoverOpts{Width: 164}, func(panel ui.Element) {
						ui.Column(c).Padding(4).Gap(2).Children(func() {
							if v2.MenuButton(c, v2.MenuButtonProps{
								Icon: icons.Send, Key: "send-opt-run", Label: "Send Request",
							}).Clicked() {
								*menuOpen = false
								triggerSend()
							}
							if v2.MenuButton(c, v2.MenuButtonProps{
								Icon: icons.Code, Key: "send-opt-curl", Label: "Copy as cURL",
							}).Clicked() {
								*menuOpen = false
								copyAsCurl()
							}
						})
					})
				})
			}
		})


	})
}

// 3. 请求配置 Tab 切换区（兼容旧调用，聚合 Tab 栏与具体内容）
func reqConfigTabs(c *ui.Context, props pageProps, coll *domain.Collection, n *domain.Node,
	activeID string, state *reqSessionState, tk theme.Tokens, m theme.Metric,
) {
	reqConfigTabBar(c, state, thV2.Of(c))
	ui.Column(c).Grow(1).Padding(8, 0, 0, 0).Children(func() {
		reqConfigTabContent(c, props, coll, n, activeID, state, tk, m)
	})
}

// reqConfigTabBar 渲染固定在顶部的胶囊分段式请求配置 Tab 栏（对齐 Pixso row / request tabs）
func reqConfigTabBar(c *ui.Context, state *reqSessionState, tk2 thV2.Tokens) {
	paramCount := countActiveKV(state.queryDraft) + countActiveKV(state.pathVarsDraft)
	headerCount := countActiveKV(state.headersDraft)

	paramsLabel := "Params"
	if paramCount > 0 {
		paramsLabel = fmt.Sprintf("Params %d", paramCount)
	}
	headersLabel := "Headers"
	if headerCount > 0 {
		headersLabel = fmt.Sprintf("Headers %d", headerCount)
	}

	tabLabels := []string{
		"Docs",
		paramsLabel,
		"Authorization",
		headersLabel,
		"Body",
		"Scripts",
		"Settings",
	}

	tabsRowWithAction(c, tabLabels, state.reqTab, func(idx int) {
		state.reqTab = idx
	}, func() {
		ui.Text(c, "Cookies").FontSize(12).TextColor(tk2.TextMuted).SingleLine()
	})
}

// reqConfigTabContent 渲染当前选中的配置 Tab 对应内容面板
func reqConfigTabContent(c *ui.Context, props pageProps, coll *domain.Collection, n *domain.Node,
	activeID string, state *reqSessionState, tk theme.Tokens, m theme.Metric,
) {
	tk2 := thV2.Of(c)
	switch state.reqTab {
	case 0:
		// Docs 文档编辑（严格对齐 Pixso 4-请求-Docs：左侧 Markdown 编辑，右侧元信息面板）
		renderDocsEditor(c, props, coll, n, state, tk2, m)
	case 1:
		// Params 参数编辑（包含 Query 与 Path 变量，与 URL 保持双向联动）
		ui.Column(c).Grow(1).Gap(6).Children(func() {
			ui.Row(c).Height(20).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Query Params").FontSize(12).FontWeight(500).TextColor(tk2.TextSecondary)
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
					ui.Text(c, "Path Variables").FontSize(12).FontWeight(500).TextColor(tk2.TextSecondary)
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
	case 2:
		// Authorization 认证配置（带继承链生效计算展示）
		renderAuthEditor(c, props, coll, n, state, tk, m)
	case 3:
		// Headers 请求头编辑
		renderKVEditor(c, tk, m, "headers", &state.headersDraft, func(items []domain.KV) {
			props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
				if node.Request != nil {
					node.Request.Headers = cleanKVList(items)
				}
			})
		})
	case 4:
		// Body 请求体配置
		renderBodyEditor(c, props, coll, n, state, tk, m)
	case 5:
		// Scripts 脚本与测试断言
		renderScriptsEditor(c, props, coll, n, state, tk, m)
	case 6:
		// Settings 传输设置
		renderSettingsEditor(c, props, coll, n, state, tk, m)
	}
}

// kvListToBulk 把键值列表转换为纯文本（对齐 Pixso 15-请求体-BulkEdit）
func kvListToBulk(items []domain.KV) string {
	var sb strings.Builder
	for _, kv := range items {
		if strings.TrimSpace(kv.Key) == "" && strings.TrimSpace(kv.Value) == "" {
			continue
		}
		if kv.Enabled {
			sb.WriteString(kv.Key + ": " + kv.Value + "\n")
		} else {
			sb.WriteString("// " + kv.Key + ": " + kv.Value + "\n")
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// bulkToKVList 把纯文本反解析回键值列表
func bulkToKVList(text string) []domain.KV {
	lines := strings.Split(text, "\n")
	var res []domain.KV
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		enabled := true
		if strings.HasPrefix(trimmed, "//") {
			enabled = false
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
		}
		idx := strings.Index(trimmed, ":")
		if idx < 0 {
			res = append(res, domain.KV{ID: domain.NewID(), Key: trimmed, Value: "", Enabled: enabled})
		} else {
			key := strings.TrimSpace(trimmed[:idx])
			val := strings.TrimSpace(trimmed[idx+1:])
			res = append(res, domain.KV{ID: domain.NewID(), Key: key, Value: val, Enabled: enabled})
		}
	}
	if len(res) == 0 {
		res = append(res, domain.KV{ID: domain.NewID(), Enabled: true})
	}
	return res
}

// renderDocsEditor 严格对齐 Pixso 4-请求-Docs（32:76288）：左侧 Markdown 编辑与工具栏，右侧 Documentation 属性面板
func renderDocsEditor(c *ui.Context, props pageProps, coll *domain.Collection, n *domain.Node,
	state *reqSessionState, tk thV2.Tokens, m theme.Metric,
) {
	ui.Row(c).FillWidth().Gap(24).Children(func() {
		// 左侧编辑器列
		ui.Column(c).Grow(1).Gap(8).Children(func() {
			ui.Row(c).Height(26).Gap(4).AlignItems(ui.Center).Children(func() {
				for _, tb := range []struct {
					key, label string
					icon       *ui.SVG
					insert     string
				}{
					{"bold", "Bold", icons.Bold, "**bold text**"},
					{"italic", "Italic", icons.Italic, "*italic text*"},
					{"code", "Code", icons.Code, "`code`"},
					{"list", "List", icons.ListOrdered, "1. Item\n2. Item"},
				} {
					tb := tb
					btn := ui.ButtonBase(c.Key("docs-tb-" + tb.key + "-" + string(n.ID))).
						Height(24).PaddingX(8).Radius(4).Border(1, tk.Border).
						Background(tk.BgSurface).Center().Cursor(ui.CursorPointer).
						Role(ui.RoleButton).Label(tb.label)
					switch {
					case btn.Pressed():
						btn.Background(tk.BgSelected)
					case btn.Hovered():
						btn.Background(tk.BgHover)
					}
					if btn.Clicked() {
						if n.Description == "" {
							n.Description = tb.insert
						} else {
							n.Description += "\n" + tb.insert
						}
						props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
							node.Description = n.Description
						})
					}
					btn.Children(func() {
						if tb.icon != nil {
							ui.Icon(c, tb.icon).FontSize(11).TextColor(tk.TextSecondary)
						} else {
							ui.Text(c, tb.label).FontSize(11).FontWeight(500).TextColor(tk.TextSecondary)
						}
					})
				}
			})

			descIn := ui.TextAreaBase(c, &n.Description).FillWidth().MinHeight(160).
				MaxHeight(320).Padding(12, 14).Radius(4).
				Border(1, tk.Border).Background(tk.BgSubtle).
				FontSize(12).FixedLineHeight(20).TextColor(tk.TextPrimary)
			if descIn.FocusLeft() {
				props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
					node.Description = n.Description
				})
			}
			ui.Text(c, "Markdown is supported. Use the toolbar above to format your documentation.").
				FontSize(11).TextColor(tk.TextMuted)
		})

		// 右侧 Documentation 信息面板（对齐 Pixso 32:76796 col / docs info，宽 260）
		ui.Column(c).Width(260).Gap(10).Children(func() {
			ui.Text(c, "Documentation").FontSize(13).FontWeight(600).TextColor(tk.TextPrimary)
			ui.Text(c, "Add documentation for this request so that everyone on your team knows how to use it.").
				FontSize(12).TextColor(tk.TextSecondary)

			ui.Box(c).FillWidth().Height(1).Background(tk.Border)

			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "METHOD").FontSize(11).TextColor(tk.TextMuted)
				mth := "GET"
				if n.Request != nil && n.Request.Method != "" {
					mth = n.Request.Method
				}
				ui.Text(c, mth).FontSize(12).Bold().TextColor(methodColor(theme.Tokens{}, mth))
			})

			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "LAST UPDATED").FontSize(11).TextColor(tk.TextMuted)
				ui.Text(c, n.UpdatedAt.Format("2006-01-02 15:04")).FontSize(12).TextColor(tk.TextPrimary)
			})

			ui.Text(c, "Learn about documenting APIs").FontSize(12).TextColor(tk.Accent).Cursor(ui.CursorPointer)
		})
	})
}

// 通用网格型键值对表格编辑器（对齐 Pixso 5-请求-Params & 15-请求体-BulkEdit：支持 Key-Value 表格与 Bulk Edit 纯文本双向切换）
func renderKVEditor(c *ui.Context, tk theme.Tokens, m theme.Metric,
	keyPrefix string, items *[]domain.KV, onPersist func([]domain.KV),
) {
	tk2 := thV2.Of(c)
	if items == nil {
		return
	}
	if len(*items) == 0 {
		*items = []domain.KV{{ID: domain.NewID(), Enabled: true}}
	}

	col := ui.Column(c.Key("kv-editor-" + keyPrefix)).FillWidth().Radius(4).
		Border(1, tk2.Border).Background(tk2.BgBase).Clip()
	bulkMode := ui.Local(col, "bulk-mode", func() bool { return false })
	bulkText := ui.Local(col, "bulk-text", func() string { return kvListToBulk(*items) })

	col.Children(func() {
		// 表头（对齐 Pixso：Key | Value | Description + 右侧 Bulk Edit 切换）
		ui.Row(c).FillWidth().Height(28).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tk2.Border).
			Children(func() {
				if *bulkMode {
					ui.Row(c).FillWidth().FillHeight().Padding(0, 8).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
						ui.Text(c, "Rows (Key: Value)").FontSize(11).FontWeight(500).TextColor(tk2.TextSecondary)
						toggleBtn := ui.ButtonBase(c.Key(keyPrefix + "-toggle-kv")).
							Height(22).PaddingX(6).Radius(4).AlignItems(ui.Center).Cursor(ui.CursorPointer).
							Role(ui.RoleButton).Label("Key-Value Edit")
						if toggleBtn.Clicked() {
							*items = bulkToKVList(*bulkText)
							*bulkMode = false
							onPersist(*items)
						}
						toggleBtn.Children(func() {
							ui.Text(c, "Key-Value Edit").FontSize(11).FontWeight(500).TextColor(tk2.Accent)
						})
					})
				} else {
					ui.Box(c).Width(32)
					ui.Row(c).Width(200).FillHeight().Padding(0, 8).AlignItems(ui.Center).
						BorderWidth(0, 1, 0, 0).BorderColor(tk2.Border).Children(func() {
						ui.Text(c, "Key").FontSize(11).FontWeight(500).TextColor(tk2.TextSecondary)
					})
					ui.Row(c).Grow(1).FillHeight().Padding(0, 8).AlignItems(ui.Center).
						BorderWidth(0, 1, 0, 0).BorderColor(tk2.Border).Children(func() {
						ui.Text(c, "Value").FontSize(11).FontWeight(500).TextColor(tk2.TextSecondary)
					})
					ui.Row(c).Width(250).FillHeight().Padding(0, 8).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
						ui.Text(c, "Description").FontSize(11).FontWeight(500).TextColor(tk2.TextSecondary)
						ui.Row(c).AlignItems(ui.Center).Gap(4).Children(func() {
							if keyPrefix == "headers" {
								resetBtn := ui.ButtonBase(c.Key(keyPrefix + "-reset-defaults")).
									Height(22).PaddingX(6).Radius(4).AlignItems(ui.Center).Cursor(ui.CursorPointer).
									Role(ui.RoleButton).Label("Reset Defaults")
								if resetBtn.Clicked() {
									*items = append(domain.DefaultRequestHeaders(), domain.KV{ID: domain.NewID(), Enabled: true})
									onPersist(*items)
								}
								resetBtn.Children(func() {
									ui.Text(c, "Reset Defaults").FontSize(11).FontWeight(500).TextColor(tk2.TextMuted)
								})
							}
							toggleBtn := ui.ButtonBase(c.Key(keyPrefix + "-toggle-bulk")).
								Height(22).PaddingX(6).Radius(4).AlignItems(ui.Center).Cursor(ui.CursorPointer).
								Role(ui.RoleButton).Label("Bulk Edit")
							if toggleBtn.Clicked() {
								*bulkText = kvListToBulk(*items)
								*bulkMode = true
							}
							toggleBtn.Children(func() {
								ui.Text(c, "Bulk Edit").FontSize(11).FontWeight(500).TextColor(tk2.Accent)
							})
						})
					})
					ui.Box(c).Width(28)
				}
			})

		if *bulkMode {
			// Bulk 纯文本模式（对齐 Pixso 34:9849 div / bulk editor）
			ui.Column(c).FillWidth().Padding(8, 12).Gap(6).Children(func() {
				in := ui.TextAreaBase(c, bulkText).FillWidth().MinHeight(120).MaxHeight(240).
					FontSize(12).FixedLineHeight(20).TextColor(tk2.TextPrimary).
					Padding(6, 8).Radius(4).Border(1, tk2.Border).Background(tk2.BgSubtle)
				if in.FocusLeft() {
					*items = bulkToKVList(*bulkText)
					onPersist(*items)
				}
				ui.Text(c, "Enter one key-value pair per line (key: value, prefix with // to disable)").
					FontSize(11).TextColor(tk2.TextMuted)
			})
			return
		}

		// 表格模式行渲染
		list := *items
		for i := 0; i < len(list); i++ {
			i := i
			kv := &(*items)[i]
			isEmptyTail := i == len(list)-1 && strings.TrimSpace(kv.Key) == "" && strings.TrimSpace(kv.Value) == ""

			row := ui.Row(c.Key(fmt.Sprintf("%s-row-%d", keyPrefix, i))).
				FillWidth().Height(30).AlignItems(ui.Center)
			if i < len(list)-1 {
				row.BorderWidth(0, 0, 1, 0).BorderColor(tk2.Border)
			}
			row.Children(func() {
				// 1. 复选框（启用/停用）
				ui.Row(c).Width(32).FillHeight().Center().Children(func() {
					chk := ui.ButtonBase(c.Key(fmt.Sprintf("%s-chk-%d", keyPrefix, i))).
						Size(15, 15).Radius(3).Center().Cursor(ui.CursorPointer).
						Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
					if kv.Enabled && !isEmptyTail {
						switch {
						case chk.Pressed():
							chk.Background(tk2.AccentPressed)
						case chk.Hovered():
							chk.Background(tk2.AccentHover)
						default:
							chk.Background(tk2.Accent)
						}
						chk.Children(func() {
							ui.Icon(c, icons.Check).Size(10, 10).TextColor(tk2.OnAccent)
						})
					} else {
						switch {
						case chk.Pressed():
							chk.Background(tk2.BgSubtle).Border(1, tk2.Accent)
						case chk.Hovered():
							chk.Border(1, tk2.Accent)
						default:
							chk.Border(1, tk2.BorderStrong).Background(tk2.BgSurface)
						}
					}
					if chk.Clicked() {
						kv.Enabled = !kv.Enabled
						onPersist(*items)
					}
				})

				// 2. Key 单元格
				ui.Row(c).Width(200).FillHeight().AlignItems(ui.Center).
					BorderWidth(0, 1, 0, 0).BorderColor(tk2.Border).Children(func() {
					keyIn := ui.TextInputBase(c.Key(fmt.Sprintf("%s-key-%d", keyPrefix, i)), &kv.Key).
						Fill().Padding(0, 8).FontSize(12).TextColor(tk2.TextPrimary).
						Placeholder("Key").PlaceholderColor(tk2.TextMuted)
					if keyIn.Changed() {
						if i == len(*items)-1 && (kv.Key != "" || kv.Value != "") {
							*items = append(*items, domain.KV{ID: domain.NewID(), Enabled: true})
						}
					}
					if keyIn.FocusLeft() || keyIn.Submitted() {
						onPersist(*items)
					}
				})

				// 3. Value 单元格
				ui.Row(c).Grow(1).FillHeight().AlignItems(ui.Center).
					BorderWidth(0, 1, 0, 0).BorderColor(tk2.Border).Children(func() {
					valIn := ui.TextInputBase(c.Key(fmt.Sprintf("%s-val-%d", keyPrefix, i)), &kv.Value).
						Fill().Padding(0, 8).FontSize(12).TextColor(tk2.TextPrimary).
						Placeholder("Value").PlaceholderColor(tk2.TextMuted)
					if valIn.Changed() {
						if i == len(*items)-1 && (kv.Key != "" || kv.Value != "") {
							*items = append(*items, domain.KV{ID: domain.NewID(), Enabled: true})
						}
					}
					if valIn.FocusLeft() || valIn.Submitted() {
						onPersist(*items)
					}
				})

				// 4. Description 单元格
				ui.Row(c).Width(250).FillHeight().AlignItems(ui.Center).Children(func() {
					descIn := ui.TextInputBase(c.Key(fmt.Sprintf("%s-desc-%d", keyPrefix, i)), &kv.Desc).
						Fill().Padding(0, 8).FontSize(12).TextColor(tk2.TextSecondary).
						Placeholder("Description").PlaceholderColor(tk2.TextMuted)
					if descIn.FocusLeft() || descIn.Submitted() {
						onPersist(*items)
					}
				})

				// 5. 删除行按钮
				ui.Row(c).Width(28).FillHeight().Center().Children(func() {
					if !isEmptyTail {
						delBtn := ui.ButtonBase(c.Key(fmt.Sprintf("%s-del-%d", keyPrefix, i))).
							Size(20, 20).Radius(4).Center().Cursor(ui.CursorPointer).
							Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
						switch {
						case delBtn.Pressed():
							delBtn.Background(tk2.BgSelected)
						case delBtn.Hovered():
							delBtn.Background(tk2.BgHover)
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
							ui.Icon(c, icons.X).Size(11, 11).TextColor(tk2.TextMuted)
						})
					}
				})
			})
		}
	})
}

// 认证配置面板（对齐 Pixso 6-请求-Authorization，采用标准 v2.Select 组件）
func renderAuthEditor(c *ui.Context, props pageProps, coll *domain.Collection, n *domain.Node,
	state *reqSessionState, tk theme.Tokens, m theme.Metric,
) {
	currType := string(n.Auth.Type)
	if currType == "" {
		currType = "inherit"
	}

	ui.Column(c).Grow(1).Gap(12).Children(func() {
		ui.Row(c).Height(32).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, "Type:").FontSize(12).FontWeight(500).TextColor(tk.TextSecondary)

			v2.Select(c, v2.SelectOpts{
				Key:       "req-auth-select-" + string(n.ID),
				Label:     "Auth Type",
				Width:     240,
				Height:    30,
				MenuWidth: 260,
				Value:     currType,
				Options: []v2.SelectOption{
					{Value: "inherit", Label: "Inherit auth from parent"},
					{Value: "bearer", Label: "Bearer Token"},
					{Value: "basic", Label: "Basic Auth"},
					{Value: "apikey", Label: "API Key"},
					{Value: "none", Label: "No Auth"},
				},
				OnSelect: func(val string) {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						node.Auth.Type = domain.AuthType(val)
					})
				},
			})
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

	tk2 := thV2.Of(c)
	ui.Column(c).Grow(1).Gap(8).Children(func() {
		// 1. Body 模式单选组（若为 raw 则把格式选择与 Beautify 置于本行末尾，不另起一行）
		ui.Row(c).Height(26).AlignItems(ui.Center).Gap(16).Children(func() {
			for _, item := range []struct{ key, label string }{
				{"none", "none"},
				{"form-data", "form-data"},
				{"urlencoded", "x-www-form-urlencoded"},
				{"raw", "raw"},
				{"binary", "binary"},
			} {
				item := item
				isSelected := mode == item.key
				btn := ui.ButtonBase(c.Key("body-mode-" + item.key)).
					Height(24).Padding(0, 4).Gap(6).
					Radius(4).AlignItems(ui.Center).Justify(ui.Start).
					Cursor(ui.CursorPointer).Role(ui.RoleRadio).Label(item.label).
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				switch {
				case btn.Pressed():
					btn.Background(tk2.BgSelected)
				case btn.Hovered():
					btn.Background(tk2.BgHover)
				}
				if btn.Clicked() {
					props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
						if node.Request != nil {
							node.Request.Body.Mode = domain.BodyMode(item.key)
						}
					})
					props.state.setUnsaved(string(n.ID), true)
				}
				btn.Children(func() {
					ring := ui.Box(c).Size(12, 12).Radius(6).Center()
					if isSelected {
						ring.Border(1.5, tk2.Accent).Children(func() {
							ui.Box(c).Size(6, 6).Radius(3).Background(tk2.Accent)
						})
					} else {
						ring.Border(1, tk2.BorderStrong)
					}
					weight := 400
					textColor := tk2.TextSecondary
					if isSelected {
						weight = 500
						textColor = tk2.TextPrimary
					}
					ui.Text(c, item.label).FontSize(12).FontWeight(weight).TextColor(textColor).SingleLine()
				})
			}

			// 当为 raw 模式时，格式选择下拉框与 Beautify 按钮紧挨着 binary（不单独起一行，也不推到末尾）
			if mode == "raw" {
				rawLang := req.Body.Language
				if rawLang == "" {
					rawLang = "json"
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					v2.Select(c, v2.SelectOpts{
						Key:       "req-raw-format-" + string(n.ID),
						Label:     "Format",
						Width:     96,
						Height:    24,
						MenuWidth: 120,
						Value:     rawLang,
						Options: []v2.SelectOption{
							{Value: "json", Label: "JSON"},
							{Value: "text", Label: "Text"},
							{Value: "javascript", Label: "JavaScript"},
							{Value: "html", Label: "HTML"},
							{Value: "xml", Label: "XML"},
						},
						OnSelect: func(val string) {
							props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
								if node.Request != nil {
									node.Request.Body.Language = val
								}
							})
							props.state.setUnsaved(string(n.ID), true)
							if state.bodyEditor != nil {
								state.bodyEditor.Buffer.SetLanguage(editor.LanguageNamed(val))
							}
						},
					})

					beautifyBtn := ui.ButtonBase(c.Key("raw-beautify-" + string(n.ID))).
						Height(22).PaddingX(6).Radius(4).AlignItems(ui.Center).
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Beautify")
					switch {
					case beautifyBtn.Pressed():
						beautifyBtn.Background(tk2.BgSelected)
					case beautifyBtn.Hovered():
						beautifyBtn.Background(tk2.BgHover)
					}
					if beautifyBtn.Clicked() {
						trimmed := strings.TrimSpace(state.bodyRawDraft)
						if trimmed != "" {
							var parsed any
							if json.Unmarshal([]byte(trimmed), &parsed) == nil {
								if indented, err := json.MarshalIndent(parsed, "", "  "); err == nil {
									state.bodyRawDraft = string(indented)
									if state.bodyEditor != nil {
										state.bodyEditor.Buffer.Reset(state.bodyRawDraft)
									}
									props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
										if node.Request != nil {
											node.Request.Body.Raw = state.bodyRawDraft
										}
									})
									props.state.setUnsaved(string(n.ID), true)
								}
							}
						}
					}
					beautifyBtn.Children(func() {
						ui.Text(c, "Beautify").FontSize(12).TextColor(tk2.Accent).SingleLine()
					})
				})
			}
		})

		ui.Box(c).Height(1).Background(tk2.Border.Alpha(0.6))

		// 2. 模式内容展示（对齐 Pixso 各子画板）
		switch mode {
		case "none":
			ui.Row(c).Height(80).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(c, "This request does not have a body").FontSize(12).
					TextColor(tk2.TextMuted)
			})
		case "raw":
			rawLang := req.Body.Language
			if rawLang == "" {
				rawLang = "json"
			}
			if state.bodyEditor == nil {
				buf := editor.NewBuffer(state.bodyRawDraft)
				buf.SetLanguage(editor.LanguageNamed(rawLang))
				state.bodyEditor = &editor.State{
					Buffer:      buf,
					ReadOnly:    false,
					SoftWrap:    true,
					LineNumbers: editor.LineNumbersOn,
				}
			} else {
				if l := editor.LanguageNamed(rawLang); l != state.bodyEditor.Buffer.Language() {
					state.bodyEditor.Buffer.SetLanguage(l)
				}
			}

			// 同步代码编辑器中的编辑内容至草稿与持久化数据模型
			if state.bodyEditor != nil {
				curText := state.bodyEditor.Buffer.String()
				if curText != state.bodyRawDraft {
					state.bodyRawDraft = curText
					if curText != req.Body.Raw {
						props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
							if node.Request != nil {
								node.Request.Body.Raw = curText
							}
						})
						props.state.setUnsaved(string(n.ID), true)
					}
				}
			}

			// 使用与底部 response body 一致的 code editor 组件
			ui.Box(c.Key("req-body-raw-editor-"+string(n.ID))).FillWidth().Height(240).Radius(4).
				Border(1, tk2.Border).Background(tk2.BgSurface).Clip().Children(func() {
				editor.View(c, state.bodyEditor).Fill()
			})
		case "form-data", "urlencoded":
			renderKVEditor(c, tk, m, "body-fields", &state.bodyFieldsDraft, func(items []domain.KV) {
				props.actions.updateNode(coll.ID, n.ID, func(node *domain.Node) {
					if node.Request != nil {
						node.Request.Body.Fields = cleanKVList(items)
					}
				})
			})
		case "binary":
			// 二进制文件上传投放区（对齐 Pixso 14-请求体-Binary：节点 32:96001）
			dropZone := ui.Column(c.Key("binary-dropzone")).FillWidth().Height(140).Radius(4).
				Border(1, tk2.Border).Background(tk2.BgSurface).Center().Gap(8)
			dropZone.Children(func() {
				ui.Text(c, "Drag and drop a file here, or click to browse").FontSize(12).
					TextColor(tk2.TextMuted)
				selFileBtn := ui.ButtonBase(c.Key("binary-select-file")).Height(28).PaddingX(12).
					Radius(4).Border(1, tk2.Border).Background(tk2.BgSurface).Center().
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Select File")
				switch {
				case selFileBtn.Pressed():
					selFileBtn.Background(tk2.BgSelected)
				case selFileBtn.Hovered():
					selFileBtn.Background(tk2.BgHover)
				}
				selFileBtn.Children(func() {
					ui.Text(c, "Select File").FontSize(12).FontWeight(500).TextColor(tk2.TextPrimary).SingleLine()
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
	tk2 := thV2.Of(c)
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
				Size(16, 16).Radius(m.RadiusSM).Justify(ui.Center).AlignItems(ui.Center).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			if followRedirects {
				switch {
				case chk.Pressed():
					chk.Background(tk2.AccentPressed)
				case chk.Hovered():
					chk.Background(tk2.AccentHover)
				default:
					chk.Background(tk.Accent)
				}
				chk.Children(func() {
					ui.Icon(c, icons.Get("check")).Size(11, 11).TextColor(tk.BgSurface)
				})
			} else {
				switch {
				case chk.Pressed():
					chk.Background(tk2.BgSubtle).Border(m.Border, tk2.Accent)
				case chk.Hovered():
					chk.Border(m.Border, tk2.Accent)
				default:
					chk.Border(m.Border, tk.Border)
				}
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
				Size(16, 16).Radius(m.RadiusSM).Justify(ui.Center).AlignItems(ui.Center).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			if verifySSL {
				switch {
				case chk.Pressed():
					chk.Background(tk2.AccentPressed)
				case chk.Hovered():
					chk.Background(tk2.AccentHover)
				default:
					chk.Background(tk.Accent)
				}
				chk.Children(func() {
					ui.Icon(c, icons.Get("check")).Size(11, 11).TextColor(tk.BgSurface)
				})
			} else {
				switch {
				case chk.Pressed():
					chk.Background(tk2.BgSubtle).Border(m.Border, tk2.Accent)
				case chk.Hovered():
					chk.Border(m.Border, tk2.Accent)
				default:
					chk.Border(m.Border, tk.Border)
				}
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
				Size(16, 16).Radius(m.RadiusSM).Justify(ui.Center).AlignItems(ui.Center).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			if useCookieJar {
				switch {
				case chk.Pressed():
					chk.Background(tk2.AccentPressed)
				case chk.Hovered():
					chk.Background(tk2.AccentHover)
				default:
					chk.Background(tk.Accent)
				}
				chk.Children(func() {
					ui.Icon(c, icons.Get("check")).Size(11, 11).TextColor(tk.BgSurface)
				})
			} else {
				switch {
				case chk.Pressed():
					chk.Background(tk2.BgSubtle).Border(m.Border, tk2.Accent)
				case chk.Hovered():
					chk.Border(m.Border, tk2.Accent)
				default:
					chk.Border(m.Border, tk.Border)
				}
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
				Size(16, 16).Radius(m.RadiusSM).Justify(ui.Center).AlignItems(ui.Center).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			if state.disableProxyDraft {
				switch {
				case chk.Pressed():
					chk.Background(tk2.AccentPressed)
				case chk.Hovered():
					chk.Background(tk2.AccentHover)
				default:
					chk.Background(tk.Accent)
				}
				chk.Children(func() {
					ui.Icon(c, icons.Get("check")).Size(11, 11).TextColor(tk.BgSurface)
				})
			} else {
				switch {
				case chk.Pressed():
					chk.Background(tk2.BgSubtle).Border(m.Border, tk2.Accent)
				case chk.Hovered():
					chk.Border(m.Border, tk2.Accent)
				default:
					chk.Border(m.Border, tk.Border)
				}
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

// 4. 底部响应区：单行胶囊分段 Tab + 状态码/耗时/体积同行展示（对齐 Pixso section / response）
func responsePane(c *ui.Context, props pageProps, n *domain.Node,
	state *reqSessionState, tk theme.Tokens, m theme.Metric,
) {
	tk2 := thV2.Of(c)
	ui.Column(c).Fill().MinHeight(0).Background(tk2.BgBase).
		BorderWidth(1, 0, 0, 0).BorderColor(tk2.Border).Children(func() {
		headerCount := len(state.respHeaders)
		cookieCount := len(state.respCookies)
		totalTests := len(state.respTestResults)
		passedTests := 0
		for _, tr := range state.respTestResults {
			if tr.Passed {
				passedTests++
			}
		}

		headersTabLabel := "Headers"
		if headerCount > 0 {
			headersTabLabel = fmt.Sprintf("Headers %d", headerCount)
		}
		cookiesTabLabel := "Cookies"
		if cookieCount > 0 {
			cookiesTabLabel = fmt.Sprintf("Cookies %d", cookieCount)
		}
		testTabLabel := "Test Results"
		if totalTests > 0 {
			testTabLabel = fmt.Sprintf("Test Results (%d/%d)", passedTests, totalTests)
		}

		// 响应头行：左侧胶囊 Tab + 右侧状态码 / 耗时 / 体积 / 复制（对齐 Pixso row / response header）
		ui.Row(c).FillWidth().Padding(8, 12, 6, 12).AlignItems(ui.Center).Children(func() {
			respTabs := []string{
				"Body",
				headersTabLabel,
				cookiesTabLabel,
				testTabLabel,
			}
			tabsRowWithAction(c, respTabs, state.respTab, func(idx int) {
				state.respTab = idx
			}, func() {
				ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
					if state.sending {
						ui.Text(c, "Sending...").FontSize(11).FontWeight(600).TextColor(tk2.Warning)
					} else if state.hasResponse {
						statusColor := tk2.Success
						if state.respStatus >= 400 || state.respStatus == 0 {
							statusColor = tk2.Danger
						} else if state.respStatus >= 300 {
							statusColor = tk2.Info
						}
						statusLabel := fmt.Sprintf("%d OK", state.respStatus)
						if state.respStatus == 0 {
							statusLabel = "ERROR"
						} else if state.respStatus != 200 {
							statusLabel = fmt.Sprintf("%d", state.respStatus)
						}
						ui.Text(c, statusLabel).FontSize(11).FontWeight(600).TextColor(statusColor).SingleLine()
						ui.Text(c, state.respDuration).FontSize(11).TextColor(tk2.TextMuted).SingleLine()
						ui.Text(c, state.respSize).FontSize(11).TextColor(tk2.TextMuted).SingleLine()

						if len(state.respBody) > 0 {
							copyBtn := ui.ButtonBase(c.Key("resp-copy-btn")).Height(22).PaddingX(6).
								Radius(4).AlignItems(ui.Center).Gap(4).Cursor(ui.CursorPointer)
							if copyBtn.Hovered() {
								copyBtn.Background(tk2.BgHover)
							}
							if copyBtn.Clicked() {
								c.Services().WriteClipboard(state.respBody)
							}
							copyBtn.Children(func() {
								ui.Icon(c, icons.Copy).Size(11, 11).TextColor(tk2.TextSecondary)
								ui.Text(c, "Copy").FontSize(11).TextColor(tk2.TextSecondary)
							})
						}
					}
				})
			})
		})

		// 响应面板内容（自适应剩余高度并支持内部滚动）
		ui.Column(c).Grow(1).MinHeight(0).Padding(4, 12, 8, 12).Children(func() {
			if state.sending {
				ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Center).Justify(ui.Center).Gap(8).Children(func() {
					ui.Icon(c, icons.Clock).Size(15, 15).TextColor(tk2.Accent)
					ui.Text(c, "Sending request...").FontSize(12).TextColor(tk2.TextSecondary)
				})
				return
			}

			if !state.hasResponse {
				ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Text(c, "Click Send to get a response").
						FontSize(12).TextColor(tk2.TextMuted)
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

	ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
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

	editor.View(c, state.respEditor).Grow(1).MinHeight(0)
}

// 响应头列表展示
func renderResponseHeaders(c *ui.Context, headers []domain.KV, tk theme.Tokens, m theme.Metric) {
	if len(headers) == 0 {
		ui.Row(c).Height(40).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, "响应中无返回标头").FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
		return
	}

	ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
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

// Cookie 列表展示（单行超出省略并支持点击复制完整内容）
func renderResponseCookies(c *ui.Context, cookies []transport.ResponseCookie, tk theme.Tokens, m theme.Metric) {
	if len(cookies) == 0 {
		ui.Row(c).Height(40).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, "响应中未包含 Cookie").FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
		return
	}

	tk2 := thV2.Of(c)
	scroll := ui.Scroll(c).Grow(1).MinHeight(0)
	copiedKey := ui.Local(scroll, "ck-copied-key", func() string { return "" })

	scroll.Children(func() {
		ui.Column(c).Grow(1).MinWidth(700).Padding(0, 4).Children(func() {
			ui.Row(c).Height(28).PaddingX(8).Gap(8).AlignItems(ui.Center).
				Background(tk2.BgSubtle).
				BorderWidth(0, 0, 1, 0).BorderColor(tk2.Border).Children(func() {
				ui.Text(c, "NAME").FontSize(10).FontWeight(600).TextColor(tk2.TextMuted).Width(130).SingleLine()
				ui.Text(c, "VALUE").FontSize(10).FontWeight(600).TextColor(tk2.TextMuted).Grow(1).MinWidth(150).SingleLine()
				ui.Text(c, "DOMAIN").FontSize(10).FontWeight(600).TextColor(tk2.TextMuted).Width(110).SingleLine()
				ui.Text(c, "PATH").FontSize(10).FontWeight(600).TextColor(tk2.TextMuted).Width(60).SingleLine()
				ui.Text(c, "EXPIRES").FontSize(10).FontWeight(600).TextColor(tk2.TextMuted).Width(130).SingleLine()
				ui.Text(c, "HTTPONLY").FontSize(10).FontWeight(600).TextColor(tk2.TextMuted).Width(65).SingleLine()
				ui.Box(c).Width(60) // 操作列
			})

			for _, ck := range cookies {
				ck := ck
				isCopied := *copiedKey == ck.Name
				row := ui.Row(c.Key("ck-row-" + ck.Name)).
					Height(30).PaddingX(8).Gap(8).AlignItems(ui.Center).
					Radius(m.RadiusSM).
					BorderWidth(0, 0, 1, 0).BorderColor(tk2.Border.Alpha(0.4))
				if row.Hovered() {
					row.Background(tk2.BgHover)
				}
				row.Children(func() {
					// 1. NAME 单行展示，超出显示省略号，悬浮显示完整文本
					ui.Text(c, ck.Name).FontSize(m.FontXS).FontWeight(500).
						TextColor(tk2.TextPrimary).Width(130).SingleLine().Tooltip(ck.Name)

					// 2. VALUE 单行展示，超出显示省略号，点击直接复制该值，悬浮显示完整文本
					valBtn := ui.ButtonBase(c.Key("ck-val-" + ck.Name)).
						Grow(1).MinWidth(150).Height(24).PaddingX(4).
						Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.Start).
						Cursor(ui.CursorPointer).Role(ui.RoleButton).
						Label("Value: " + ck.Value).Tooltip(ck.Value)
					if valBtn.Hovered() {
						valBtn.Background(tk2.BgSelected)
					}
					if valBtn.Clicked() {
						c.Services().WriteClipboard(ck.Value)
						*copiedKey = ck.Name
					}
					valBtn.Children(func() {
						ui.Text(c, ck.Value).FontSize(m.FontXS).
							TextColor(tk.Info).SingleLine().Grow(1)
					})

					// 3. DOMAIN
					ui.Text(c, ck.Domain).FontSize(m.FontXS).
						TextColor(tk2.TextMuted).Width(110).SingleLine().Tooltip(ck.Domain)

					// 4. PATH
					ui.Text(c, ck.Path).FontSize(m.FontXS).
						TextColor(tk2.TextMuted).Width(60).SingleLine().Tooltip(ck.Path)

					// 5. EXPIRES
					ui.Text(c, ck.Expires).FontSize(m.FontXS).
						TextColor(tk2.TextMuted).Width(130).SingleLine().Tooltip(ck.Expires)

					// 6. HTTPONLY
					httpOnlyStr := ""
					if ck.HttpOnly {
						httpOnlyStr = "✓"
					}
					ui.Text(c, httpOnlyStr).FontSize(m.FontXS).
						TextColor(tk.Success).Width(65).SingleLine()

					// 7. 复制按钮
					copyBtn := ui.ButtonBase(c.Key("ck-copy-" + ck.Name)).
						Height(22).PaddingX(6).Gap(4).Radius(m.RadiusSM).
						AlignItems(ui.Center).Justify(ui.Center).
						Cursor(ui.CursorPointer).Role(ui.RoleButton).
						Label("Copy " + ck.Name).Tooltip("Copy value")
					if isCopied {
						copyBtn.Background(tk2.AccentSoft)
					} else if copyBtn.Hovered() {
						copyBtn.Background(tk2.BgSelected)
					}
					if copyBtn.Clicked() {
						c.Services().WriteClipboard(ck.Value)
						*copiedKey = ck.Name
					}
					copyBtn.Children(func() {
						if isCopied {
							ui.Icon(c, icons.Check).FontSize(11).TextColor(tk.Success)
							ui.Text(c, "Copied").FontSize(10).TextColor(tk.Success).SingleLine()
						} else {
							ui.Icon(c, icons.Copy).FontSize(11).TextColor(tk2.TextSecondary)
							ui.Text(c, "Copy").FontSize(10).TextColor(tk2.TextSecondary).SingleLine()
						}
					})
				})
			}
		})
	})
}

// tabsRowWithAction 是对齐 Pixso 的胶囊分段式 Tab 行（灰底轨道 + 选中白底卡片 + 右侧可选动作）
func tabsRowWithAction(c *ui.Context, labels []string, active int,
	onSelect func(int), trailing func(),
) {
	tk2 := thV2.Of(c)
	ui.Row(c).FillWidth().Height(28).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Height(28).Padding(2).Gap(2).Radius(6).
			Background(tk2.BgSubtle).AlignItems(ui.Center).Children(func() {
			for i, label := range labels {
				i := i
				isActive := i == active
				tab := ui.ButtonBase(c.Key(fmt.Sprintf("tab-btn-%s-%d", label, i))).
					Height(24).MinWidth(50).PaddingX(10).Radius(4).
					Justify(ui.Center).AlignItems(ui.Center).
					Cursor(ui.CursorPointer).Role(ui.RoleTab).Label(label).
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				switch {
				case isActive:
					tab.Background(tk2.BgSurface).
						Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.08))
				case tab.Pressed():
					tab.Background(tk2.BgSelected)
				case tab.Hovered():
					tab.Background(tk2.BgHover)
				default:
					tab.Background(ui.Transparent)
				}
				if tab.Clicked() && onSelect != nil {
					onSelect(i)
				}
				tab.Children(func() {
					color := tk2.TextSecondary
					if isActive {
						color = tk2.TextPrimary
					}
					// 统一使用 Medium (500) 字重，避免在选中与未选切换时因字形加粗导致文字宽度变化引起排版跳动（Layout Shift）
					ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(color).SingleLine()
				})
			}
		})
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
