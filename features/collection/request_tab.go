package collection

import (
	"fmt"
	"strconv"
	"strings"

	"rddpostman/components/theme"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

func requestTab(c *ui.Context, n *domain.Node, crumb string) {
	m := theme.Metrics
	tk := theme.Of(c)
	req := n.Request
	ui.Column(c).Grow(1).Background(tk.BgSurface).Children(func() {
		// 请求编辑区。
		ui.Column(c).Padding(m.Space3, m.Space4).Gap(m.Space2).Children(func() {
			ui.Row(c).Gap(m.Space1).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.Get("folder")).FontSize(10).TextColor(tk.TextMuted)
				ui.Text(c, crumb).FontSize(10).
					TextColor(tk.TextMuted).Grow(1)
				ui.Text(c, "已保存 · 林晓").FontSize(10).TextColor(tk.TextMuted)
			})

			ui.Row(c).Height(40).Gap(m.Space2).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Height(m.SizeXL).PaddingX(m.Space2).Gap(m.Space1).
					AlignItems(ui.Center).Radius(m.RadiusMD).
					Background(tk.BgSurface).Border(m.Border, tk.Border).
					Children(func() {
						ui.Text(c, req.Method).FontSize(m.FontXS).Bold().
							TextColor(methodColor(tk, req.Method))
						ui.Icon(c, icons.Get("chevron-down")).FontSize(10).
							TextColor(tk.TextMuted)
					})
				ui.Row(c).Grow(1).Height(m.SizeXL).PaddingX(m.Space2).
					AlignItems(ui.Center).Radius(m.RadiusMD).
					Background(tk.BgSurface).Border(m.Border, tk.Border).
					Children(func() {
						ui.Text(c, req.URL).FontSize(m.FontSM).
							TextColor(tk.TextSecondary).SingleLine()
					})
				ui.PrimaryButton(c, locales.Send())
				for _, ic := range []string{"ellipsis", "code"} {
					ui.Row(c).Width(m.SizeXL).Height(m.SizeXL).Justify(ui.Center).
						AlignItems(ui.Center).Radius(m.RadiusMD).
						Background(tk.BgSurface).Border(m.Border, tk.Border).
						Children(func() {
							ui.Icon(c, icons.Get(ic)).FontSize(m.IconXS).
								TextColor(tk.TextMuted)
						})
				}
			})

			tabsRow(c, []string{
				fmt.Sprintf("Params %d", len(req.Query)),
				"Authorization",
				fmt.Sprintf("Headers %d", len(req.Headers)),
				"Body", "Scripts", "Tests", "Settings",
			}, 0, func() {
				ui.Icon(c, icons.Get("code")).FontSize(m.IconXS).
					TextColor(tk.TextMuted)
				ui.Text(c, "代码片段").FontSize(10).TextColor(tk.TextMuted)
			})

			ui.Divider(c)

			ui.Row(c).Gap(m.Space2).Height(170).Children(func() {
				ui.Column(c).Grow(1).Clip().Radius(m.RadiusMD).
					Background(tk.BgSurface).Border(m.Border, tk.Border).
					Children(func() {
						ui.Row(c).Height(26).Padding(m.Space1, m.Space2).Gap(m.Space2).
							AlignItems(ui.Center).
							BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).
							Children(func() {
								ui.Text(c, "Key").FontSize(10).TextColor(tk.TextMuted).Width(160)
								ui.Text(c, "Value").FontSize(10).TextColor(tk.TextMuted).Grow(1)
								ui.Text(c, "Description").FontSize(10).TextColor(tk.TextMuted).Width(120)
							})
						for _, kv := range req.Query {
							ui.Row(c).Height(32).Padding(m.Space1, m.Space2).Gap(m.Space2).
								AlignItems(ui.Center).Children(func() {
								ui.Box(c).Size(12, 12).Radius(m.RadiusSM).
									Border(m.Border, tk.Border)
								ui.Text(c, kv.Key).FontSize(m.FontSM).
									TextColor(tk.TextPrimary).Width(140)
								ui.Text(c, kv.Value).FontSize(m.FontSM).
									TextColor(tk.Info).Grow(1)
								ui.Text(c, kv.Desc).FontSize(10).
									TextColor(tk.TextMuted).Width(120)
							})
						}
					})

				ui.Column(c).Width(220).Padding(m.Space2).Gap(m.Space1).
					Radius(m.RadiusMD).Background(tk.BgSurface).
					Border(m.Border, tk.Border).Children(func() {
					ui.Row(c).Gap(m.Space1).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.Get("triangle-alert")).FontSize(10).
							TextColor(tk.Danger)
						ui.Text(c, "解析问题").FontSize(10).Bold().TextColor(tk.Danger)
						ui.Spacer(c)
						ui.Text(c, "Sandbox").FontSize(10).TextColor(tk.TextMuted)
					})
					for _, it := range [][2]string{
						{"baseUrl", "api.sandbox.devhub.io"},
						{"userId", "4821"},
						{"accessToken", "eyJhbGciOiJIUzI1..."},
					} {
						ui.Row(c).Gap(m.Space1).Children(func() {
							ui.Text(c, it[0]).FontSize(10).TextColor(tk.Accent).Width(78)
							ui.Text(c, it[1]).FontSize(10).TextColor(tk.TextSecondary).
								Grow(1).SingleLine()
						})
					}
				})
			})
		})

		// 响应区。
		ui.Column(c).Grow(1).BorderWidth(m.Border, 0, 0, 0).
			BorderColor(tk.Border).Children(func() {
			ui.Row(c).Height(42).PaddingX(m.Space4).Gap(m.Space2).
				AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.Response()).FontSize(m.FontSM).Bold().
					TextColor(tk.TextPrimary)
				ui.Text(c, "200 OK").FontSize(10).TextColor(tk.Success).
					Padding(1, m.Space1).Radius(m.RadiusSM).
					Background(tk.Success.Alpha(0.12))
				ui.Text(c, "182 ms · 2.41 KB · TTFE 96 ms").FontSize(10).
					TextColor(tk.TextMuted)
				ui.Spacer(c)
				ui.Text(c, locales.TestsPassed()).FontSize(10).TextColor(tk.Success)
				ui.Icon(c, icons.Get("ellipsis")).FontSize(m.IconXS).
					TextColor(tk.TextMuted)
			})

			tabsRow(c, []string{
				"Body", "Cookies 2", "Headers 11", "Test Results 3", "Visualize",
			}, 0, func() {
				ui.Text(c, "Preview  ›  JSON").FontSize(10).TextColor(tk.TextMuted)
			})

			ui.Row(c).Grow(1).Padding(m.Space3, m.Space4).Gap(m.Space2).
				AlignItems(ui.Stretch).Children(func() {
				ui.Row(c).Grow(1).AlignItems(ui.Start).Clip().Radius(m.RadiusMD).
					Background(tk.BgSurface).Border(m.Border, tk.Border).
					Children(func() {
						lines := []string{
							"{",
							`  "id": "usr_4821",`,
							`  "username": "lin.wei",`,
							`  "displayName": "林薇",`,
							`  "email": "lin.wei@devhub.io",`,
							`  "role": "backend_engineer",`,
							`  "tenantId": "acme-apac-02",`,
							`  "verified": true,`,
							`  "createdAt": "2026-03-18T09:24:11Z",`,
							`  "scopes": ["users:read", "orders:write"],`,
							`  "quota": {`,
							`    "dailyLimit": 20000,`,
							`    "used": 8342`,
							`  },`,
							`  "resultsAs": "json",`,
							`  "lastActiveIp": "203.0.113.42"`,
							"}",
						}
						ui.Column(c).Width(28).Padding(m.Space2, m.Space1).
							Gap(1).Children(func() {
							for i := range lines {
								ui.Text(c, strconv.Itoa(i+1)).FontSize(10).
									TextColor(tk.TextMuted).TextAlign(ui.End)
							}
						})
						ui.Column(c).Grow(1).Padding(m.Space2, m.Space2).
							Gap(1).Children(func() {
							for _, line := range lines {
								jsonLine(c, tk, line)
							}
						})
					})

				ui.Column(c).Width(200).Padding(m.Space2).Gap(m.Space1).
					Radius(m.RadiusMD).Background(tk.BgSurface).
					Border(m.Border, tk.Border).Children(func() {
					ui.Text(c, locales.ResponseSummary()).FontSize(10).Bold().
						TextColor(tk.TextPrimary)
					for _, it := range [][2]string{
						{"Status", "200 OK"},
						{"Time", "182 ms"},
						{"Size", "2.41 KB"},
						{"Content-Type", "application/json"},
						{"Server", "devhub-edge/2.14"},
						{"X-Request-Id", "req_9f2c81b4"},
					} {
						ui.Row(c).Gap(m.Space1).Children(func() {
							ui.Text(c, it[0]).FontSize(10).TextColor(tk.TextMuted).Width(70)
							ui.Text(c, it[1]).FontSize(10).TextColor(tk.TextSecondary).
								Grow(1).SingleLine()
						})
					}
					ui.Divider(c)
					ui.Text(c, locales.SavedExamples()).FontSize(10).Bold().
						TextColor(tk.TextPrimary)
					for _, it := range [][3]string{
						{"GET", "完整资料", "12 分钟前"},
						{"GET", "字段精简", "1 小时前"},
						{"GET", "用户不存在", "昨天"},
					} {
						ui.Row(c).Gap(m.Space1).AlignItems(ui.Center).Children(func() {
							ui.Text(c, it[0]).FontSize(10).Width(32).
								TextColor(methodColor(tk, it[0]))
							ui.Text(c, it[1]).FontSize(10).TextColor(tk.TextSecondary).Grow(1)
							ui.Text(c, it[2]).FontSize(10).TextColor(tk.TextMuted)
							ui.Text(c, "200").FontSize(10).TextColor(tk.TextMuted)
						})
					}
				})
			})
		})
	})
}

// tabsRow 画设计稿里的下划线 tab：等高、文字居中、底部 2px 指示条。
func tabsRow(c *ui.Context, labels []string, active int, trailing func()) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Row(c).Height(34).PaddingX(m.Space4).Gap(m.Space6).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).
		Children(func() {
			for i, label := range labels {
				ui.Column(c).Height(34).Children(func() {
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
			}
			if trailing != nil {
				ui.Spacer(c)
				trailing()
			}
		})
}

// jsonLine 把一行 JSON 拆成键和值分别着色。
func jsonLine(c *ui.Context, tk theme.Tokens, line string) {
	t := strings.TrimSpace(line)
	if t == "{" || t == "}" || t == "}," || strings.HasSuffix(t, "{") {
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
			strings.HasPrefix(val, "true") || strings.HasPrefix(val, "false")) {
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
