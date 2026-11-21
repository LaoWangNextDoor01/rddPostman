package workspace

import (
	"path/filepath"
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/internal/nav"
	"rddpostman/internal/routes"
	"rddpostman/internal/store"

	"github.com/egoist/mygo/ui"
)

// createWorkspaceView 按审核通过的 Pixso 29 / 31 / 39 绘制「Create your workspace」表单页：
// 含 Workspace icon（本地图片选择替换默认图标）、Workspace name、语义图标 Type/Setup 卡片
// （Open a local folder 支持读取本机最近/项目目录列表 + 系统目录选择器真实落盘），
// 以及简洁的 Description (Optional) 输入；移除了 Who can access 与 AI 提示。
func createWorkspaceView(c *ui.Context, db *store.Store, r nav.Navigator, st *wsContentState) {
	m, tk := thV2.Metrics, thV2.Of(c)

	// 1. 首次进入创建页时异步加载本地目录候选列表（避免阻塞 UI 线程）
	if st.createFolderScan != nil {
		if items, ok := st.createFolderScan.drain(); ok {
			st.localFolders = items
		}
		if st.createFolderScan.start() {
			loader := loadLocalFolders
			scanRes := st.createFolderScan
			services := c.Services()
			go func() {
				items := loader()
				scanRes.finish(items)
				services.Invalidate()
			}()
		}
	}

	// 2. 回收异步图标文件选择结果
	if st.createIconPick != nil {
		if path, errMsg, ok := st.createIconPick.drain(); ok {
			if errMsg != "" {
				st.createIconErr = errMsg
			} else if path != "" {
				entry := st.loadIcon(path)
				if entry.failed {
					st.createIconErr = "Unsupported or unreadable image file."
				} else {
					st.createIcon = path
					st.createIconErr = ""
					if strings.TrimSpace(st.createName) == "" {
						st.createNameHandle.Focus(c)
					}
				}
			}
		}
	}

	// 3. 回收异步目录选择器结果
	selectLocalDir := func(dir string) {
		dir = filepath.Clean(strings.TrimSpace(dir))
		if dir == "" {
			return
		}
		st.createSetup = "local"
		st.createLocalDir = dir
		st.createLocalErr = ""
		st.createErr = ""
		// 如果用户还没手写工作区名称（或是此前由其它目录自动带入），自动填入目录名
		if strings.TrimSpace(st.createName) == "" || st.createNameAuto {
			base := filepath.Base(dir)
			if base != "" && base != "." && base != string(filepath.Separator) {
				st.createName = base
				st.createNameAuto = true
			}
		}
	}

	if st.createFolderPick != nil {
		if dir, errMsg, ok := st.createFolderPick.drain(); ok {
			if errMsg != "" {
				st.createLocalErr = errMsg
			} else if dir != "" {
				if !isExistingDir(dir) {
					st.createLocalErr = "Selected path is not an accessible folder."
				} else {
					rememberRecentFolder(dir)
					st.localFolders = prependLocalFolder(st.localFolders, dir)
					selectLocalDir(dir)
				}
			}
		}
	}

	triggerPickIcon := func() {
		if st.createIconPick == nil || !st.createIconPick.start() {
			return
		}
		picker := openImagePicker
		pickRes := st.createIconPick
		services := c.Services()
		go func() {
			path, err := picker()
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			}
			pickRes.finish(path, errMsg)
			services.Invalidate()
		}()
	}

	triggerPickFolder := func() {
		st.createSetup = "local"
		if st.createFolderPick == nil || !st.createFolderPick.start() {
			return
		}
		defaultPath := st.createLocalDir
		if defaultPath == "" && len(st.localFolders) > 0 {
			defaultPath = filepath.Dir(st.localFolders[0].Path)
		}
		picker := openFolderPicker
		pickRes := st.createFolderPick
		services := c.Services()
		go func() {
			dir, err := picker(defaultPath)
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			}
			pickRes.finish(dir, errMsg)
			services.Invalidate()
		}()
	}

	ui.Scroll(c).Fill().Background(tk.BgBase).Children(func() {
		ui.Column(c).FillWidth().Padding(28, m.Space6, 40, m.Space6).AlignItems(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(480).Gap(m.Space4).Children(func() {
				// 标题
				ui.Text(c, "Create your workspace").
					FontSize(m.FontXL).Bold().TextColor(tk.TextPrimary).SingleLine()

				// 1. Workspace icon（选择本地图片文件替换 Name 列默认图标）
				ui.Column(c).FillWidth().Gap(m.Space2).Children(func() {
					ui.Text(c, "Workspace icon").FontSize(m.FontXS).FontWeight(500).
						TextColor(tk.TextPrimary).SingleLine()

					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(m.Space3).Children(func() {
						wsIconBadge(c, st, st.createIcon, 40, st.createIcon == "")

						ui.Column(c).Gap(m.Space15).AlignItems(ui.Start).Children(func() {
							if st.createIcon == "" {
								if wsIconUploadButton(c, "ws-icon-choose", "Choose local image").Clicked() {
									triggerPickIcon()
								}
								ui.Text(c, "PNG, JPG or SVG · Replaces the default icon in the workspace list.").
									FontSize(m.FontMicro).TextColor(tk.TextMuted).SingleLine()
							} else {
								ui.Row(c).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
									if wsIconUploadButton(c, "ws-icon-change", "Change image").Clicked() {
										triggerPickIcon()
									}
									rmBtn := ui.ButtonBase(c.Key("ws-icon-remove")).
										Height(26).PaddingX(m.Space1).Center().Radius(m.RadiusSM).
										Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Remove").
										Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
									if rmBtn.Hovered() {
										rmBtn.Background(tk.DangerSoft)
									}
									if rmBtn.Clicked() {
										st.createIcon = ""
										st.createIconErr = ""
									}
									rmBtn.Children(func() {
										ui.Text(c, "Remove").FontSize(m.FontXS).FontWeight(500).
											TextColor(tk.Danger).SingleLine()
									})
								})
								ui.Text(c, formatIconMeta(st.createIcon, st)).
									FontSize(m.FontMicro).TextColor(tk.TextSecondary).SingleLine()
							}
							if st.createIconErr != "" {
								ui.Text(c, st.createIconErr).FontSize(m.FontMicro).TextColor(tk.Danger).SingleLine()
							}
						})
					})
				})

				// 2. Workspace name
				ui.Column(c).FillWidth().Gap(m.Space15).Children(func() {
					ui.Text(c, "Workspace name").FontSize(m.FontXS).FontWeight(500).
						TextColor(tk.TextPrimary).SingleLine()

					prevName := st.createName
					nameIn := ui.TextInputBase(c.Key("ws-create-name"), &st.createName).
						Bind(&st.createNameHandle).
						FillWidth().Height(m.ButtonLarge).PaddingX(m.Space3).
						Radius(m.RadiusSM).Background(tk.BgSurface).
						FontSize(m.FontSM).TextColor(tk.TextPrimary).
						Placeholder("A team name, project or service...").
						PlaceholderColor(tk.TextMuted).
						Label("Workspace name")
					if st.createName != prevName {
						st.createNameAuto = false
					}

					switch {
					case st.createErr != "":
						nameIn.Border(m.Border, tk.Danger)
					case nameIn.Focused():
						nameIn.Border(m.Border, tk.Focus).Shadow(0, 0, 0, 3, tk.FocusRing)
					case nameIn.Hovered():
						nameIn.Border(m.Border, tk.BorderStrong)
					default:
						nameIn.Border(m.Border, tk.BorderPopover)
					}

					if st.createErr != "" {
						ui.Text(c, st.createErr).FontSize(m.FontXS).TextColor(tk.Danger)
					}
					if nameIn.Submitted() {
						submitCreateWorkspace(db, st)
					}
				})

				// 3. Select workspace type
				ui.Column(c).FillWidth().Gap(m.Space15).Children(func() {
					ui.Text(c, "Select workspace type").FontSize(m.FontXS).FontWeight(500).
						TextColor(tk.TextPrimary).SingleLine()

					ui.Row(c).FillWidth().Gap(m.Space2).AlignItems(ui.Stretch).Children(func() {
						wsTypeCard(c, st, "Internal", "Share APIs with your team.", "", icons.Users)
						wsTypeCard(c, st, "Partner", "Share APIs with your partners.", "Upgrade plan", icons.Link)
						wsTypeCard(c, st, "Public", "Everyone can access your APIs.", "", icons.Globe)
					})
				})

				// 4. Choose how you'd like to set up this workspace
				ui.Column(c).FillWidth().Gap(m.Space15).Children(func() {
					ui.Text(c, "Choose how you'd like to set up this workspace").
						FontSize(m.FontXS).FontWeight(500).TextColor(tk.TextPrimary).SingleLine()

					ui.Row(c).FillWidth().Gap(m.Space3).AlignItems(ui.Stretch).Children(func() {
						wsSetupBlankCard(c, st)
						wsSetupLocalCard(c, st, selectLocalDir, triggerPickFolder)
					})
				})

				// 5. Description (Optional)
				ui.Column(c).FillWidth().Gap(m.Space15).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(m.Space15).Children(func() {
						ui.Text(c, "Description").FontSize(m.FontXS).FontWeight(500).
							TextColor(tk.TextPrimary).SingleLine()
						ui.Text(c, "(Optional)").FontSize(m.FontMicro).
							TextColor(tk.TextMuted).SingleLine()
					})

					descIn := ui.TextAreaBase(c.Key("ws-create-desc"), &st.createDesc).
						FillWidth().Height(64).Padding(m.Space2, m.Space3).
						Radius(m.RadiusSM).Background(tk.BgSurface).
						FontSize(m.FontSM).TextColor(tk.TextPrimary).
						Placeholder("Add a brief description for this workspace...").
						PlaceholderColor(tk.TextMuted).
						Label("Workspace description")
					switch {
					case descIn.Focused():
						descIn.Border(m.Border, tk.Focus).Shadow(0, 0, 0, 3, tk.FocusRing)
					case descIn.Hovered():
						descIn.Border(m.Border, tk.BorderStrong)
					default:
						descIn.Border(m.Border, tk.BorderPopover)
					}
				})

				// 6. 底部按钮行：Cancel + Create Workspace
				ui.Row(c).FillWidth().Justify(ui.End).Gap(m.Space2).Margin(m.Space1, 0, 0, 0).Children(func() {
					if wsSecondaryButton(c, "ws-create-cancel", "Cancel").Clicked() {
						st.resetCreateForm()
						if r != nil {
							r.Push(routes.HubWorkspaces)
						}
					}
					disabled := strings.TrimSpace(st.createName) == "" ||
						(st.createSetup == "local" && strings.TrimSpace(st.createLocalDir) == "")
					if wsPrimaryButton(c, "ws-create-submit", "Create Workspace", disabled).Clicked() {
						if submitCreateWorkspace(db, st) && r != nil {
							r.Push(routes.HubWorkspaces)
						}
					}
				})
			})
		})
	})
}

