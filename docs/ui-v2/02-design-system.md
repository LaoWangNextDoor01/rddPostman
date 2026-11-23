# 02 · 设计系统与 Token 规划

> 设计源：Pixso 文件 `EMFtfhEwU9AggnqdzkFnQw` 的「组件」页。
> 组件页有明确标注的规格直接采用；没有标注的颜色/字号标为「待核对」，Phase 0 逐项对照。

## 1. 设计原则

- **桌面优先、信息密度高**：行高 26 / 30，控件 24–32 高，避免移动端的大留白。
- **中性底 + 橙色强调**：大部分界面是白 / 浅灰，橙色只用于主操作、选中、发送。
- **细边框、低阴影**：分割线 1px `#E6E6E6`；弹层才用阴影。
- **4px 网格**：间距以 2 / 4 / 6 / 8 / 12 / 16 / 20 / 24 / 32 为主。
- **状态色克制**：方法、状态码、错误使用语义色，不做大面积填充。

## 2. 颜色 Token

### 2.1 Light

| Token | 值 | 用途 |
| --- | --- | --- |
| `BgBase` | `#FFFFFF` | 主内容区、编辑器、表格底 |
| `BgApp` | `#F9F9F9` | 工作台侧栏、标签条、表头 |
| `BgTitleBar` | `#FFFFFF` | 标题栏（与主内容同底色，靠分割线区分；旧值 #F2F2F2 为取样污染，已更正） |
| `BgStatusBar` | `#FFFFFF` | 状态栏 |
| `BgSubtle` | `#F2F2F2` | Segment 轨道、搜索框默认底 |
| `BgHover` | `#EDEDED` | 通用 hover |
| `BgSelected` | `#E6E6E6` | 选中/按下 |
| `BgSurface` | `#FFFFFF` | 卡片、弹层、输入框 |
| `Border` | `#E6E6E6` | 分割线、输入框默认边框 |
| `BorderStrong` | `#A6A6A6` | 输入框 hover 边框、复选框边框 |
| `TextPrimary` | `#212121` | 主文字 |
| `TextSecondary` | `#6B6B6B` | 次要文字、表头 |
| `TextMuted` | `#909090` | 占位符、说明 |
| `TextDisabled` | `#B5B5B5` | 禁用文字 |
| `Accent` | `#FF6C37` | 主按钮、选中、方法 POST |
| `AccentHover` | `#ED5B2A` | 主按钮 hover |
| `AccentPressed` | `#D03E0A` | 主按钮按下（+ 内阴影） |
| `AccentSoft` | `#FFF1EB` | 强调浅底、选中行（建议值，待核对） |
| `OnAccent` | `#FFFFFF` | 主按钮文字 |
| `Focus` | `#097BED` | 输入框聚焦边框 |
| `FocusRing` | `#E7F0FF` | 3px 聚焦外环 |
| `Danger` | `#EB2013` | 错误输入、删除 |
| `DangerSoft` | `#FFEBE7` | 危险按钮 hover |
| `Warning` | `#C77700` | 警告、POST 方法（建议值） |
| `Success` | `#1F9D6B` | 成功、GET 方法（建议值） |
| `Info` | `#2F6FEB` | 链接、PUT 方法（建议值） |
| `Overlay` | `#0F172A66` | 模态遮罩（沿用现有值） |

### 2.2 Dark

