package icons

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 临时验证：把全部图标渲染成一张图，供人工确认。
func TestRenderAllIcons(t *testing.T) {
	names := []string{
		"send", "plus", "trash", "copy", "search",
		"settings", "chevron-down", "x", "folder", "file",
	}
	view := func(c *ui.Context) {
		ui.Column(c).Fill().Padding(24).Gap(14).Background(ui.RGB(255, 255, 255)).Children(func() {
			for _, n := range names {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, Get(n)).FontSize(28).TextColor(ui.RGB(0, 0, 0))
					ui.Text(c, n).FontSize(18).TextColor(ui.RGB(0, 0, 0))
				})
			}
		})
	}

	tt := ui.NewTester(view, 320, 480)
	tt.Frame()
	img := tt.Image()

	// 统计非白像素，确认确有内容被画出来。
	dark := 0
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if img.Pix[i] < 200 || img.Pix[i+1] < 200 || img.Pix[i+2] < 200 {
			dark++
		}
	}
	if dark == 0 {
		t.Fatal("画面全白，图标没有绘制")
	}
	t.Logf("图像 %v，非白像素 %d", img.Bounds(), dark)

	tmpPath := filepath.Join(os.TempDir(), "mygo-icons.png")
	f, err := os.Create(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
