package collection

import (
	"crypto/rand"
	"fmt"
	"sort"
	"strings"
	"time"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/domain"

	"github.com/egoist/mygo/ui"
)

// URLVarInfo 表示从 URL 中解析出的单个变量信息。
type URLVarInfo struct {
	Name     string
	Value    string
	Source   string
	Resolved bool
	Secret   bool
}

// URLSegmentKind 表示 URL 分段类型：纯文本字面量或变量 Token。
type URLSegmentKind int

const (
	SegmentLiteral URLSegmentKind = iota
	SegmentVariable
)

// URLSegment 表示 URL 拆分后的单个呈现片段。
type URLSegment struct {
	Kind    URLSegmentKind
	Text    string
	Raw     string
	VarInfo URLVarInfo
}

// parseURLSegments 将 rawURL 拆分为字面量文本段与 {{var}} 变量段，用于在地址栏以交互胶囊展示。
func parseURLSegments(rawURL string, vars map[string]domain.Variable) []URLSegment {
	var segments []URLSegment
	s := rawURL

	for {
		start := strings.Index(s, "{{")
		if start < 0 {
			if len(s) > 0 {
				segments = append(segments, URLSegment{
					Kind: SegmentLiteral,
					Text: s,
					Raw:  s,
				})
			}
			break
		}

		if start > 0 {
			literal := s[:start]
			segments = append(segments, URLSegment{
				Kind: SegmentLiteral,
				Text: literal,
				Raw:  literal,
			})
		}

		end := strings.Index(s[start+2:], "}}")
		if end < 0 {
			// 未闭合的 {{ 视为剩余字面量
			segments = append(segments, URLSegment{
				Kind: SegmentLiteral,
				Text: s[start:],
				Raw:  s[start:],
			})
			break
		}

		varName := strings.TrimSpace(s[start+2 : start+2+end])
		rawToken := s[start : start+2+end+2]

		info := URLVarInfo{Name: varName}
		if v, ok := vars[varName]; ok {
			info.Resolved = true
			info.Value = v.Value
			info.Source = string(v.Source)
			info.Secret = v.Secret
		} else if dynVal, ok := getDynamicVariableValue(varName); ok {
			info.Resolved = true
			info.Value = dynVal
			info.Source = "dynamic"
		} else {
			info.Resolved = false
			info.Source = "unresolved"
		}

		segments = append(segments, URLSegment{
			Kind:    SegmentVariable,
			Text:    varName,
			Raw:     rawToken,
			VarInfo: info,
		})

		s = s[start+2+end+2:]
	}

	return segments
}

// extractURLVariables 解析 rawURL 中的全部 {{name}} 占位符，并结合可用变量集判定解析状态。
func extractURLVariables(rawURL string, vars map[string]domain.Variable) []URLVarInfo {
	var results []URLVarInfo
	seen := make(map[string]bool)

	s := rawURL
	for {
		start := strings.Index(s, "{{")
		if start < 0 {
			break
		}
		end := strings.Index(s[start+2:], "}}")
		if end < 0 {
			break
		}
		name := strings.TrimSpace(s[start+2 : start+2+end])
		s = s[start+2+end+2:]

		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		info := URLVarInfo{Name: name}
		if v, ok := vars[name]; ok {
			info.Resolved = true
			info.Value = v.Value
			info.Source = string(v.Source)
			info.Secret = v.Secret
		} else if dynVal, ok := getDynamicVariableValue(name); ok {
			info.Resolved = true
			info.Value = dynVal
			info.Source = "dynamic"
		} else {
			info.Resolved = false
			info.Source = "unresolved"
		}
		results = append(results, info)
	}
	return results
}

// evaluateURL 计算 URL 中的全部变量替换结果，并返回缺失/未解析变量名列表。
func evaluateURL(rawURL string, vars map[string]domain.Variable) (string, []string) {
	if !strings.Contains(rawURL, "{{") {
		return rawURL, nil
	}

	missingMap := make(map[string]bool)
	var b strings.Builder
	s := rawURL

	for {
		start := strings.Index(s, "{{")
		if start < 0 {
			b.WriteString(s)
			break
		}
		end := strings.Index(s[start+2:], "}}")
		if end < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:start])
		name := strings.TrimSpace(s[start+2 : start+2+end])
		if v, ok := vars[name]; ok {
			b.WriteString(v.Value)
		} else if dynVal, ok := getDynamicVariableValue(name); ok {
			b.WriteString(dynVal)
		} else {
			missingMap[name] = true
			b.WriteString("{{" + name + "}}")
		}
		s = s[start+2+end+2:]
	}

	missing := make([]string, 0, len(missingMap))
	for name := range missingMap {
		missing = append(missing, name)
	}
	sort.Strings(missing)

	return b.String(), missing
}