| Token | 值 | 用途 |
| --- | --- | --- |
| `BgBase` | `#1E1E1E` | 主内容区 |
| `BgApp` | `#191919` | 工作台侧栏、标签条 |
| `BgTitleBar` | `#1E1E1E` | 标题栏（与主内容同底色） |
| `BgStatusBar` | `#1E1E1E` | 状态栏 |
| `BgSubtle` | `#2C2C2C` | Segment 轨道、搜索框底 |
| `BgHover` | `#333333` | 通用 hover（建议值） |
| `BgSelected` | `#3A3A3A` | 选中/按下 |
| `BgSurface` | `#1E1E1E` | 卡片、弹层 |
| `Border` | `#383838` | 分割线 |
| `BorderStrong` | `#4E4E4E` | 输入框 hover 边框（建议值） |
| `TextPrimary` | `#E4E4E4` | 主文字 |
| `TextSecondary` | `#A8A8A8` | 次要文字 |
| `TextMuted` | `#858585` | 占位符 |
| `TextDisabled` | `#636363` | 禁用文字 |
| `Accent` | `#FF6C37` | 主色不变 |
| `AccentHover` | `#ED5B2A` | 主按钮 hover |
| `AccentPressed` | `#D03E0A` | 主按钮按下 |
| `AccentSoft` | `#3A2A24` | 强调浅底（建议值，待核对） |
| `OnAccent` | `#FFFFFF` | 主按钮文字 |
| `Focus` | `#4C9AFF` | 深色下提高对比（建议值） |
| `FocusRing` | `#1E3A5F` | 聚焦外环（建议值） |
| `Danger` | `#FF5F57` | 错误、删除 |
| `DangerSoft` | `#3A2220` | 危险按钮 hover（建议值） |
| `Success` | `#34D399` | 成功、GET（建议值） |
| `Warning` | `#F2B34B` | 警告、POST（建议值） |
| `Info` | `#6C9EE8` | 链接、PUT（建议值） |

> 取色依据：导出帧的主色直方图 + 组件页标注。未标注的值先按上表落地，Phase 0 用 Pixso 节点属性逐项核对。

## 3. 字体与字阶

### 3.1 字体

- UI：`-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif`
- 等宽：`"SF Mono", "Cascadia Code", Consolas, monospace`
- 不引入 Web Font，优先系统字体；如果后续确认设计使用 Inter，再评估打包。

### 3.2 字阶（建议）

| Token | 字号 / 行高 | 字重 | 用途 |
| --- | --- | --- | --- |
| `FontMicro` | 10 / 14 | 500 | 方法标签、徽标、极小的元信息 |
| `FontXS` | 11 / 16 | 400 / 500 | 表头、状态码、辅助说明 |
| `FontSM` | 12 / 18 | 400 / 500 | 默认 UI 文字、按钮、输入、表格正文 |
| `FontMD` | 14 / 22 | 400 / 500 | 正文强调、面包屑当前项、卡片标题 |
| `FontLG` | 16 / 24 | 500 | 页面区块标题 |
| `FontXL` | 20 / 28 | 500 | 页面主标题 |
| `FontXXL` | 24 / 32 | 500 | 创建页/空态大标题 |

依据：

- 组件页标注：按钮 12 Medium；表头 11 Medium；方法标签 10 SemiBold / 11 SemiBold；面包屑当前项加粗。
- 页面标题级字号未逐项标注，上表为建议值，Phase 0 对照 28 / 29 号画面核对。

## 4. 间距与圆角

### 4.1 间距

沿用 **Tailwind 的步进命名**：`SpaceN = N × 4px`，半步写作 `05` / `15`（对应 Tailwind 的 `0.5` / `1.5`）。与 v1 同名同值，迁移时无需换算。

```
Space0  = 0
Space05 = 2    （Tailwind 0.5）
Space1  = 4
Space15 = 6    （Tailwind 1.5）
Space2  = 8
Space3  = 12
Space4  = 16
Space5  = 20
Space6  = 24
Space8  = 32
```

### 4.2 圆角

| Token | 值 | 用途 |
| --- | --- | --- |
| `RadiusSM` | 4 | 按钮、输入、菜单项、Tab chip、树选中行 |
| `RadiusMD` | 6 | 搜索框、较大的输入/卡片 |
| `RadiusLG` | 8 | 弹层、面板、空态容器 |
| `RadiusFull` | 999 | 头像、圆形图标底 |

## 5. 布局尺寸