func wsIconUploadButton(c *ui.Context, key, label string) ui.Element {
	m, tk := thV2.Metrics, thV2.Of(c)
	b := ui.ButtonBase(c.Key(key)).
		Height(26).PaddingX(10).Gap(m.Space15).Center().
		Radius(m.RadiusSM).Border(m.Border, tk.BorderPopover).
		Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(label).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	switch {
	case b.Pressed():
		b.Background(tk.BgSelected)
	case b.Hovered():
		b.Background(tk.BgHover)
	default:
		b.Background(tk.BgSurface)
	}
	b.Children(func() {
		ui.Icon(c, icons.Upload).FontSize(m.IconXS).TextColor(tk.TextSecondary)
		ui.Text(c, label).FontSize(m.FontXS).FontWeight(500).TextColor(tk.TextPrimary).SingleLine()
	})
	return b
}

func submitCreateWorkspace(db *store.Store, st *wsContentState) bool {
	name := strings.TrimSpace(st.createName)
	if name == "" {
		st.createErr = "Please enter a workspace name."
		return false
	}
	dir := ""
	if st.createSetup == "local" {
		dir = strings.TrimSpace(st.createLocalDir)
		if dir == "" {
			st.createLocalErr = "Please choose a local folder first."
			return false
		}
		if !isExistingDir(dir) {
			st.createLocalErr = "Selected folder does not exist."
			return false
		}
	}

	if db != nil {
		var (
			ws  *domain.Workspace
			err error
		)
		if dir != "" {
			ws, err = db.CreateWorkspaceAt(name, dir)
		} else {
			ws, err = db.CreateWorkspace(name)
		}
		if err != nil {
			st.createErr = err.Error()
			return false
		}
		if dir != "" {
			rememberRecentFolder(dir)
		}
		if ws != nil {
			desc := strings.TrimSpace(st.createDesc)
			wsType := st.createType
			iconPath := strings.TrimSpace(st.createIcon)
			if desc != "" || wsType != "" {
				_ = db.UpdateWorkspace(ws.ID, func(w *domain.Workspace) {
					if desc != "" {
						w.Description = desc
					}
					if wsType != "" {
						w.Type = wsType
					}
				})
			}
			if iconPath != "" {
				if err := db.SetWorkspaceIcon(ws.ID, iconPath); err != nil {
					st.createIconErr = err.Error()
				}
			}
			_ = db.SetActiveWorkspace(ws.ID)
		}
	}
	st.resetCreateForm()
	return true
}

