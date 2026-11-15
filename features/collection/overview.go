package collection

import (
	"fmt"
	"strings"
	"time"

	"rddpostman/components/theme"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/internal/routes"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// 概览页分段。只有概览有内容，其余分段按设计稿保留入口。
const (
	segOverview = "overview"
	segAuth     = "auth"
	segScripts  = "scripts"
	segVars     = "vars"
	segRun      = "run"

	// docWidth 是设计稿里 720 宽的正文列。
	docWidth = 720
)

// titleState 是标题的编辑态。切换 tab 时按 id 重置，避免草稿串到别的页面。
type titleState struct {
	id      string
	draft   string
	editing bool
	fresh   bool
}

// overviewTab 画集合概览（node 为 nil）或文件夹概览。
func overviewTab(c *ui.Context, props pageProps, coll *domain.Collection, node *domain.Node) {
	tk := theme.Of(c)
	id := string(coll.ID)
	if node != nil {
		id = string(node.ID)
	}
	root := ui.Column(c).Grow(1).Background(tk.BgSurface)
	title := ui.Local(root, "title-edit", func() titleState { return titleState{id: id} })
	if title.id != id {
		*title = titleState{id: id}
	}
	seg := props.state.segment(id)
	root.Children(func() {
		docHeader(c, coll, node)
		segmentBar(c, props, node, id)
		ui.Box(c).Height(theme.Metrics.Border).Background(tk.Border)
		if seg != segOverview {
			ui.Box(c).Grow(1)
			return
		}
		docBody(c, props, coll, node, id, title)
	})
}

// docHeader 是设计稿的文档头：左侧集合名或面包屑。
func docHeader(c *ui.Context, coll *domain.Collection, node *domain.Node) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Row(c).Height(52).Padding(0, m.Space4, 0, m.Space5).
		AlignItems(ui.Center).Children(func() {
		if node == nil {
			ui.Text(c, coll.Name).FontSize(m.FontMD).FontWeight(500).
				TextColor(tk.TextPrimary).SingleLine()
		} else {
			ui.Row(c).Height(16).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.Get("folder")).Size(15, 13).
					TextColor(tk.TextSecondary)
				ui.Text(c, coll.Name).FontSize(m.FontXS).
					TextColor(tk.TextMuted).SingleLine()
				ui.Text(c, "/").FontSize(m.FontXS).TextColor(tk.TextMuted)
				ui.Text(c, node.Name).FontSize(m.FontMD).FontWeight(500).
					TextColor(tk.TextPrimary).SingleLine()
			})
		}
	})
}

// segmentBar 是设计稿的分段栏：概览、授权、脚本、变量，集合页多一个运行。
func segmentBar(c *ui.Context, props pageProps, node *domain.Node, id string) {
	m := theme.Metrics
	tk := theme.Of(c)
	type segTab struct {
		key   string
		label string
		dot   bool
	}
	segs := []segTab{
		{segOverview, locales.Overview(), false},
		{segAuth, locales.AuthorizationTab(), true},
		{segScripts, locales.ScriptsTab(), false},
		{segVars, locales.VariablesTab(), false},
	}
	if node == nil {
		segs = append(segs, segTab{segRun, locales.RunTab(), false})
	}
	active := props.state.segment(id)

	ui.Row(c).Height(40).Padding(0, m.Space4, 0, m.Space5).
		AlignItems(ui.Center).Children(func() {
		ui.Row(c).Gap(m.Space1).AlignItems(ui.Center).Children(func() {
			for _, s := range segs {
				s := s
				btn := ui.ButtonBase(c.Key("seg-" + id + "-" + s.key)).Height(24).
					PaddingX(10).Gap(5).AlignItems(ui.Center).Radius(m.RadiusMD).
					Label(s.label).Role(ui.RoleTab)
				switch {
				case s.key == active:
					btn.Background(tk.BgElevated).Border(m.Border, tk.Border)
				case btn.Hovered():
					btn.Background(tk.HoverSurface)
				}
				if btn.Clicked() {
					props.state.setSegment(id, s.key)
				}
				btn.Children(func() {
					color := tk.TextSecondary
					text := ui.Text(c, s.label).FontSize(m.FontXS).TextColor(color)
					if s.key == active {
						text.FontWeight(500).TextColor(tk.TextPrimary)
					}
					if s.dot {
						ui.Box(c).Size(5, 5).Radius(m.RadiusFull).
							Background(tk.Success)
					}
				})
			}
		})
		ui.Spacer(c)
		ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.Get("eye")).Size(15, 11).TextColor(tk.TextMuted)
			ui.Text(c, "0").FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
	})
}