| Token | 值 | 说明 |
| --- | --- | --- |
| `TitleBarHeight` | 36 | **实测对齐 Postman**（2026-11-19 肉眼对比 34 矮 1px，定 36）：内容 35 + 底部 1px 分割线，border-box 外框 36 |
| `StatusBarHeight` | 24 | 设计稿测量（底边 y=776） |
| `SidebarWidth` | 232 | 管理端与工作台统一侧栏宽度（设计稿分割线 x=232） |
| `TabBarHeight` | 35 | 工作台顶部标签条（y=35–70） |
| `RowHeight` | 26 | 表格行 / 树行 |
| `TableHeaderHeight` | 30 | 表头 |
| `ButtonSmall` | 24 | 工具条文字按钮 |
| `ButtonHeight` | 28 | 默认按钮 |
| `ButtonLarge` | 32 | 弹窗/空态主操作 |
| `InputHeight` | 28 | 单行输入 |
| `SearchHeight` | 22 | 搜索框 |
| `SelectHeight` | 32 | 下拉触发器 |
| `ContentPaddingX` | 20 | 页面左右留白（建议值） |
| `ContentPaddingY` | 20 | 页面上下留白（建议值） |

### 4.3 动效时长（Motion）

Postman 风格：**快速、克制的淡入淡出**，不做弹跳与大幅位移。缓动用 MyGo 默认的 `EaseOut`；未来接系统「减少动态效果」时，把 `Duration` 置为负数即可整体关闭动画。

| Token | 值 | 用途 |
| --- | --- | --- |
| `Motion.Fast` | 120ms | hover / press 的背景、边框、文字色过渡 |
| `Motion.Base` | 200ms | 尺寸与位置过渡（MyGo 默认值，面板开合、侧栏折叠） |

用法（MyGo 的 `Element.Transition` 会自动插值背景/边框色、位置、尺寸与阴影，视图只需声明状态）：

```go
// 只过渡颜色（hover 底色与文字色变化）：
b := ui.ButtonBase(c.Key(key)).Background(tk.BgSubtle)
if b.Hovered() {
    b.Background(tk.BgHover)
}
b.Transition(ui.ElementTransition{Colors: true, Duration: themv2.Motion.Fast})

// 过渡全部属性（面板开合、行增删）：
el.Transition(ui.ElementTransition{Duration: themv2.Motion.Base})
```

规则：

- 元素必须有跨帧稳定的 `Key`（或稳定的兄弟次序），否则过渡无从对齐；
- 优先用属性开关（`Colors: true`）收窄过渡范围，避免 hover 时连位置一起动；
- 快捷键角标这类「提示性显隐」保持瞬时（`Opacity(0/1)`），不做淡入——Postman 的提示都是即现即隐；
- Enter/Exit（元素增删的收缩与淡出）在列表/面板阶段再引入，本版只做状态色过渡。

### 5.1 标题栏的高度定案：36

标题栏高度 **36**（内容 35 + 底部 1px 分割线，border-box 外框 36）。定案依据（2026-11-19）：与 Postman 桌面版并排肉眼对比，34 矮约 1px，定 36——与设计稿当初测量的 36（内容 35 + y=35 分割线）互相印证。

- **Windows / Linux**：在 `WindowOptions` 显式设置 `TitleBarHeight: 36`（覆盖 Windows 默认的 32）。系统按钮会填满 36，`c.TitleBar().Height` 返回 36。
- **macOS**：`TitleBarHeight` 不生效，`c.TitleBar()` 返回交通灯占用的区域（约 28）。页面行高直接用 `bar.Height`，不设最小值，保持原生观感。
- **全屏 / 无系统按钮**：`c.TitleBar()` 为 0，此时用 36 作为自绘标题栏的兜底高度。
- **border-box 注意**：`Height(36) + 底部 1px 边框` 的内容区是 35；放 20–24 高的控件足够。
- 页面不要写死高度；始终通过 `c.TitleBar()` 计算，兜底值只来自 `theme.Metrics.TitleBarHeight`（36）。
- 设计稿与代码保持同步（2026-11-19 定稿）：组件 `cmp / row / title bar` 高 36（底部 1px 分割线 stroke），65 块画板 `div.workspace` 从 y=36、高 740，锚定在标题栏下的 overlay（workspace 下拉、菜单等）与设置弹窗等居中弹层位置见 32 号画面。
- 决策记录：中途曾按「成熟应用纯头部落在系统高度 32」改过一版，实测对比 Postman 后回滚到 36。教训：像素级 UI 决策，实测 > 推理。

