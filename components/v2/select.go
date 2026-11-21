package v2

import (
	themv2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"

	"github.com/egoist/mygo/ui"
)

// SelectOption 表示 Select 下拉菜单的一项。
type SelectOption struct {
	Value string
	Label string
}

// SelectOpts 配置下拉选择框（对齐 Pixso 组件库 · Input 05 与 6-请求-Authorization）。
type SelectOpts struct {
	Key        string
	Label      string
	Width      float32 // 0 = FillWidth
	Height     float32 // 0 = 32
	MenuWidth  float32 // 0 = 跟随 Width（默认 220）
	Value      string
	Options    []SelectOption
	OnSelect   func(value string)
}

// Select 绘制标准 v2 下拉选择框：
// 收起态白底 + 1px Border；展开态 1px Accent (#336FFF) 无外环 + 弹出双层阴影菜单（选中项 BgHover + 右侧对勾）。
func Select(c *ui.Context, o SelectOpts) ui.Element {
	m, tk := themv2.Metrics, themv2.Of(c)
	h := o.Height
	if h == 0 {
		h = 32
	}
	selectedLabel := o.Value
	for _, opt := range o.Options {
		if opt.Value == o.Value {
			selectedLabel = opt.Label
			break
		}
	}
	if selectedLabel == "" && len(o.Options) > 0 {
		selectedLabel = o.Options[0].Label
	}

	btn := ui.ButtonBase(c.Key(o.Key)).
		Height(h).Padding(0, 10).Gap(8).
		Radius(m.RadiusSM).Background(tk.BgSurface).
		AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(o.Label).
		Transition(ui.ElementTransition{Colors: true, Duration: themv2.Motion.Fast})
	if o.Width > 0 {
		btn.Width(o.Width)
	} else {
		btn.FillWidth()
	}

	open := ui.Local(btn, "select-open", func() bool { return false })
	if btn.Clicked() {
		*open = !*open
	}

	switch {
	case *open:
		btn.Border(m.Border, tk.Accent)
	case btn.Pressed():
		btn.Border(m.Border, tk.BorderStrong).Background(tk.BgSubtle)
	case btn.Hovered():
		btn.Border(m.Border, tk.BorderStrong)
	default:
		btn.Border(m.Border, tk.Border)
	}

	btn.Children(func() {
		ui.Text(c, selectedLabel).FontSize(m.FontSM).TextColor(tk.TextPrimary).SingleLine().Grow(1)
		chevronColor := tk.TextMuted
		if *open {
			chevronColor = tk.Accent
		}
		ui.Icon(c, icons.ChevronDown).FontSize(12).TextColor(chevronColor)
	})

	menuW := o.MenuWidth
	if menuW == 0 {
		if o.Width > 0 {
			menuW = o.Width
		} else {
			menuW = 220
		}
	}

	Popover(c, btn, open, PopoverOpts{Width: menuW}, func(panel ui.Element) {
		ui.Column(c).Padding(4, 0).Children(func() {
			for _, opt := range o.Options {
				item := opt
				isSel := item.Value == o.Value
				row := ui.ButtonBase(c.Key(o.Key+"-opt-"+item.Value)).
					FillWidth().Height(30).Padding(0, 12).Gap(8).
					AlignItems(ui.Center).Justify(ui.SpaceBetween).
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(item.Label)
				switch {
				case row.Pressed():
					row.Background(tk.BgSelected)
				case isSel:
					row.Background(tk.BgHover)
				case row.Hovered():
					row.Background(tk.BgSubtle)
				}
				if row.Clicked() {
					if o.OnSelect != nil {
						o.OnSelect(item.Value)
					}
					*open = false
				}
				row.Children(func() {
					weight := 400
					fg := tk.TextSecondary
					if isSel {
						weight = 500
						fg = tk.TextPrimary
					}
					ui.Text(c, item.Label).FontSize(m.FontSM).FontWeight(weight).
						TextColor(fg).SingleLine().Grow(1)
					if isSel {
						ui.Icon(c, icons.Check).FontSize(12).TextColor(tk.TextPrimary)
					}
				})
			}
		})
	})

	return btn
}