// docBody 是 720 宽的正文列：标题、元信息、分隔线、描述、说明列表与文档链接。
func docBody(c *ui.Context, props pageProps, coll *domain.Collection,
	node *domain.Node, id string, title *titleState) {
	m := theme.Metrics
	tk := theme.Of(c)
	name := coll.Name
	description := coll.Description
	updated := coll.UpdatedAt
	requests := requestTotal(coll.Items)
	folders := folderTotal(coll.Items)
	if node != nil {
		name = node.Name
		description = node.Description
		updated = node.UpdatedAt
		requests = requestTotal(node.Children)
		folders = folderTotal(node.Children)
	}

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Column(c).Width(docWidth).Padding(34, 0, 24, 0).Children(func() {
				editableTitle(c, props, id, name, title)
				metaRow(c, tk, m, []string{
					locales.RequestCount(requests),
					locales.FolderCount(folders),
				}, locales.UpdatedAt(relativeTime(updated)))
				ui.Box(c).Height(m.Border).Background(tk.Border).
					Margin(18, 0, 0, 0)

				if strings.TrimSpace(description) == "" {
					ui.Text(c, locales.NoDescription()).Width(docWidth).
						FontSize(m.FontMD).FixedLineHeight(24).
						TextColor(tk.TextMuted).Margin(23, 0, 0, 0)
				} else {
					ui.Text(c, description).Width(docWidth).
						FontSize(m.FontMD).FixedLineHeight(24).
						TextColor(tk.TextPrimary).Margin(23, 0, 0, 0)
				}

				if node == nil {
					envSection(c, props, tk, m)
				} else {
					tocSection(c, node, tk, m)
				}

				link := ui.Text(c, locales.ViewFullDocs()).FontSize(m.FontSM).
					FontWeight(500).TextColor(tk.Info).Margin(31, 0, 0, 0)
				if link.Clicked() {
					props.actions.navigate(routes.Docs)
				}
			})
		})
	})
}

// editableTitle 画可编辑标题：单击进入编辑，Enter 或失焦保存，Esc 取消。
func editableTitle(c *ui.Context, props pageProps, id, name string, title *titleState) {
	m := theme.Metrics
	tk := theme.Of(c)
	if !title.editing {
		btn := ui.ButtonBase(c.Key("doc-title")).FillWidth().Height(32).
			Justify(ui.Start).AlignItems(ui.Center).Label(name)
		if btn.Clicked() {
			title.editing, title.fresh, title.draft = true, true, name
		}
		btn.Children(func() {
			ui.Text(c, name).FontSize(m.FontXXL).FontWeight(600).
				TextColor(tk.TextPrimary).SingleLine()
		})
		return
	}

	in := ui.TextInputBase(c, &title.draft).FillWidth().Height(32).
		Padding(0, 6).Radius(m.RadiusSM).Border(m.Border, tk.Accent).
		FontSize(m.FontXXL).FontWeight(600).TextColor(tk.TextPrimary)
	if title.fresh {
		title.fresh = false
		in.AutoFocus()
		return
	}

	done := func(keep bool) {
		title.editing = false
		if !keep {
			return
		}
		next := strings.TrimSpace(title.draft)
		if next == "" || next == name {
			return
		}
		props.actions.rename(id, next)
	}
	switch {
	case in.Submitted():
		done(true)
	case in.Shortcut(0, ui.KeyEscape):
		done(false)
	case !in.Focused():
		done(true)
	}
}

