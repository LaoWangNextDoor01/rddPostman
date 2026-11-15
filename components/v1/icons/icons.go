// Package icons 提供应用用到的 Lucide 图标。
//
// 只有 assets 目录下的 .svg 会被打包，scripts/icons.txt 是唯一清单：
// 清单里没有的图标不会下载，目录里多出来的会被 scripts/sync-icons.sh 删除。
package icons

import (
	"embed"
	"fmt"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:embed assets/*.svg
var files embed.FS

var (
	mu    sync.Mutex
	cache = map[string]*ui.SVG{}
)

// Get 按 Lucide 的文件名返回图标，例如 icons.Get("chevron-down")。
// 图标在第一次用到时解析，之后复用同一个 *ui.SVG。
func Get(name string) *ui.SVG {
	mu.Lock()
	defer mu.Unlock()
	if s, ok := cache[name]; ok {
		return s
	}
	data, err := files.ReadFile("assets/" + name + ".svg")
	if err != nil {
		panic(fmt.Sprintf("icons: %q 未打包，请在 scripts/icons.txt 中加上它并运行 scripts/sync-icons.sh", name))
	}
	s := ui.MustParseSVG(data)
	cache[name] = s
	return s
}