## 6. 组件规范

### 6.1 Button

- **IconButton**（`components/v2`，已落地）：24×24 纯图标钮，默认次要色图标、hover `BgHover`/按下 `BgSelected`、光标默认 Pointer 手型（`Cursor` 配置可覆盖）、`Label` 无障碍名、可选 Tooltip。

基础：高 28、圆角 4、字号 12 Medium、左右内边距 12、图标与文字间距 8。尺寸 24 / 28 / 32。

| 类型 | 默认 | Hover | Pressed | Disabled |
| --- | --- | --- | --- | --- |
| Primary | `#FF6C37` 底 + 白字 | `#ED5B2A` | `#D03E0A` + 内阴影 | 整体 40% |
| Secondary（Save / Share） | 白底 + 1px `#E6E6E6` | `#EDEDED` 底 | `#E6E6E6` + 内阴影 | 整体 40% |
| Ghost | 无底无框 | `#EDEDED` 底 | `#E6E6E6` | 40% |
| Danger | 白底 + 红字/红图标（菜单项如 Quit 默认态与普通项一致：透明底 + `TextSecondary`） | `#FFEBE7`（`DangerSoft`）底 + `Danger` 红字 | 加深红底 + `Danger` 红字 | 40% |

组合：

- 图标 + 文字 + 下拉：右侧 `chevron-down` 12，间距 8。
- Split Button：主区 + 箭头区，分隔线主按钮 `#FF9B73`（30% 白叠加），次级 `#E6E6E6`；两段各自 hover，圆角由外层裁切保证。

### 6.2 Input / Select / Checkbox

- 单行输入：白底 + 1px `#E6E6E6`，高 28，圆角 4；hover 边框 `#A6A6A6`；focus 1px `#097BED` + 3px `#E7F0FF` 外环；error 1px `#EB2013` + 红色外环。
- 搜索框：高 22，圆角 6，默认底 `#F2F2F2` + 文字 `#909090`；hover 白底 + 文字/图标转 `#212121`（2026-11-19 按用户确认的交互修正，替换原 #EDEDED 方案）；focus 白底 + 蓝框 + 外环；disabled `#F9F9F9`。`Ctrl/Cmd+K` 角标默认透明、hover 淡入。
- 下拉触发器：高 32，圆角 4；默认白底 + 1px `#E6E6E6`；展开 1px `#097BED` + 外环；disabled `#F9F9F9` + 文字 `#A6A6A6`。
- 弹出菜单：白底 + 1px `#D8D8D8`，圆角 4，下方阴影；菜单项 hover `#F2F2F2`，选中 `#EDEDED`，禁用 `#A6A6A6`。
- Checkbox：未选边框 1px `#A6A6A6`；已选 `#FF6C37` 底 + 白色勾；禁用底 `#EDEDED`。

### 6.3 Navigation

- Segment：轨道高 28，内边距 2，chip 高 24，圆角 4，字号 12；选中白底 + 轻阴影；轨道底 `#F2F2F2`，未选 hover `#E6E6E6`。
- Tab chip：选中白底 + 轻边框/阴影；未选透明；hover `#EDEDED`；请求标签带方法圆点和关闭按钮。
- **关闭按钮（Tab chip 内）**：图标 12×12，热区 16×16（图标四周外扩 2px），圆角 4；Hover 时热区底色 `#E6E6E6`（Dark 用 `#3A3A3A`，建议值）。显示时机按成熟应用惯例：激活标签常显 ×，非激活标签整条 chip hover 时出现 ×，未保存改动用橙点替代 ×（hover 切回 ×）。点击 × 关闭标签，中键直接关闭；Phase 3 实现时遵循。组件库 Navigation 页 02 区已补「关闭 Hover」示例卡。
- 面包屑：文件夹图标 14 + 分隔 `chevron` 12（`#A6A6A6`）+ 当前项加粗；上下间距 6。
- 侧栏项：高 28–32，圆角 4；hover `#F2F2F2`，选中 `#EDEDED` 或 `#E6E6E6`。