// getDynamicVariableValue 获取内置动态变量（$guid、$timestamp、$randomInt 等）的即时值。
func getDynamicVariableValue(name string) (string, bool) {
	switch name {
	case "$guid", "$uuid", "$randomUUID":
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), true
	case "$timestamp":
		return fmt.Sprintf("%d", time.Now().Unix()), true
	case "$isoTimestamp":
		return time.Now().UTC().Format(time.RFC3339), true
	case "$randomInt":
		b := make([]byte, 2)
		if _, err := rand.Read(b); err != nil {
			return "0", true
		}
		return fmt.Sprintf("%d", int(b[0])<<8|int(b[1])), true
	case "$randomEmail":
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		return fmt.Sprintf("user_%x@example.com", b), true
	case "$randomFirstName":
		return "Alex", true
	}
	return "", false
}

// allAvailableVariables 汇总全部可用变量（环境变量、集合变量、全局/工作区变量与内置动态变量），供选择器与补全使用。
func allAvailableVariables(vars map[string]domain.Variable) []URLVarInfo {
	var list []URLVarInfo
	seen := make(map[string]bool)

	// 1. 用户已配置变量
	for k, v := range vars {
		seen[k] = true
		list = append(list, URLVarInfo{
			Name:     k,
			Value:    v.Value,
			Source:   string(v.Source),
			Resolved: true,
			Secret:   v.Secret,
		})
	}

	// 2. 常用内置动态变量
	builtin := []struct {
		name string
		desc string
	}{
		{"$guid", "Random UUID v4"},
		{"$timestamp", "Current Unix timestamp"},
		{"$isoTimestamp", "Current ISO-8601 UTC timestamp"},
		{"$randomInt", "Random integer (0-65535)"},
		{"$randomEmail", "Random example email"},
	}
	for _, b := range builtin {
		if !seen[b.name] {
			list = append(list, URLVarInfo{
				Name:     b.name,
				Value:    b.desc,
				Source:   "dynamic",
				Resolved: true,
			})
		}
	}

	// 排序：环境变量/集合变量排前，动态变量随后，同级按名字字母序
	sort.Slice(list, func(i, j int) bool {
		pi := sourcePriority(list[i].Source)
		pj := sourcePriority(list[j].Source)
		if pi != pj {
			return pi < pj
		}
		return list[i].Name < list[j].Name
	})

	return list
}

func sourcePriority(source string) int {
	switch source {
	case "environment":
		return 1
	case "collection":
		return 2
	case "folder":
		return 3
	case "workspace":
		return 4
	case "global":
		return 5
	case "dynamic":
		return 6
	default:
		return 7
	}
}

// detectUnclosedVariable 检测 URL 输入串中末尾或光标前是否有正在输入的未闭合 {{ 变量前缀。
func detectUnclosedVariable(u string) (prefix string, hasUnclosed bool) {
	lastOpen := strings.LastIndex(u, "{{")
	if lastOpen < 0 {
		return "", false
	}
	after := u[lastOpen+2:]
	if strings.Contains(after, "}}") {
		return "", false
	}
	// 空格、换行或路径分隔符不视为合法变量名输入
	if strings.ContainsAny(after, " \t\r\n/?#") {
		return "", false
	}
	return after, true
}

// filterVariables 根据键入前缀实时过滤可用变量列表。
func filterVariables(all []URLVarInfo, prefix string) []URLVarInfo {
	q := strings.ToLower(strings.TrimSpace(prefix))
	if q == "" {
		return all
	}
	var res []URLVarInfo
	for _, v := range all {
		if strings.Contains(strings.ToLower(v.Name), q) || strings.Contains(strings.ToLower(v.Value), q) {
			res = append(res, v)
		}
	}
	return res
}

