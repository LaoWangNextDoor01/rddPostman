package v2

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestForFollowsTheme 验证 Of/For 按外观返回对应的一套 token。
func TestForFollowsTheme(t *testing.T) {
	lightT := For(&ui.Theme{})
	darkT := For(&ui.Theme{Dark: true})

	if lightT.BgBase != light.BgBase {
		t.Errorf("浅色 BgBase 应为 %v，得到 %v", light.BgBase, lightT.BgBase)
	}
	if darkT.BgBase != dark.BgBase {
		t.Errorf("深色 BgBase 应为 %v，得到 %v", dark.BgBase, darkT.BgBase)
	}
	if lightT.BgBase == darkT.BgBase {
		t.Error("浅色与深色的 BgBase 不应相同")
	}

	if got := For(&ui.Theme{Dark: true}).TextPrimary; got != dark.TextPrimary {
		t.Errorf("For 深色 TextPrimary 应为 %v，得到 %v", dark.TextPrimary, got)
	}
}

// TestKeyValues 验证关键颜色与设计稿一致（02 §2）。
func TestKeyValues(t *testing.T) {
	cases := []struct {
		name string
		got  ui.Color
		want ui.Color
	}{
		{"light BgBase", light.BgBase, ui.Hex("#FFFFFF")},
		{"light BgApp", light.BgApp, ui.Hex("#F9F9F9")},
		{"light Border", light.Border, ui.Hex("#E6E6E6")},
		{"light TextPrimary", light.TextPrimary, ui.Hex("#212121")},
		{"light Accent", light.Accent, ui.Hex("#FF6C37")},
		{"light AccentHover", light.AccentHover, ui.Hex("#ED5B2A")},
		{"light Focus", light.Focus, ui.Hex("#097BED")},
		{"light FocusRing", light.FocusRing, ui.Hex("#E7F0FF")},
		{"light Danger", light.Danger, ui.Hex("#EB2013")},
		{"dark BgBase", dark.BgBase, ui.Hex("#1E1E1E")},
		{"dark BgApp", dark.BgApp, ui.Hex("#191919")},
		{"dark BgTitleBar", dark.BgTitleBar, ui.Hex("#1E1E1E")},
		{"dark Border", dark.Border, ui.Hex("#383838")},
		{"dark TextPrimary", dark.TextPrimary, ui.Hex("#E4E4E4")},
		{"dark Accent", dark.Accent, ui.Hex("#FF6C37")},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %v， want %v", c.name, c.got, c.want)
		}
	}
}

// TestAccentConsistency 验证强调色在两套外观下一致（品牌色不随外观变化）。
func TestAccentConsistency(t *testing.T) {
	if light.Accent != dark.Accent {
		t.Errorf("Accent 应在两套外观下一致: light %v, dark %v", light.Accent, dark.Accent)
	}
	if light.OnAccent != dark.OnAccent {
		t.Errorf("OnAccent 应在两套外观下一致: light %v, dark %v", light.OnAccent, dark.OnAccent)
	}
}

// TestMetricValues 验证关键尺寸与文档一致（02 §3–§5）。
func TestMetricValues(t *testing.T) {
	cases := []struct {
		name string
		got  float32
		want float32
	}{
		{"Space05", Metrics.Space05, 2},
		{"Space1", Metrics.Space1, 4},
		{"Space15", Metrics.Space15, 6},
		{"Space2", Metrics.Space2, 8},
		{"Space3", Metrics.Space3, 12},
		{"Space4", Metrics.Space4, 16},
		{"Space6", Metrics.Space6, 24},
		{"Space8", Metrics.Space8, 32},
		{"RadiusSM", Metrics.RadiusSM, 4},
		{"RadiusMD", Metrics.RadiusMD, 6},
		{"RadiusLG", Metrics.RadiusLG, 8},
		{"FontSM", Metrics.FontSM, 12},
		{"FontXS", Metrics.FontXS, 11},
		{"ButtonHeight", Metrics.ButtonHeight, 28},
		{"InputHeight", Metrics.InputHeight, 28},
		{"SearchHeight", Metrics.SearchHeight, 22},
		{"SelectHeight", Metrics.SelectHeight, 32},
		{"RowHeight", Metrics.RowHeight, 26},
		{"TableHeaderHeight", Metrics.TableHeaderHeight, 30},
		{"SizeXS", Metrics.SizeXS, 16},
		{"SizeMD", Metrics.SizeMD, 24},
		{"SizeXL", Metrics.SizeXL, 32},
		{"TitleBarHeight", Metrics.TitleBarHeight, 36},
		{"StatusBarHeight", Metrics.StatusBarHeight, 24},
		{"SidebarWidth", Metrics.SidebarWidth, 232},
		{"TabBarHeight", Metrics.TabBarHeight, 35},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %v， want %v", c.name, c.got, c.want)
		}
	}
}