func wsTypeCard(c *ui.Context, st *wsContentState, title, desc, badge string, icon *ui.SVG) {
	m, tk := thV2.Metrics, thV2.Of(c)
	selected := st.createType == title

	card := ui.ButtonBase(c.Key("ws-type-"+title)).
		Grow(1).Padding(m.Space3).Radius(m.RadiusSM).
		Background(tk.BgSurface).
		AlignItems(ui.Start).Justify(ui.Start).
		Cursor(ui.CursorPointer).Role(ui.RoleRadio).Label(title).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

	switch {
	case selected:
		card.Border(m.Border, tk.Focus)
	case card.Hovered():
		card.Border(m.Border, tk.BorderStrong)
	default:
		card.Border(m.Border, tk.BorderPopover)
	}

	if card.Clicked() {
		st.createType = title
	}

	iconColor := tk.TextSecondary
	if selected {
		iconColor = tk.Focus
	}

	card.Children(func() {
		ui.Column(c).FillWidth().Gap(m.Space15).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(m.Space15).Children(func() {
				ui.Icon(c, icon).FontSize(m.IconSM).TextColor(iconColor)
				ui.Text(c, title).FontSize(m.FontSM).FontWeight(600).
					TextColor(tk.TextPrimary).SingleLine()
			})
			ui.Text(c, desc).FontSize(m.FontMicro).TextColor(tk.TextMuted)
			if badge != "" {
				ui.Text(c, badge).FontSize(m.FontMicro).TextColor(tk.Focus).SingleLine()
			}
		})
	})
}