// completeVariableInURL 将选中的变量名补全到当前未闭合的 {{ 之后，或智能追加到末尾。
func completeVariableInURL(state *reqSessionState, varName string) {
	u := state.urlDraft
	token := "{{" + varName + "}}"
	lastOpen := strings.LastIndex(u, "{{")
	lastClose := strings.LastIndex(u, "}}")

	if lastOpen >= 0 && lastOpen > lastClose {
		// 有正在输入的未闭合 {{
		state.urlDraft = u[:lastOpen] + token
	} else {
		// 没有未闭合的 {{，做智能追加
		if u == "" {
			state.urlDraft = token
		} else if strings.HasSuffix(u, "/") || strings.HasSuffix(u, "?") || strings.HasSuffix(u, "&") || strings.HasSuffix(u, "=") {
			state.urlDraft = u + token
		} else {
			state.urlDraft = u + "/" + token
		}
	}

	state.autocompleteOpen = false
	state.autocompletePrefix = ""
	state.queryDraft = parseQueryFromURL(state.urlDraft)
	state.pathVarsDraft = extractPathVariables(state.urlDraft, state.pathVarsDraft)
}

// insertVariableToURL 将选中的变量插入到 URL 编辑草稿中（兼容旧调用）。
func insertVariableToURL(state *reqSessionState, varName string) {
	completeVariableInURL(state, varName)
}

// renderVariableTooltip 渲染鼠标悬停在行内变量胶囊上的 Popover 浮层（严格对齐 Pixso 画板 48 与 49）。
func renderVariableTooltip(c *ui.Context, info URLVarInfo, activeEnvName string) {
	tk2 := thV2.Of(c)
	m := thV2.Metrics

	ui.Column(c).Padding(10).Gap(8).FillWidth().Children(func() {
		if info.Resolved {
			// 画板 48：已解析变量状态
			// 1. 顶部变量名 + 作用域徽标
			ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Text(c, info.Name).FontSize(13).FontWeight(600).
					TextColor(tk2.TextPrimary).SingleLine()

				scopeBg := tk2.AccentSoft
				scopeColor := tk2.Accent
				switch info.Source {
				case "collection":
					scopeBg = ui.RGBA(254, 243, 199, 255)
					scopeColor = ui.RGBA(180, 83, 9, 255)
				case "dynamic":
					scopeBg = ui.RGBA(220, 252, 231, 255)
					scopeColor = ui.RGBA(21, 128, 61, 255)
				}

				ui.Box(c).Padding(2, 6).Radius(m.RadiusSM).Background(scopeBg).Children(func() {
					ui.Text(c, strings.ToUpper(info.Source)).FontSize(10).FontWeight(600).
						TextColor(scopeColor).SingleLine()
				})
			})

			// 细分割线
			ui.Box(c).FillWidth().Height(1).Background(tk2.Border)

			// 2. 当前值预览
			ui.Column(c).FillWidth().Gap(3).Children(func() {
				ui.Text(c, "CURRENT VALUE:").FontSize(10).FontWeight(600).
					TextColor(tk2.TextMuted).SingleLine()

				dispVal := info.Value
				if info.Secret {
					dispVal = "••••••••"
				}
				ui.Box(c).FillWidth().Padding(5, 8).Radius(m.RadiusSM).
					Background(tk2.BgSubtle).Border(1, tk2.Border).Children(func() {
					ui.Text(c, dispVal).FontSize(11).FontWeight(500).
						TextColor(tk2.TextPrimary).SingleLine()
				})
			})

			// 细分割线
			ui.Box(c).FillWidth().Height(1).Background(tk2.Border)

			// 3. 所属生效环境
			ui.Row(c).AlignItems(ui.Center).Gap(4).Children(func() {
				ui.Icon(c, icons.Globe).FontSize(11).TextColor(tk2.TextMuted)
				ui.Text(c, activeEnvName).FontSize(11).TextColor(tk2.TextSecondary).SingleLine()
			})
		} else {
			// 画板 49：未定义变量警告状态
			ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Text(c, info.Name).FontSize(13).FontWeight(600).
					TextColor(tk2.Danger).SingleLine()

				ui.Box(c).Padding(2, 6).Radius(m.RadiusSM).Background(tk2.DangerSoft).Children(func() {
					ui.Text(c, "UNRESOLVED").FontSize(10).FontWeight(600).
						TextColor(tk2.Danger).SingleLine()
				})
			})

			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Icon(c, icons.TriangleAlert).FontSize(12).TextColor(tk2.Warning)
				ui.Text(c, "Variable not defined in active environment.").
					FontSize(11).TextColor(tk2.TextSecondary)
			})

			ui.Box(c).Padding(4, 8).Radius(m.RadiusSM).Background(tk2.BgSubtle).
				Border(1, tk2.Border).Children(func() {
				ui.Text(c, "+ Set in "+activeEnvName).FontSize(11).FontWeight(500).
					TextColor(tk2.Accent).SingleLine()
			})
		}
	})
}

type varPopoverState struct {
	open                bool
	pinned              bool
	dismissedUntilLeave bool
}

