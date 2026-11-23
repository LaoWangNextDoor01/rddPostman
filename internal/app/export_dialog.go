package app

import (
	"fmt"
	"os"
	"strings"
	"sync"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/internal/importer"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type exportSaveResult struct {
	mu      sync.Mutex
	pending bool
	saving  bool
	path    string
	err     string
}

func (r *exportSaveResult) start() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saving {
		return false
	}
	r.saving = true
	return true
}

func (r *exportSaveResult) finish(path, errMsg string) {
	r.mu.Lock()
	r.saving = false
	r.pending = true
	r.path = path
	r.err = errMsg
	r.mu.Unlock()
}

func (r *exportSaveResult) drain() (path, errMsg string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.pending {
		return "", "", false
	}
	r.pending = false
	path, errMsg = r.path, r.err
	r.path, r.err = "", ""
	return path, errMsg, true
}

// saveExportFile 负责弹出系统保存文件框并写入导出 JSON；测试中可替换。
var saveExportFile = defaultSaveExportFile

func defaultSaveExportFile(defaultName string, data []byte) (string, error) {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Title:       "Export JSON",
		DefaultPath: defaultName,
		Filters: []mygo.FileFilter{
			{Name: "JSON Files", Extensions: []string{"json"}},
		},
	})
	if err != nil || strings.TrimSpace(path) == "" {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

type exportTarget struct {
	kind string // "collection" | "environment"
	id   domain.ID
	name string
}

// exportDialog 绘制 Postman Collection / Environment JSON 导出弹窗。
func (a *App) exportDialog(c *ui.Context, open *bool) {
	if open == nil || a.store == nil {
		return
	}
	m, tk := thV2.Metrics, thV2.Of(c)
	ws := a.store.Snapshot().ActiveWorkspace()

	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		selectedKey := ui.Local(panel, "export-selected", func() string { return "" })
		statusMsg := ui.Local(panel, "export-status", func() string { return "" })
		errMsg := ui.Local(panel, "export-err", func() string { return "" })
		saveRes := *ui.Local(panel, "export-save-res", func() *exportSaveResult { return &exportSaveResult{} })

		if savedPath, saveErr, ok := saveRes.drain(); ok {
			if saveErr != "" {
				*errMsg = saveErr
				*statusMsg = ""
			} else if savedPath != "" {
				*errMsg = ""
				*statusMsg = "Saved to " + savedPath
			}
		}

		var targets []exportTarget
		if ws != nil {
			for _, coll := range ws.Collections {
				targets = append(targets, exportTarget{kind: "collection", id: coll.ID, name: coll.Name})
			}
			for _, env := range ws.Environments {
				targets = append(targets, exportTarget{kind: "environment", id: env.ID, name: env.Name})
			}
		}
		if *selectedKey == "" && len(targets) > 0 {
			*selectedKey = targets[0].kind + ":" + string(targets[0].id)
		}

		var previewJSON string
		var previewBytes []byte
		defaultFile := "export.json"
		if ws != nil {
			for _, t := range targets {
				k := t.kind + ":" + string(t.id)
				if k != *selectedKey {
					continue
				}
				safeName := sanitizeExportFileName(t.name)
				if t.kind == "collection" {
					for _, coll := range ws.Collections {
						if coll.ID == t.id {
							if data, err := importer.ExportPostmanCollection(coll); err == nil {
								previewBytes = data
								previewJSON = string(data)
								defaultFile = safeName + ".postman_collection.json"
							}
							break
						}
					}
				} else {
					for _, env := range ws.Environments {
						if env.ID == t.id {
							e := env
							if data, err := importer.ExportPostmanEnvironment(&e); err == nil {
								previewBytes = data
								previewJSON = string(data)
								defaultFile = safeName + ".postman_environment.json"
							}
							break
						}
					}
				}
				break
			}
		}

		backdrop.Background(tk.Overlay)
		panel.Width(760).Height(520).Radius(m.RadiusLG).
			Background(tk.BgSurface).
			Border(m.Border, tk.BorderPopover).
			Shadow(0, 12, 32, 0, tk.Overlay).
			Clip()

		panel.Children(func() {
			ui.Column(c).Fill().Children(func() {
				// 1. 顶部标题栏
				ui.Row(c).FillWidth().Height(52).Padding(0, m.Space5).Gap(m.Space2).
					AlignItems(ui.Center).
					BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
					ui.Icon(c, icons.Download).FontSize(m.IconMD).TextColor(tk.Accent)
					ui.Text(c, "Export Postman JSON").FontSize(m.FontLG).FontWeight(600).
						TextColor(tk.TextPrimary).SingleLine().Grow(1)

					closeBtn := ui.ButtonBase(c.Key("export-close")).
						Size(28, 28).Radius(m.RadiusSM).Center().
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Close export")
					if closeBtn.Hovered() {
						closeBtn.Background(tk.BgHover)
					}
					if closeBtn.Clicked() {
						*open = false
						*statusMsg = ""
						*errMsg = ""
					}
					closeBtn.Children(func() {
						ui.Icon(c, icons.X).FontSize(m.IconSM).TextColor(tk.TextSecondary)
					})
				})

				// 2. 主体左右分栏：左侧选择集合/环境，右侧预览 JSON
				ui.Row(c).Grow(1).FillWidth().AlignItems(ui.Stretch).Children(func() {
					ui.Column(c).Width(230).FillHeight().Background(tk.BgApp).
						BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).
						Padding(m.Space3, m.Space2).Gap(m.Space1).Children(func() {
						ui.Text(c, "ITEMS IN WORKSPACE").FontSize(10).FontWeight(600).
							TextColor(tk.TextMuted).Padding(0, m.Space2, m.Space1, m.Space2)

						if len(targets) == 0 {
							ui.Text(c, "Nothing to export").FontSize(m.FontXS).TextColor(tk.TextMuted).
								Padding(m.Space2, m.Space2)
						} else {
							ui.Scroll(c).Grow(1).FillWidth().Children(func() {
								ui.Column(c).FillWidth().Gap(2).Children(func() {
									for _, t := range targets {
										it := t
										k := it.kind + ":" + string(it.id)
										sel := *selectedKey == k
										row := ui.ButtonBase(c.Key("export-item-"+k)).
											FillWidth().Height(30).Padding(0, 8).Gap(6).
											Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.Start).
											Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(it.name)
										if sel {
											row.Background(tk.BgSelected)
										} else if row.Hovered() {
											row.Background(tk.BgHover)
										}
										if row.Clicked() {
											*selectedKey = k
											*statusMsg = ""
											*errMsg = ""
										}
										row.Children(func() {
											ic := icons.Layers
											if it.kind == "environment" {
												ic = icons.Globe
											}
											ui.Icon(c, ic).FontSize(12).TextColor(tk.TextSecondary)
											ui.Text(c, it.name).FontSize(m.FontXS).
												TextColor(tk.TextPrimary).SingleLine().Grow(1)
										})
									}
								})
							})
						}
					})

					ui.Column(c).Grow(1).FillHeight().Padding(m.Space3, m.Space4).Gap(m.Space2).Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
							ui.Text(c, defaultFile).FontSize(m.FontXS).FontWeight(500).
								TextColor(tk.TextSecondary).SingleLine()
							ui.Text(c, fmt.Sprintf("%d bytes", len(previewBytes))).FontSize(11).
								TextColor(tk.TextMuted).SingleLine()
						})

						ui.Box(c).Grow(1).FillWidth().Radius(m.RadiusMD).Background(tk.BgApp).
							Border(m.Border, tk.Border).Padding(m.Space3).Children(func() {
							if previewJSON == "" {
								ui.Box(c).Fill().Center().Children(func() {
									ui.Text(c, "Select a collection or environment to preview JSON").
										FontSize(m.FontXS).TextColor(tk.TextMuted)
								})
							} else {
								ui.Scroll(c).Fill().Children(func() {
									ui.Text(c, previewJSON).Font("monospace").FontSize(11).
										TextColor(tk.TextPrimary)
								})
							}
						})
					})
				})

				// 3. 底部操作栏
				ui.Row(c).FillWidth().Height(50).Padding(0, m.Space5).Gap(m.Space2).
					AlignItems(ui.Center).
					BorderWidth(m.Border, 0, 0, 0).BorderColor(tk.Border).Children(func() {
					if *errMsg != "" {
						ui.Text(c, *errMsg).FontSize(m.FontXS).TextColor(tk.Danger).SingleLine().Grow(1)
					} else if *statusMsg != "" {
						ui.Text(c, *statusMsg).FontSize(m.FontXS).TextColor(tk.Success).SingleLine().Grow(1)
					} else {
						ui.Spacer(c)
					}

					copyBtn := ui.ButtonBase(c.Key("export-copy-json")).
						Height(30).PaddingX(12).Radius(m.RadiusSM).Center().Gap(6).
						Border(m.Border, tk.BorderPopover).Background(tk.BgSurface).
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Copy JSON").
						Disabled(len(previewBytes) == 0)
					if len(previewBytes) > 0 && copyBtn.Hovered() {
						copyBtn.Background(tk.BgHover)
					}
					if copyBtn.Clicked() && len(previewBytes) > 0 {
						c.Services().WriteClipboard(previewJSON)
						*errMsg = ""
						*statusMsg = "Copied JSON to clipboard"
					}
					copyBtn.Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
							ui.Icon(c, icons.Copy).FontSize(12).TextColor(tk.TextSecondary)
							ui.Text(c, "Copy JSON").FontSize(m.FontXS).FontWeight(500).
								TextColor(tk.TextPrimary).SingleLine()
						})
					})

					saveBtn := ui.ButtonBase(c.Key("export-save-file")).
						Height(30).PaddingX(14).Radius(m.RadiusSM).Center().
						Background(tk.Accent).
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Save to File").
						Disabled(len(previewBytes) == 0)
					if len(previewBytes) > 0 && saveBtn.Hovered() {
						saveBtn.Background(tk.AccentHover)
					}
					if saveBtn.Clicked() && len(previewBytes) > 0 && saveRes.start() {
						dataCopy := append([]byte(nil), previewBytes...)
						nameCopy := defaultFile
						saver := saveExportFile
						services := c.Services()
						go func() {
							p, err := saver(nameCopy, dataCopy)
							eStr := ""
							if err != nil {
								eStr = err.Error()
							}
							saveRes.finish(p, eStr)
							services.Invalidate()
						}()
					}
					saveBtn.Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
							ui.Icon(c, icons.Download).FontSize(12).TextColor(tk.OnAccent)
							ui.Text(c, "Save to File").FontSize(m.FontXS).FontWeight(500).
								TextColor(tk.OnAccent).SingleLine()
						})
					})
				})
			})
		})
	})
}

func sanitizeExportFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "export"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(name)
}
