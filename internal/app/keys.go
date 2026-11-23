package app

// 控件 Key 集中声明（仿前端 const 文件的维护习惯）。
//
// Key 的作用：跨帧保持控件身份（状态复用、过渡动画对齐），
// 同时是 ui.NewTester 的点击名，改名字会连带影响测试。
//
// 约定：
//   - 固定 Key 在这里加常量，控件处直接引用，不写裸字符串；
//   - 动态 Key（按数据生成的，如 ws-{id}）留在使用处拼接，
//     只把前缀定义为常量；
//   - 常量不带类型：未类型化字符串常量既能传 c.Key(any)，
//     也能传 v2.IconButtonOpts.Key（string），零转换摩擦；
//   - 命名：key + PascalCase 区域前缀，如 keyTitlebarSearch。
const (
	// 分组 Layout 根节点
	keyLayoutHub       = "layout-hub"
	keyLayoutWorkbench = "layout-workbench"

	// 标题栏
	keyTitlebarBack      = "titlebar-back"
	keyTitlebarForward   = "titlebar-forward"
	keyTitlebarSearch    = "titlebar-search"
	keyTitlebarBell      = "titlebar-bell"
	keyTitlebarSettings  = "titlebar-settings"
	keyTitlebarAvatar    = "titlebar-avatar"
	keyWorkspaceSelector = "workspace-selector"

	// 状态栏与错误日志弹窗
	keyStatusBarLogBtn     = "status-bar-log-btn"
	keyStatusBarGlobalsBtn = "status-bar-globals-btn"
	keyStatusBarToolsBtn   = "status-bar-tools-btn"
	keyGlobalSearchInput   = "global-search-input"
	keyLogCopyAll          = "log-copy-all"
	keyLogClearAll         = "log-clear-all"
	keyLogCloseBtn         = "log-close-btn"
)

// 动态 Key 的前缀：拼接数据标识后使用。
const (
	keyPrefixWS = "ws-" // + workspace ID → "ws-<id>"
)