// metaRow 是 11 号元信息行：请求数、文件夹数与更新时间，用圆点分隔。
func metaRow(c *ui.Context, tk theme.Tokens, m theme.Metric, counts []string, updated string) {
	ui.Row(c).Height(14).Gap(10).AlignItems(ui.Center).
		Margin(6, 0, 0, 0).Children(func() {
		for i, count := range counts {
			if i > 0 {
				ui.Text(c, "·").FontSize(m.FontXS).TextColor(tk.TextMuted)
			}
			ui.Text(c, count).FontSize(m.FontXS).
				TextColor(tk.TextSecondary).SingleLine()
		}
		ui.Text(c, "·").FontSize(m.FontXS).TextColor(tk.TextMuted)
		ui.Text(c, updated).FontSize(m.FontXS).
			TextColor(tk.TextMuted).SingleLine()
	})
}

// envSection 是集合概览的「环境说明」：每个环境的名称与 baseUrl。
func envSection(c *ui.Context, props pageProps, tk theme.Tokens, m theme.Metric) {
	if len(props.environments) == 0 {
		return
	}
	ui.Text(c, locales.EnvironmentNotes()).FontSize(m.FontSM).
		FontWeight(500).TextColor(tk.TextPrimary).Margin(18, 0, 0, 0)
	ui.Column(c).Gap(9).Margin(7, 0, 0, 0).Children(func() {
		for _, env := range props.environments {
			ui.Row(c).Height(18).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(3.5, 3.5).Radius(m.RadiusFull).
					Background(tk.TextMuted)
				ui.Text(c, env.Name).Width(84).FontSize(m.FontSM).
					Font("monospace").TextColor(tk.TextSecondary).SingleLine()
				ui.Text(c, envBaseURL(env.Vars)).FontSize(m.FontSM).
					Font("monospace").TextColor(tk.TextSecondary).SingleLine()
			})
		}
	})
}

// tocSection 是文件夹概览的「目录」：直接子节点列表，请求带上方法。
func tocSection(c *ui.Context, node *domain.Node, tk theme.Tokens, m theme.Metric) {
	if len(node.Children) == 0 {
		return
	}
	ui.Text(c, locales.Contents()).FontSize(m.FontSM).FontWeight(500).
		TextColor(tk.TextPrimary).Margin(18, 0, 0, 0)
	ui.Column(c).Gap(9).Margin(7, 0, 0, 0).Children(func() {
		for _, child := range node.Children {
			label := child.Name
			if child.Kind == domain.KindRequest && child.Request != nil {
				label = fmt.Sprintf("%s（%s）", child.Name, child.Request.Method)
			}
			ui.Row(c).Height(18).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(3.5, 3.5).Radius(m.RadiusFull).
					Background(tk.TextMuted)
				ui.Text(c, label).FontSize(m.FontSM).
					TextColor(tk.TextSecondary).SingleLine()
			})
		}
	})
}

// requestTotal 递归统计请求数。
func requestTotal(items []*domain.Node) int {
	total := 0
	for _, it := range items {
		if it.Kind == domain.KindRequest {
			total++
			continue
		}
		total += requestTotal(it.Children)
	}
	return total
}

// folderTotal 统计直接子目录数，与设计稿「N 个文件夹」一致。
func folderTotal(items []*domain.Node) int {
	total := 0
	for _, it := range items {
		if it.Kind == domain.KindFolder {
			total++
		}
	}
	return total
}

// envBaseURL 取环境里的 baseUrl 变量值。
func envBaseURL(vars []domain.KV) string {
	for _, v := range vars {
		if strings.EqualFold(v.Key, "baseUrl") {
			return v.Value
		}
	}
	return ""
}

// relativeTime 把更新时间转成设计稿里的相对时间文案。
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return locales.RelativeJustNow()
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return locales.RelativeJustNow()
	case d < time.Hour:
		return locales.RelativeMinutes(int(d.Minutes()))
	case d < 24*time.Hour:
		return locales.RelativeHours(int(d.Hours()))
	case d < 48*time.Hour:
		return locales.RelativeYesterday()
	default:
		return locales.RelativeDays(int(d.Hours() / 24))
	}
}
