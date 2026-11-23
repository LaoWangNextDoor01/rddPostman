# 06 · 开发者小工具（Dev Tools）UI 设计规范

> 严格遵循 [02-design-system.md](./02-design-system.md) 与 `components/theme/v2/tokens.go` 语义 Token 体系：
> **12px 桌面紧凑密度 · 科技蓝 `#336FFF` 强调 · 1px 细分割线 · Light / Dark 双主题同步**。
> 可视化设计稿见同目录 [dev-tools-design.html](./dev-tools-design.html)（浏览器直接打开可 1:1 审阅全部画板与交互态）。

## 1. 整体信息架构与入口

- **工作台左侧 Sidebar 顶部导航栏**：由 3 个图标扩展为 **4 个图标**：
  1. `Collections`（`icons.Layers`）— `Ctrl/⌘ + 1`
  2. `Environments`（`icons.Globe`）— `Ctrl/⌘ + 2`
  3. `History`（`icons.Clock`）— `Ctrl/⌘ + 3`
  4. **`Tools`（`icons.Code`）— `Ctrl/⌘ + 4`**（新增；底部 StatusBar 右下角 `Tools` 也可一键直达）
- **左侧面板（`240px` 宽，底色 `BgApp`：`#F9F9F9` / Dark `#191919`）**：
  - 顶部标题 `DEV TOOLS`（`11px SemiBold`，`TextMuted`）+ 实时搜索过滤框（`28px` 高，`BgSubtle` 底色，聚焦 `Focus` 蓝框 + 3px 外环）。
  - 按功能分为 **4 个分组**，每项行高 `30px`、左右内边距 `8px`、圆角 `6px`：
    - **JSON**：`JSON 格式化 / 转义`
    - **编码与解码**：`Base64 编解码`、`URL 编解码`、`Unicode 转义`
    - **哈希与签名**：`MD5 / SHA / HMAC`
    - **令牌与时间**：`JWT 解析器`、`时间戳转换`、`UUID / 随机串生成`
  - **选中态**：背景 `AccentSoft`（Light `#EBF1FF` / Dark `#1D2842`），图标 `Accent`（`#336FFF`），文字 `TextPrimary` Medium；Hover 态背景 `BgHover`。
- **右侧多标签区域（复用现有 Workbench Tabs）**：
  - 点击任意工具在右侧打开独立 Tool Tab（`tool:{id}`），标签带 `</>` 图标与短标题，**与 HTTP 请求 Tab、环境 Tab 并存**。
  - 切回 `Collections` 查看接口时，已打开的工具 Tab 保持不丢失；`Ctrl/⌘ + K` 全局搜索支持直接检索并打开任意小工具。

## 2. 面板内部统一布局规范

所有工具页共享统一的轻量骨架，避免浪费垂直空间：

| 区域 | 尺寸与样式 | 说明 |
| --- | --- | --- |
| **顶部工具标题栏** | 高度 `48px`，底部分割线 `1px Border`，`padding: 0 16px` | 左侧：`28×28` 图标徽章（`AccentSoft` 底 + `Accent` 图标）+ 标题（`14px SemiBold`）+ 单行副标题（`11px TextMuted`）；右侧：快捷操作（`示例` / `清空` / `复制`） |
| **功能工具条** | 高度 `38px`，`BgApp` 或 `BgBase` + 底分割线，`padding: 0 16px`，控件统一 `26px–28px` 高 | 主操作按钮用 `Accent #336FFF`，模式切换用 `BgSubtle` Segmented Control，次操作用 Outline 按钮 |
| **主体工作区** | `BgBase`（Light `#FFFFFF` / Dark `#1E1E1E`），支持滚动或双栏 `flex: 1` 自适应 | 见下方 4 类模板 |
| **底部轻状态条（编辑器类）** | 高度 `26px`，`11px` 等宽数字，左校验状态（`Success` / `Danger`）、右字符/行数统计 | 仅双栏文本/JSON 类工具展示 |

## 3. 四类工具界面模板

### 3.1 文本转换双栏模板（JSON 格式化/转义、Base64、URL、Unicode）
- **JSON 格式化 / 转义**：
  - 工具条：`[格式化]`（主蓝） + `[2空格 | 4空格]` 分段器 + `[压缩]` + `[Key 排序]` | 分割线 | `[转义]` + `[去转义]`；右侧 `[互换 ⇄]` `[复制]`。
  - 下方 50/50 左右双栏代码编辑器（带行号、JSON 语法高亮、各自标题条 `INPUT` / `OUTPUT` 与字节数/校验徽标）。
- **Base64 / URL / Unicode**：
  - 工具条：编码模式分段器（如 Base64 的 `标准 | URL-Safe`，URL 的 `Query | Path`）+ `[编码 →]`（主蓝） + `[← 解码]` + `[互换 ⇄]` + `[复制]`。
  - 下方 50/50 左右双栏等宽文本框，非法输入在底部红字内联报错（`DangerSoft` 底 + `Danger` 字），不弹窗打断。

### 3.2 实时哈希与签名模板（MD5 / SHA / HMAC）
- 上半区：明文输入框（高度 `84px`，实时触发计算，右上角显示 `UTF-8 · N bytes` 与 `清空`）。
- 中间区：摘要结果表卡片（`1px Border`，圆角 `6px`，每行 `34px` 高）：依次列出 `MD5 (32 小写)`、`MD5 (32 大写)`、`MD5 (16 位)`、`SHA-1`、`SHA-256`、`SHA-512`，等宽字体显示，行尾独立 `Copy`（点击后变为绿色 `Copied` 1.5s）。
- 下半区：HMAC 签名条（Secret Key 输入框 + 算法分段器 `MD5 | SHA1 | SHA256 | SHA512` + 输出格式 `HEX | BASE64` + 实时签名结果行与 `Copy`）。

### 3.3 结构化解析模板（JWT 解析器）
- 上半区：JWT Token 输入框 + **三段着色预览条**（Header 蓝 / Payload 紫 / Signature 绿）+ 右侧状态徽标（`未过期 · 剩余 X` / `已过期` / `仅解码未验签`）。
- 下半区：左右 50/50 双栏：
  - 左栏：`HEADER`（JSON 高亮）+ `TIME CLAIMS` 卡片（自动将 `iat` / `nbf` / `exp` 转为本地可读时间 + 相对时间）+ `SIGNATURE` 摘要。
  - 右栏：`PAYLOAD`（完整 JSON 语法高亮 + 一键复制）。

### 3.4 仪表盘卡片模板（时间戳转换、UUID / 随机串生成）
- **时间戳转换**：顶部当前时间条（大号等宽秒/毫秒时间戳 + `暂停/继续` + 一键复制）+ 下方左右双卡片（左：`时间戳 → 日期`；右：`日期 → 时间戳`，支持秒/毫秒与 Local/UTC 切换）。
- **UUID / 随机串生成**：左右或上下双卡片（UUID v4 批量生成：数量 1/5/10/20 + 大写/无连字符开关；随机字符串：长度 + 字符集勾选 + 实时生成列表与批量复制）。
