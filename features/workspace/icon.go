package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/locales"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// imagePickResult 保存异步系统文件选择器返回的结果，跨 goroutine 通过互斥锁同步到 UI 帧。
type imagePickResult struct {
	mu      sync.Mutex
	pending bool
	picking bool
	path    string
	err     string
}

func (p *imagePickResult) start() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.picking {
		return false
	}
	p.picking = true
	return true
}

func (p *imagePickResult) finish(path, errMsg string) {
	p.mu.Lock()
	p.picking = false
	p.pending = true
	p.path = path
	p.err = errMsg
	p.mu.Unlock()
}

func (p *imagePickResult) drain() (path, errMsg string, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.pending {
		return "", "", false
	}
	p.pending = false
	path, errMsg = p.path, p.err
	p.path, p.err = "", ""
	return path, errMsg, true
}

// openImagePicker 负责弹出本地图片选择器；测试中可替换以避免阻塞系统弹窗。
var openImagePicker = defaultOpenImagePicker

func defaultOpenImagePicker() (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title: locales.WsPickIconTitle(),
		Filters: []mygo.FileFilter{
			{
				Name:       locales.WsPickIconFilter(),
				Extensions: []string{"png", "jpg", "jpeg", "svg", "webp", "gif"},
			},
		},
	})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// cachedIcon 缓存本地路径解码后的图片源，避免每帧重复读盘与解码。
type cachedIcon struct {
	src    ui.ImageSource
	size   int64
	failed bool
}

func (s *wsContentState) loadIcon(path string) cachedIcon {
	path = strings.TrimSpace(path)
	if path == "" {
		return cachedIcon{failed: true}
	}
	if s.iconCache == nil {
		s.iconCache = make(map[string]cachedIcon)
	}
	if entry, ok := s.iconCache[path]; ok {
		return entry
	}

	info, err := os.Stat(path)
	if err != nil {
		entry := cachedIcon{failed: true}
		s.iconCache[path] = entry
		return entry
	}
	data, err := os.ReadFile(path)
	if err != nil {
		entry := cachedIcon{failed: true}
		s.iconCache[path] = entry
		return entry
	}

	var src ui.ImageSource
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		svgDoc, err := ui.ParseSVG(data)
		if err != nil {
			entry := cachedIcon{failed: true}
			s.iconCache[path] = entry
			return entry
		}
		src = svgDoc
	} else {
		bmp, err := ui.DecodeBitmapScaled(data, 128)
		if err != nil {
			entry := cachedIcon{failed: true}
			s.iconCache[path] = entry
			return entry
		}
		src = bmp
	}

	entry := cachedIcon{src: src, size: info.Size()}
	s.iconCache[path] = entry
	return entry
}

// wsIconBadge 渲染圆形工作区图标：存在有效本地图片时展示裁剪后的图片，否则回退为默认网格图标。
func wsIconBadge(c *ui.Context, st *wsContentState, iconPath string, size float32, withBorder bool) {
	m, tk := thV2.Metrics, thV2.Of(c)

	if iconPath != "" {
		if entry := st.loadIcon(iconPath); !entry.failed && entry.src != nil {
			imgBox := ui.Box(c).Size(size, size).Radius(size / 2).Clip().Shrink(0).
				Background(tk.BgSubtle)
			if withBorder {
				imgBox.Border(m.Border, tk.BorderPopover)
			}
			imgBox.Children(func() {
				ui.Image(c, entry.src).Size(size, size).Fit(ui.Cover)
			})
			return
		}
	}

	iconSize := m.IconSM
	if size >= 36 {
		iconSize = m.IconLG
	}
	box := ui.Box(c).Size(size, size).Radius(size / 2).
		Background(tk.BgSubtle).Center().Shrink(0)
	if withBorder {
		box.Border(m.Border, tk.BorderPopover)
	}
	box.Children(func() {
		ui.Icon(c, icons.LayoutGrid).FontSize(iconSize).TextColor(tk.TextSecondary)
	})
}

var (
	sharedIconMu    sync.Mutex
	sharedIconCache = map[string]cachedIcon{}
)

func loadSharedIcon(path string) cachedIcon {
	path = strings.TrimSpace(path)
	if path == "" {
		return cachedIcon{failed: true}
	}
	sharedIconMu.Lock()
	if entry, ok := sharedIconCache[path]; ok {
		sharedIconMu.Unlock()
		return entry
	}
	sharedIconMu.Unlock()

	st := &wsContentState{iconCache: map[string]cachedIcon{}}
	entry := st.loadIcon(path)

	sharedIconMu.Lock()
	sharedIconCache[path] = entry
	sharedIconMu.Unlock()
	return entry
}

// RenderIconBadge 在全局标题栏/下拉等复用位置渲染工作区图标（支持自定义本地图片，失败时回退为默认图标）。
func RenderIconBadge(c *ui.Context, iconPath string, size, radius float32) {
	tk := thV2.Of(c)
	if iconPath != "" {
		if entry := loadSharedIcon(iconPath); !entry.failed && entry.src != nil {
			ui.Box(c).Size(size, size).Radius(radius).Clip().Shrink(0).
				Background(tk.BgSubtle).Children(func() {
				ui.Image(c, entry.src).Size(size, size).Fit(ui.Cover)
			})
			return
		}
	}
	iconSize := size * 0.62
	if iconSize < 12 {
		iconSize = 12
	}
	ui.Box(c).Size(size, size).Radius(radius).Background(tk.BgSubtle).
		AlignItems(ui.Center).Justify(ui.Center).Shrink(0).Children(func() {
		ui.Icon(c, icons.Layers).FontSize(iconSize).TextColor(tk.TextMuted)
	})
}

func formatIconMeta(path string, st *wsContentState) string {
	name := filepath.Base(path)
	entry := st.loadIcon(path)
	if entry.failed || entry.size <= 0 {
		return name
	}
	if entry.size < 1024 {
		return fmt.Sprintf("%s · %d B", name, entry.size)
	}
	return fmt.Sprintf("%s · %d KB", name, (entry.size+512)/1024)
}