func wsSetupBlankCard(c *ui.Context, st *wsContentState) {
	m, tk := thV2.Metrics, thV2.Of(c)
	selected := st.createSetup == "blank"

	card := ui.ButtonBase(c.Key("ws-setup-blank")).
		Grow(1).MinHeight(152).Padding(m.Space3).Radius(m.RadiusSM).
		Background(tk.BgSurface).
		AlignItems(ui.Start).Justify(ui.Start).
		Cursor(ui.CursorPointer).Role(ui.RoleRadio).Label("Blank workspace").
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

	switch {
	case selected:
		card.Border(m.Border, tk.Focus)
	case card.Hovered():
		card.Border(m.Border, tk.BorderStrong)
	default:
		card.Border(m.Border, tk.BorderPopover)
	}

	if card.Clicked() {
		st.createSetup = "blank"
		st.createLocalErr = ""
	}

	iconColor := tk.TextSecondary
	if selected {
		iconColor = tk.Focus
	}

	card.Children(func() {
		ui.Column(c).FillWidth().Gap(m.Space15).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(m.Space15).Children(func() {
				ui.Icon(c, icons.FilePlus).FontSize(m.IconSM).TextColor(iconColor)
				ui.Text(c, "Blank workspace").FontSize(m.FontSM).FontWeight(600).
					TextColor(tk.TextPrimary).SingleLine()
			})
			ui.Text(c, "I want to start from scratch.").FontSize(m.FontMicro).TextColor(tk.TextMuted)
		})
	})
}

