package settings

// ThemeMode 表示应用外观主题模式：跟随系统、浅色或深色。
type ThemeMode string

const (
	ThemeSystem ThemeMode = "system"
	ThemeLight  ThemeMode = "light"
	ThemeDark   ThemeMode = "dark"
)

// DialogOptions 承载全局设置弹窗的运行时选项与回调。
type DialogOptions struct {
	InitialTab   string
	ThemeMode    ThemeMode
	OnThemeMode  func(ThemeMode)
	DataDir      string
}
