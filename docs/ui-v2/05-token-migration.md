# 05 · Theme / Token 迁移方案

> 状态：**`components/theme/v2` 已落地（2026-11-19）**——Tokens（含方法色）+ Metrics + Light/Dark 均按 02 号文档实现，含单测；v1 冻结未动。下一步是 `components/v2` 原子组件。
> 目标：token 可以整体重构，但迁移期间旧页面仍要能看、能用。采用「v1 冻结、v2 新建、逐页切换、最后删除 v1」的方式，不直接在旧 token 上改值。

## 1. 迁移策略

```
Phase 1        旧页面 ── components/theme (v1) ── 保持不变
               新壳层 ── components/theme (v1) ── 先复用旧值，保证可运行

Phase 2        旧页面 ── components/theme (v1)
               新页面 ── components/theme/v2 + components/v2 ── 新 token

Phase 3–5      逐页从 v1 迁到 v2
Phase 6        删除 v1，所有页面只依赖 v2
```

原因：

- 现有 `Space3=12`、`SidebarWidth=268`、`TitleBarHeight=40` 等值被大量旧页面引用。直接改值会让未迁移页面同时错位，难以判断是新组件问题还是旧页面问题。
- v2 token 使用新的语义命名（`BgApp`、`BgHover`、`BgSelected`、`BorderStrong`），与旧字段不是一一对应，强行别名会留下歧义。
- 双轨期只允许新页面使用 v2；旧页面不新增 v1 用法。

## 2. 颜色映射

| 旧 token | 旧值（Light） | 新 token | 新值（Light） | 处理 |
| --- | --- | --- | --- | --- |
| `BgBase` | `#F7F7F8` | `BgBase` | `#FFFFFF` | 语义变化：旧的是应用底色，新的是主内容底 |
| `BgSurface` | `#FFFFFF` | `BgSurface` | `#FFFFFF` | 保留 |
| `BgElevated` | `#F2F2F4` | `BgSubtle` | `#F2F2F2` | 改名 |
| `HoverSurface` | `#F2F2F4` | `BgHover` | `#EDEDED` | 改名 + 改值 |
| `PopoverBg` | `#FFFFFF` | `BgSurface` | `#FFFFFF` | 合并 |
| `PopoverItemHover` | `#F2F2F4` | `BgHover` | `#EDEDED` | 合并 |
| `Border` | `#E4E4E7` | `Border` | `#E6E6E6` | 改值 |
| — | — | `BorderStrong` | `#A6A6A6` | 新增 |
| `TextPrimary` | `#1F1F23` | `TextPrimary` | `#212121` | 改值 |
| `TextSecondary` | `#52525B` | `TextSecondary` | `#6B6B6B` | 改值 |
| `TextMuted` | `#9A9AA4` | `TextMuted` | `#909090` | 改值 |
| `TextDisabled` | `#B4B4BC` | `TextDisabled` | `#B5B5B5` | 微调 |
| `Accent` | `#FF6C37` | `Accent` | `#FF6C37` | 保留 |
| `AccentSoft` | `#FFF1EB` | `AccentSoft` | `#FFF1EB` | 保留，待核对 |
| `OnAccent` | `#FFFFFF` | `OnAccent` | `#FFFFFF` | 保留 |
| `FocusRing` | `#FF6C37` | `Focus` + `FocusRing` | `#097BED` + `#E7F0FF` | 拆成边框色和外环色 |
| `Danger` | `#D9483B` | `Danger` | `#EB2013` | 改值 |
| — | — | `DangerSoft` | `#FFEBE7` | 新增 |
| `Warning` | `#C77700` | `Warning` | `#C77700` | 待核对 |
| `Success` | `#1F9D6B` | `Success` | `#1F9D6B` | 待核对 |
| `Info` | `#2F6FEB` | `Info` | `#2F6FEB` | 待核对 |
| `Overlay` | `#0F172A66` | `Overlay` | `#0F172A66` | 保留 |
| — | — | `BgApp` | `#F9F9F9` | 新增：工作台侧栏/标签条 |
| — | — | `BgTitleBar` | `#FFFFFF` | 新增；2026-11-19 复测取样后由 #F2F2F2 更正（旧值命中了搜索框底色） |
| — | — | `BgStatusBar` | `#FFFFFF` | 新增 |
| — | — | `BgSelected` | `#E6E6E6` | 新增 |
| — | — | `AccentHover` | `#ED5B2A` | 新增 |
| — | — | `AccentPressed` | `#D03E0A` | 新增 |

Dark 侧同理，新值见 `02-design-system.md`。

## 3. 尺寸映射