// renderTokenizedURLBar 渲染常规展示态下的行内变量胶囊（对齐 Pixso 画板 47 与 48）。
func renderTokenizedURLBar(c *ui.Context, props pageProps, activeID string,
	state *reqSessionState, allVars map[string]domain.Variable, activeEnvName string,
) {
	tk2 := thV2.Of(c)
	m := thV2.Metrics
	segments := parseURLSegments(state.urlDraft, allVars)

	// 容器支持点击直接切换到编辑模式
	ui.Row(c.Key("url-token-row-" + activeID)).Grow(1).FillHeight().
		AlignItems(ui.Center).Gap(2).Children(func() {
		for i, seg := range segments {
			idx := i
			segment := seg

			if segment.Kind == SegmentLiteral {
				// 字面量文本段：点击触发编辑态
				txtBtn := ui.ButtonBase(c.Key(fmt.Sprintf("url-lit-%s-%d", activeID, idx))).
					Height(24).PaddingX(2).AlignItems(ui.Center).Cursor(ui.CursorText).
					Role(ui.RoleButton).Label(segment.Text)
				if txtBtn.Clicked() {
					state.urlEditing = true
					state.urlFocusReq = true
				}
				txtBtn.Children(func() {
					ui.Text(c, segment.Text).FontSize(12).TextColor(tk2.TextPrimary).SingleLine()
				})
			} else {
				// 变量段：精致软底高亮胶囊（已解析为蓝，未解析为警示红）
				pillKey := fmt.Sprintf("url-var-pill-%s-%d-%s", activeID, idx, segment.Text)
				pill := ui.ButtonBase(c.Key(pillKey)).
					Height(20).PaddingX(6).Radius(m.RadiusSM).AlignItems(ui.Center).
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(segment.Raw)

				st := ui.Local(pill, "var-popover-state", func() varPopoverState {
					return varPopoverState{}
				})
				pillHovered := pill.Hovered()
				if !pillHovered {
					st.dismissedUntilLeave = false
				}
				if pillHovered && !st.dismissedUntilLeave {
					st.open = true
				}
				if pill.Clicked() {
					if st.pinned {
						st.pinned = false
						st.open = false
						st.dismissedUntilLeave = true
					} else {
						st.pinned = true
						st.open = true
						st.dismissedUntilLeave = false
					}
				}

				if segment.VarInfo.Resolved {
					if st.open || pillHovered {
						pill.Background(ui.RGBA(220, 231, 255, 255)).Border(1, tk2.Accent.Alpha(0.6))
					} else {
						pill.Background(tk2.AccentSoft).Border(1, tk2.Accent.Alpha(0.25))
					}
				} else {
					if st.open || pillHovered {
						pill.Background(ui.RGBA(255, 220, 215, 255)).Border(1, tk2.Danger.Alpha(0.6))
					} else {
						pill.Background(tk2.DangerSoft).Border(1, tk2.Danger.Alpha(0.25))
					}
				}

				pill.Children(func() {
					txtColor := tk2.Accent
					if !segment.VarInfo.Resolved {
						txtColor = tk2.Danger
					}
					ui.Text(c, segment.Raw).FontSize(11).FontWeight(500).
						TextColor(txtColor).SingleLine()
				})

				// 悬停/点击 Popover（画板 48 & 49）：外层透明 4px 顶部过渡区避免鼠标移入浮层时闪退
				panelHovered := false
				ui.PopoverBase(c, pill, &st.open, func(panel ui.Element) {
					panel.Padding(m.Space1, 0, 0, 0)
					panelHovered = panel.Hovered()
					ui.Box(c).Width(280).
						Background(tk2.BgSurface).
						Border(m.Border, tk2.BorderPopover).
						Radius(m.RadiusLG).
						Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.08)).
						Shadow(0, 12, 28, 0, ui.RGBA(0, 0, 0, 0.14)).
						Children(func() {
							renderVariableTooltip(c, segment.VarInfo, activeEnvName)
						})
				})

				// 外部点击或 Esc 关闭时同步清除固定态；非固定态下鼠标移出胶囊与浮层后立即收起
				if !st.open {
					st.pinned = false
				} else if !st.pinned && !pillHovered && !panelHovered {
					st.open = false
					c.Invalidate()
				}
			}
		}

		// 末尾点击空白区进入编辑态
		blankArea := ui.ButtonBase(c.Key("url-token-blank-" + activeID)).
			Grow(1).FillHeight().Cursor(ui.CursorText).Role(ui.RoleButton).Label("Edit URL")
		if blankArea.Clicked() {
			state.urlEditing = true
			state.urlFocusReq = true
		}
	})
}