func wsSetupLocalCard(
	c *ui.Context,
	st *wsContentState,
	onSelectDir func(string),
	onPickNewFolder func(),
) {
	m, tk := thV2.Metrics, thV2.Of(c)
	selected := st.createSetup == "local"

	// 外层用 Column 避免 ButtonBase 嵌套 ButtonBase，卡片标题、目录行和「Open New Folder」各自独立交互
	card := ui.Column(c.Key("ws-setup-local-card")).
		Grow(1).MinHeight(152).Padding(m.Space3).Radius(m.RadiusSM).
		Background(tk.BgSurface).Gap(m.Space15).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

	switch {
	case selected:
		card.Border(m.Border, tk.Focus)
	case card.Hovered():
		card.Border(m.Border, tk.BorderStrong)
	default:
		card.Border(m.Border, tk.BorderPopover)
	}

	iconColor := tk.TextSecondary
	if selected {
		iconColor = tk.Focus
	}

	card.Children(func() {
		// 1. 卡片头部：点击切换至 local 模式
		hdrBtn := ui.ButtonBase(c.Key("ws-setup-local")).
			FillWidth().Justify(ui.Start).AlignItems(ui.Center).Gap(m.Space15).
			Cursor(ui.CursorPointer).Role(ui.RoleRadio).Label("Open a local folder")
		if hdrBtn.Clicked() {
			st.createSetup = "local"
		}
		hdrBtn.Children(func() {
			ui.Icon(c, icons.Folder).FontSize(m.IconSM).TextColor(iconColor)
			ui.Text(c, "Open a local folder").FontSize(m.FontSM).FontWeight(600).
				TextColor(tk.TextPrimary).SingleLine()
		})

		// 2. 本地目录候选列表（真实读取最近目录 / 同级项目目录）
		if len(st.localFolders) == 0 {
			ui.Text(c, "Select an existing folder or Git repo on your machine.").
				FontSize(m.FontMicro).TextColor(tk.TextMuted)
		} else {
			selKey := normalizePathKey(st.createLocalDir)
			ui.Column(c).FillWidth().Gap(2).Children(func() {
				for _, item := range st.localFolders {
					it := item
					isChosen := selected && selKey != "" && normalizePathKey(it.Path) == selKey
					rowBtn := ui.ButtonBase(c.Key("ws-local-dir-"+normalizePathKey(it.Path))).
						FillWidth().Height(22).PaddingX(m.Space15).Radius(m.RadiusSM).
						AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(m.Space15).
						Cursor(ui.CursorPointer).Role(ui.RoleOption).Label(it.Name).
						Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
					switch {
					case isChosen:
						rowBtn.Background(tk.AccentSoft)
					case rowBtn.Hovered():
						rowBtn.Background(tk.BgSubtle)
					}
					if rowBtn.Clicked() {
						onSelectDir(it.Path)
					}
					rowBtn.Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(m.Space15).Grow(1).Children(func() {
							icCol := tk.TextMuted
							txtCol := tk.TextSecondary
							weight := 400
							if isChosen {
								icCol = tk.Focus
								txtCol = tk.TextPrimary
								weight = 500
							}
							ui.Icon(c, icons.Folder).FontSize(m.IconXS).TextColor(icCol)
							ui.Text(c, it.Name).FontSize(m.FontXS).FontWeight(weight).
								TextColor(txtCol).SingleLine()
						})
						if isChosen {
							ui.Icon(c, icons.Check).FontSize(11).TextColor(tk.Focus)
						}
					})
				}
			})
		}

		// 已选路径提示或错误
		if st.createLocalErr != "" {
			ui.Text(c, st.createLocalErr).FontSize(m.FontMicro).TextColor(tk.Danger)
		} else if selected && st.createLocalDir != "" {
			ui.Text(c, st.createLocalDir).FontSize(m.FontMicro).TextColor(tk.TextMuted).SingleLine()
		}

		// 3. 底部「Open New Folder」系统目录选择按钮
		ui.Row(c).Margin(m.Space05, 0, 0, 0).Children(func() {
			pickBtn := ui.ButtonBase(c.Key("ws-open-new-folder")).
				Height(24).PaddingX(m.Space2).Center().
				Radius(m.RadiusSM).Border(m.Border, tk.BorderPopover).
				Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Open New Folder").
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			switch {
			case pickBtn.Pressed():
				pickBtn.Background(tk.BgSelected)
			case pickBtn.Hovered():
				pickBtn.Background(tk.BgHover)
			default:
				pickBtn.Background(tk.BgSurface)
			}
			if pickBtn.Clicked() {
				onPickNewFolder()
			}
			pickBtn.Children(func() {
				ui.Text(c, "Open New Folder").FontSize(m.FontMicro).FontWeight(500).
					TextColor(tk.TextPrimary).SingleLine()
			})
		})
	})
}
