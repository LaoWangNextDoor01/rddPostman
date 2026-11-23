package collection

import (
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/domain"
	"rddpostman/internal/routes"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// addMenuButton 是侧栏工具栏里的「新建」按钮与 v2 下拉菜单（完整 Hover/Pressed 态）。
func addMenuButton(c *ui.Context, props pageProps) {
	addBtn := v2.IconButton(c, v2.IconButtonOpts{
		Key:      "add",
		Label:    locales.Add(),
		Tooltip:  locales.Add(),
		Icon:     icons.Plus,
		Size:     24,
		IconSize: 14,
		Radius:   4,
	})
	if addBtn.Clicked() {
		props.state.addOpen = !props.state.addOpen
	}

	v2.Popover(c, addBtn, &props.state.addOpen, v2.PopoverOpts{Width: 196}, func(panel ui.Element) {
		ui.Column(c).Padding(4).Gap(2).Children(func() {
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.Library, Key: "add-collection", Label: locales.NewCollection(),
			}).Clicked() {
				if props.actions.createCollectionTab != nil {
					props.actions.createCollectionTab()
				}
				props.state.addOpen = false
			}

			if len(props.collections) > 0 {
				if v2.MenuButton(c, v2.MenuButtonProps{
					Icon: icons.FolderPlus, Key: "add-folder", Label: locales.NewFolder(),
				}).Clicked() {
					props.actions.createNode(domain.ID(props.state.selected), domain.KindFolder)
					props.state.addOpen = false
				}
				if v2.MenuButton(c, v2.MenuButtonProps{
					Icon: icons.Send, Key: "add-request", Label: locales.NewRequest(),
				}).Clicked() {
					props.actions.createNode(domain.ID(props.state.selected), domain.KindRequest)
					props.state.addOpen = false
				}
			}

			v2.PopoverDivider(c)

			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.Code, Key: "add-import-curl", Label: "Import cURL",
			}).Clicked() {
				props.state.importCurlOpen = true
				props.state.importCurlText = ""
				props.state.importCurlErr = ""
				props.state.addOpen = false
			}
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.File, Key: "add-import-postman", Label: "Import Postman Collection",
			}).Clicked() {
				props.state.importPostmanOpen = true
				props.state.importPostmanText = ""
				props.state.importPostmanErr = ""
				props.state.addOpen = false
			}

			v2.PopoverDivider(c)

			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.Globe, Key: "add-environment", Label: locales.RailEnvironments(),
			}).Clicked() {
				props.actions.navigate(routes.WorkbenchEnvironmentsOf(string(props.workspaceID)))
				props.state.addOpen = false
			}
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.BookOpen, Key: "add-docs", Label: locales.DocsTitle(),
			}).Clicked() {
				props.actions.navigate(routes.WorkbenchDocsOf(string(props.workspaceID)))
				props.state.addOpen = false
			}
		})
	})
}
