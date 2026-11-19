// Package history 实现真实的请求历史记录管理与查看。
package history

import (
	"fmt"
	"strings"
	"time"

	"rddpostman/components/theme"
	v1 "rddpostman/components/v1"
	"rddpostman/internal/domain"
	"rddpostman/internal/logger"
	"rddpostman/internal/nav"
	"rddpostman/internal/store"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// View 绘制历史记录页。
func View(c *ui.Context, db *store.Store, n nav.Navigator) {
	ui.Scroll(c).Grow(1).Children(func() {
		content(c, db, n)
	})
}

func content(c *ui.Context, db *store.Store, n nav.Navigator) {
	m := theme.Metrics
	tk := theme.Of(c)
	v1.PageHeader(c, locales.HistoryTitle(), locales.HistorySubtitle())

	if db == nil {
		v1.EmptyState(c, "存储未就绪")
		return
	}

	snap := db.Snapshot()
	ws := snap.ActiveWorkspace()
	if ws == nil {
		v1.EmptyState(c, "未选择工作区")
		return
	}

	entries, err := db.History(ws.ID, 100)
	if err != nil {
		logger.Error("加载历史记录失败: %v", err)
	}

	// 顶部操作栏：记录总数与清空按钮
	ui.Row(c).Padding(0, m.PagePaddingX, m.Space3, m.PagePaddingX).
		AlignItems(ui.Center).Children(func() {
		countText := fmt.Sprintf("共 %d 条历史记录 (最近 100 条)", len(entries))
		ui.Text(c, countText).FontSize(m.FontSM).TextColor(tk.TextMuted)

		ui.Spacer(c)

		if len(entries) > 0 {
			clearBtn := ui.Button(c, "清空历史记录")
			if clearBtn.Clicked() {
				if err := db.ClearHistory(ws.ID); err != nil {
					logger.Error("清空历史记录失败: %v", err)
				} else {
					c.Invalidate()
				}
			}
		}
	})

	if len(entries) == 0 {
		v1.EmptyState(c, "暂无请求历史记录，发送请求后将自动记录在此")
		return
	}

	// 列表展示真实历史项
	ui.Column(c).Gap(1).Padding(0, m.PagePaddingX).Children(func() {
		for _, it := range entries {
			historyRow(c, it)
		}
	})
}

func historyRow(c *ui.Context, it domain.HistoryEntry) {
	m := theme.Metrics
	tk := theme.Of(c)

	row := ui.Row(c.Key("hist-"+string(it.ID))).
		Gap(m.Space3).Padding(m.Space2, m.Space2).
		AlignItems(ui.Center).Radius(m.RadiusSM)

	if row.Hovered() {
		row.Background(tk.HoverSurface)
	}

	method := strings.ToUpper(it.Method)
	if method == "" {
		method = "GET"
	}

	statusText := fmt.Sprintf("%d", it.Status)
	statusColor := tk.Success
	if it.Status == 0 {
		statusText = "Error"
		statusColor = tk.Danger
	} else if it.Status >= 400 {
		statusColor = tk.Danger
	} else if it.Status >= 300 {
		statusColor = tk.Info
	}

	durationText := ""
	if it.DurationMS > 0 {
		if it.DurationMS < 1000 {
			durationText = fmt.Sprintf("%d ms", it.DurationMS)
		} else {
			durationText = fmt.Sprintf("%.2f s", float64(it.DurationMS)/1000.0)
		}
	}

	timeStr := formatRelativeTime(it.At)

	row.Children(func() {
		v1.Tag(c, method).TextColor(methodColor(tk, method))
		ui.Text(c, it.URL).FontSize(m.FontSM).TextColor(tk.TextPrimary).Grow(1).SingleLine()
		ui.Text(c, statusText).FontSize(m.FontXS).TextColor(statusColor).Width(60)
		if durationText != "" {
			ui.Text(c, durationText).FontSize(m.FontXS).TextColor(tk.TextMuted).Width(70)
		}
		ui.Text(c, timeStr).FontSize(m.FontXS).TextColor(tk.TextMuted).Width(120).SingleLine()

		copyBtn := ui.ButtonBase(c.Key("copy-"+string(it.ID))).
			Padding(m.Space1, m.Space2).Radius(m.RadiusSM).Label("复制 URL")
		if copyBtn.Hovered() {
			copyBtn.Background(tk.BgElevated)
		}
		if copyBtn.Clicked() {
			c.WriteClipboard(it.URL)
		}
		copyBtn.Children(func() {
			ui.Text(c, "复制").FontSize(m.FontXS).TextColor(tk.TextSecondary)
		})
	})
}

func methodColor(tk theme.Tokens, method string) ui.Color {
	switch strings.ToUpper(method) {
	case "GET":
		return tk.Success
	case "POST":
		return tk.Accent
	case "PUT":
		return tk.Info
	case "DELETE":
		return tk.Danger
	case "PATCH":
		return ui.Hex("#7C5CE0")
	default:
		return tk.TextMuted
	}
}

func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	now := time.Now()
	diff := now.Sub(t)
	if diff < time.Minute {
		return "刚刚"
	}
	if diff < time.Hour {
		return fmt.Sprintf("%d 分钟前", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%d 小时前", int(diff.Hours()))
	}
	return t.Format("2006-01-02 15:04")
}