// renderVariableAutocompleteDropdown 渲染输入 {{ 时自动弹出的补全下拉菜单（严格对齐 Pixso 画板 50）。
func renderVariableAutocompleteDropdown(c *ui.Context, anchor ui.Element, state *reqSessionState,
	allVars map[string]domain.Variable, activeEnvName string, onSelect func(varName string),
) {
	tk2 := thV2.Of(c)
	m := thV2.Metrics

	allList := allAvailableVariables(allVars)
	filtered := filterVariables(allList, state.autocompletePrefix)

	if len(filtered) == 0 {
		return
	}

	if state.autocompleteIndex >= len(filtered) {
		state.autocompleteIndex = 0
	}
	if state.autocompleteIndex < 0 {
		state.autocompleteIndex = len(filtered) - 1
	}

	open := &state.autocompleteOpen
	v2.Popover(c, anchor, open, v2.PopoverOpts{Width: 360}, func(panel ui.Element) {
		ui.Column(c).Padding(6).Gap(2).FillWidth().Children(func() {
			// 1. 顶部标题栏
			ui.Row(c).FillWidth().Height(20).PaddingX(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Text(c, fmt.Sprintf("AVAILABLE VARIABLES (%d)", len(filtered))).
					FontSize(10).FontWeight(600).TextColor(tk2.TextMuted).SingleLine()
				if state.autocompletePrefix != "" {
					ui.Text(c, "Filter: "+state.autocompletePrefix).FontSize(10).TextColor(tk2.Accent).SingleLine()
				}
			})

			// 2. 变量列表（最多展示前 8 项）
			maxItems := 8
			if len(filtered) < maxItems {
				maxItems = len(filtered)
			}
			for i := 0; i < maxItems; i++ {
				idx := i
				item := filtered[i]
				isActive := idx == state.autocompleteIndex

				rowKey := fmt.Sprintf("auto-item-%d-%s", idx, item.Name)
				rowBtn := ui.ButtonBase(c.Key(rowKey)).FillWidth().Height(32).
					Padding(0, 8).Radius(m.RadiusSM).Cursor(ui.CursorPointer).
					Role(ui.RoleButton).Label(item.Name)

				if isActive {
					rowBtn.Background(ui.RGBA(238, 242, 255, 255)) // #EEF2FF
				} else if rowBtn.Hovered() {
					rowBtn.Background(tk2.BgHover)
				}

				if rowBtn.Clicked() {
					onSelect(item.Name)
				}

				rowBtn.Children(func() {
					ui.Row(c).Fill().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
						// 左侧：变量名与作用域彩色胶囊
						ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
							nameColor := tk2.TextPrimary
							if isActive {
								nameColor = ui.RGBA(37, 99, 235, 255) // #2563EB
							}
							ui.Text(c, item.Name).FontSize(12).FontWeight(500).
								TextColor(nameColor).SingleLine()

							scopeBg := ui.RGBA(219, 234, 254, 255)    // #DBEAFE
							scopeColor := ui.RGBA(29, 78, 216, 255)   // #1D4ED8
							switch item.Source {
							case "collection":
								scopeBg = ui.RGBA(254, 243, 199, 255)  // #FEF3C7
								scopeColor = ui.RGBA(180, 83, 9, 255) // #B45309
							case "dynamic":
								scopeBg = ui.RGBA(220, 252, 231, 255)  // #DCFCE7
								scopeColor = ui.RGBA(21, 128, 61, 255) // #15803D
							}

							ui.Box(c).Padding(1, 5).Radius(3).Background(scopeBg).Children(func() {
								ui.Text(c, item.Source).FontSize(10).FontWeight(500).
									TextColor(scopeColor).SingleLine()
							})
						})

						// 右侧：值预览
						dispVal := item.Value
						if item.Secret {
							dispVal = "••••••••"
						}
						ui.Text(c, dispVal).FontSize(11).TextColor(tk2.TextMuted).SingleLine()
					})
				})
			}

			// 细分割线
			ui.Box(c).FillWidth().Height(1).Background(tk2.Border)

			// 3. 底部导航提示条
			ui.Row(c).FillWidth().Height(20).PaddingX(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Text(c, "↑ ↓ navigate  •  ↵ select").FontSize(10).TextColor(tk2.TextMuted).SingleLine()
				ui.Text(c, "esc to dismiss").FontSize(10).TextColor(tk2.TextMuted).SingleLine()
			})
		})
	})
}