### 6.4 Data

- 表格：表头高 30，行高 26，分隔线 1px `#E6E6E6`；表头 11 Medium `#6B6B6B`，正文 12；表头底 `#F9F9F9`，单元格左内边距 10；行 hover `#F9F9F9`，选中 `#EDEDED`；无外边框。
- 树：集合行 = caret 11 + folder 25 + 文字；请求行 = 方法标签 38 + 名称；目录行 = caret 23 + folder 37 + 文字；选中行 `#EDEDED`，圆角 4。
- 状态码：2xx 绿、3xx 蓝、4xx 琥珀、5xx 红；字号 11 SemiBold。
- 方法标签：GET 绿、POST 橙、PUT 蓝、DELETE 红、PATCH 紫；URL 栏用 11 SemiBold，列表用 10 SemiBold 胶囊。

### 6.5 Overlay

- Dialog：白底、圆角 8、阴影；标题 14–16，正文 12；按钮右对齐。
- **Popover**（`components/v2`，已落地）：白底 `BgSurface`、1px `BorderPopover`（#D8D8D8 / #4E4E4E）、圆角 `RadiusLG`(8)、双层阴影（0 1 2 黑 8% + 0 12 28 黑 14%）、锚点下方 8px、空间不足自动上翻。基于 `ui.PopoverBase`，`*open` 状态用 `ui.Local` 挂在触发元素上；内容超长在 panel 内用 `ui.Scroll`。分割线用 `v2.PopoverDivider`（#EDEDED）。
- **Workspace 切换下拉（`overlay / workspace dropdown`）**：按桌面 12px 紧凑密度统一收敛——弹层宽 `276`、搜索框高 `28`（内边距 `8`，字号 `12`）、列表行高 `32`（图标 `20×20` 圆角 `4`，字号 `12 Medium`，当前项右侧 `12px` 对钩）、底部操作栏高 `36`（左右 `50/50` 等分 + 贯通 `1px` 竖线，`13px` 图标 + `12px Medium` 文字）。
- Tooltip：深色底或白底 + 边框，字号 11–12，最大宽度 240。
- Toast / 通知：按现有设计稿 30 号画面补齐（本版先不做复杂动效）。

### 6.6 Empty State

- 居中容器，最大宽度 420；图标/插画 148 左右；标题 16–18；说明 12；快捷键提示行高 26。
- 工作台无标签页沿用设计稿 75 号画面的结构，但视觉按新 token 重绘。

## 7. 图标

- 统一 Lucide，尺寸 12 / 14 / 16 / 18，线宽 1.5–2。
- 缺失图标用语义最接近的 Lucide 替代，不改设计稿。建议映射：

| 场景 | Lucide |
| --- | --- |
| 团队 / 成员 | `users` |
| Workspaces | `layout-grid` 或 `layers` |
| Application Inventory | `boxes` / `package` |
| API Catalog | `book-open` / `library-big` |
| Private API Network | `lock` / `network` |
| Public API Network | `globe` |
| Agent Context | `sparkles` / `bot` |
| Integrations | `plug` / `link` |
| Reports | `chart-column` / `table` |
| Notifications | `bell` |
| Settings | `settings` |
| Create | `plus` |
| More | `ellipsis` |
| Environment | `variable` / `globe` |
| Specs | `file-text` |
| Mocks | `server` |
| Datasets | `database` |
| Flows | `workflow` |
| Toolsets | `wrench` |
| Documents | `file-text` |

- Phase 0 产出「设计稿图标 → Lucide 名称」完整清单，再更新 `scripts/icons.txt` 并同步资源。

### 7.1 类型化图标注册表（脚本自动生成）

- 新代码统一使用 `components/v1/icons` 包（导入路径不变，54 处旧调用点零改动）：类型化变量由脚本自动生成在 `components/v1/icons/icons_gen.go`（头部带 DO NOT EDIT，勿手改），kebab-case → PascalCase，IDE 可补全、拼错编译期报错：
  - `icons.Search`、`icons.ChevronDown`、`icons.LibraryBig`、`icons.Bell` …
  - 动态名字（配置/表格驱动）仍可用 `icons.Get(name)`。