| 旧 Metric | 旧值 | 新 Metric | 新值 | 影响 |
| --- | --- | --- | --- | --- |
| `Space1..Space8` | 4 / 8 / 12 / 16 / 20 / 24 / 32 | `Space0..Space8`（Tailwind 步进：SpaceN = N×4，半步 05/15） | 同名同值 + 新增 2 / 6 / 20 档 | 旧页面可直接迁移，无需换算 |
| `RadiusSM..XL` | 4 / 6 / 8 / 10 / 999 | `RadiusSM..Full` | 4 / 6 / 8 / 999 | 删除 10 |
| `FontXS..XXL` | 10 / 12 / 14 / 16 / 18 / 20 | `FontMicro..XXL` | 10 / 11 / 12 / 14 / 16 / 20 / 24 | 新增 11，删除 18 |
| `LineXS/SM/MD` | 15 / 16 / 18 | `Line*` | 按 14 / 16 / 18 / 22 / 24 等重排 | 旧页面受影响 |
| `Border` | 1 | `Border` | 1 | 保留 |
| `IconXS..LG` | 12 / 14 / 16 / 18 | 同名 | 12 / 14 / 16 / 18 | 保留 |
| `SizeXS..XL` | 20 / 22 / 24 / 28 / 34 | `SizeXS..XL` + `Control*` | Size 16 / 20 / 24 / 28 / 32（4 的倍数阶梯）；控件高度另拆 Button 24/28/32、Input 28、Search 22、Select 32 | 通用方形尺寸保留同名，值微调；按钮/输入用 Control 语义字段 |
| `TitleBarHeight` | 40 | `TitleBarHeight` | 36 | 实测对齐 Postman（36 = 内容 35 + 分割线 1）；Windows/Linux 显式设置，macOS 跟随原生，36 兼作兜底 |
| `IconRailWidth` | 52 | — | 删除 | 新设计无图标栏 |
| `SidebarWidth` | 268 | `SidebarWidth` | 232 | 全部侧栏 |
| `StatusBarHeight` | 26 | `StatusBarHeight` | 24 | 状态栏 |
| `PagePaddingX/Y` | 24 / 20 | `PagePaddingX/Y` | 20 / 20（建议） | 页面留白 |
| `CardHeight` | 96 | `CardHeight` | 待定 | 新卡片按内容自适应 |
| `TagWidth/Height` | 50 / 23 | `MethodTag*` | 按方法标签 38 宽 / 18 高（建议） | 方法标签 |

## 4. 组件映射（v1 → v2）

| v1 | v2 | 备注 |
| --- | --- | --- |
| `v1.Button` / `PrimaryButton` | `v2.Button`（Variant + Size） | 支持 primary / secondary / ghost / danger / split |
| `v1.MiniButton` | `v2.IconButton` | 24 / 28 / 32 |
| `v1.Tag` | `v2.MethodTag` / `v2.Badge` | 方法和通用徽标分开 |
| `v1.Card` | `v2.Card` / `v2.Panel` | 圆角 6–8，边框 1px |
| `v1.PageHeader` | `v2.PageHeader` | 字号按新字阶 |
| `v1.EmptyState` | `v2.EmptyState` | 统一空态结构 |
| `theme.PopoverPanel` | `v2.Popover` | 菜单/弹层统一 |
| `theme.DialogPanel` | `v2.Dialog` | Settings / 确认框 |
| 现有 `Split` 用法 | `v2.SplitPane` | 侧栏宽度 232，可拖拽 |
| 现有 tabs | `v2.TabBar` / `v2.TabChip` | 统一工作台标签 |
| 现有树 | `v2.Tree` | 行高 26，选中 `#EDEDED` |

## 5. 迁移顺序

1. ~~新建 `components/theme/v2`：Tokens、Metrics、Light/Dark。~~（已完成，含单测）
2. 新建 `components/v2`：先 Button、Input、IconButton、Tag、EmptyState。（已起步：`IconButton` 于 2026-11-19 落地，含 hover/pressed/tooltip/无障碍名；其余待补）
3. 重绘 AppShell（TitleBar、StatusBar）与 HubLayout；这部分只用 v2。
4. 重绘 Workspaces 列表 / Create 页。
5. Phase 3 重绘 Workbench 侧栏、TabBar、Tree。
6. Phase 4 重绘请求详情与响应区。
7. 每迁完一页，从该页删除 v1 引用；Phase 6 全局检查无 v1 引用后删除旧包。

## 6. 验收

- [ ] `components/theme`（v1）在迁移期不被修改，避免旧页面视觉漂移。
- [ ] 新页面 0 处 `ui.Hex`、0 处直接数字颜色。
- [ ] 同一页面不会同时引用 v1 和 v2 组件。
- [ ] Light/Dark 两套 token 都有对应截图。
- [ ] Phase 6 删除 v1 后 `go build ./...` 通过，无残留 import。
- [ ] `grep` 检查旧 token 字段（`BgElevated`、`HoverSurface`、`IconRailWidth`）无引用。
