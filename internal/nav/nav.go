// Package nav 定义页面可用的最小导航能力。
package nav

// Navigator 由窗口的路由实现；页面只依赖它，不依赖 App 或 ui.Router。
type Navigator interface {
	// Path 是当前显示的路径。
	Path() string
	// Push 跳转到 target。
	Push(target string)
}