- 新增图标的完整流程：`scripts/icons.txt` 加行 → 跑 `scripts/sync-icons.ps1|.sh`（同步资产后自动调用 `go run ./scripts/gen-icons` 重新生成）→ 无其他手工步骤。生成器按 assets 目录实际内容生成，内容无变化时跳过写入。
- **命名冲突防护**（生成器内置）：保留字表 `Get / Names / Must / All`——图标的 Pascal 名撞上包内标识符时自动加 `Icon` 后缀（如 Lucide `get` → `GetIcon`，生成行会带注释说明）；两个图标转出同名变量直接报错。Go 保留字不可能冲突（Pascal 名首字母大写）。
- `Get` 对未打包图标保持 fail fast（panic）；生成的变量在包初始化阶段就会暴露清单与资产的不一致。
- 参照 `locales/catalog_gen.go` 的模式：清单/资产是源，生成的 Go 文件是产物，二者都入库。

## 8. Token 落地方式

### 8.1 结构

建议把 `components/theme` 重构为语义 token：

```go
type Tokens struct {
    BgBase, BgApp, BgTitleBar, BgStatusBar, BgSubtle ui.Color
    BgHover, BgSelected, BgSurface                   ui.Color
    Border, BorderStrong                             ui.Color
    TextPrimary, TextSecondary, TextMuted, TextDisabled ui.Color
    Accent, AccentHover, AccentPressed, AccentSoft, OnAccent ui.Color
    Focus, FocusRing                                 ui.Color
    Danger, DangerSoft, Warning, Success, Info       ui.Color
    Overlay                                          ui.Color
}
```

- 保留 `theme.Of(c)` 入口，页面不直接读全局变量。
- Light / Dark 两套值；系统外观切换时自动跟随。
- 所有新组件只允许使用 token，不允许 `ui.Hex(...)` 直接写在页面里。
- 旧 token 字段（`BgSurface`、`HoverSurface` 等）在迁移期做别名，Phase 6 删除。

### 8.2 Metrics

建议重排 `components/theme/metrics.go`：

- 数值按新稿：StatusBar 24、Sidebar 232、TabBar 35、Row 26、Button 24/28/32、Input 28、Search 22、TitleBar 36。
- 旧代码里的 `Space3=12`、`SidebarWidth=268`、`IconRailWidth=52` 不能沿用；迁移时统一替换。
- `IconRailWidth` 在新设计里不再使用，Phase 6 删除。

### 8.3 组件包

- 新建 `components/v2`，与 `components/v1` 并存，逐步迁移。
- Phase 6 删除 `components/v1` 和旧页面组件，避免两套设计系统长期共存。
- 组件 API 按新稿重命名（例如 `Button` 增加 `Variant` / `Size`），不要为了兼容旧调用保留旧签名。

## 9. 可访问性

- 文字对比度：正文至少 4.5:1；`TextMuted` 只用于非关键信息。
- 焦点：所有可交互元素有 `Focus` 边框或 `FocusRing`；键盘 Tab 顺序与视觉顺序一致。
- 图标按钮必须有 `Label` / `Tooltip`；纯图标操作不能只有图标。
- 动效尊重 `ReduceMotion`；Phase 1 统一 Fade，不做大幅位移。
- 颜色不是唯一状态表达：方法、状态码同时有文字。

## 10. 待核对项

- [ ] 页面标题级字号（16 / 20 / 24）与字重。
- [ ] `AccentSoft`、`BgHover`、`BgSelected` 的精确值。
- [ ] GET / POST / PUT / DELETE / PATCH 的精确色值。
- [ ] 深色模式下 Focus / Danger / Success / Warning 的精确值。
- [ ] 侧栏项高度（28 还是 32）与分组间距。
- [ ] 表格行高是 26 还是 30（组件页标注存在两种口径）。
- [ ] 字体是否确实为 Inter；如果是，是否接受系统字体近似。
