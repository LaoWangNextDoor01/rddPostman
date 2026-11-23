// Package tools 实现工作台开发者小工具侧栏与多 Tab 工具编辑视图。
package tools

import (
	"rddpostman/components/v1/icons"

	"github.com/egoist/mygo/ui"
)

// ToolID 标识单个小工具的唯一键。
const (
	ToolJSON      = "json"
	ToolCodec     = "codec"
	ToolHash      = "hash"
	ToolJWT       = "jwt"
	ToolTimestamp = "timestamp"
	ToolUUID      = "uuid"

	// 兼容旧版独立编解码入口 ID（自动路由至编解码工作台对应子模式）。
	ToolBase64  = "base64"
	ToolURL     = "url"
	ToolUnicode = "unicode"
)

// ToolDef 描述一个小工具的元信息，供左侧工具栏与全局 Ctrl/⌘+K 搜索统一消费。
type ToolDef struct {
	ID          string
	Category    string
	Name        string
	ShortName   string
	Description string
	Keywords    string
	Icon        *ui.SVG
}

var registry = []ToolDef{
	{
		ID:          ToolJSON,
		Category:    "数据处理",
		Name:        "JSON 格式化 / 转义",
		ShortName:   "JSON 格式化",
		Description: "校验语法 · 美化缩进 · 压缩排序 · 字符串转义",
		Keywords:    "json format pretty minify escape unescape sort 格式化 压缩 转义 排序",
		Icon:        icons.Code,
	},
	{
		ID:          ToolCodec,
		Category:    "数据处理",
		Name:        "编解码工作台",
		ShortName:   "编解码工作台",
		Description: "Base64 (Standard / URL-Safe) · URL Query/Path · Unicode 转义",
		Keywords:    "base64 url unicode encode decode urlsafe query escape unescape 编码 解码 转义",
		Icon:        icons.Link,
	},
	{
		ID:          ToolHash,
		Category:    "安全与鉴权",
		Name:        "哈希与 HMAC 签名",
		ShortName:   "哈希与签名",
		Description: "MD5 · SHA-1 / 256 / 512 实时摘要 · HMAC 签名 · 校验比对",
		Keywords:    "md5 sha1 sha256 sha512 hmac hash digest sign compare 哈希 摘要 签名 比对",
		Icon:        icons.Lock,
	},
	{
		ID:          ToolJWT,
		Category:    "安全与鉴权",
		Name:        "JWT 解析器",
		ShortName:   "JWT 解析器",
		Description: "自动剥离 Bearer 前缀 · 三段着色 · Claims 时间换算 · 本地验签",
		Keywords:    "jwt token bearer decode header payload signature verify exp 令牌 解析 验签",
		Icon:        icons.Eye,
	},
	{
		ID:          ToolTimestamp,
		Category:    "时间与标识",
		Name:        "时间戳转换",
		ShortName:   "时间戳转换",
		Description: "实时秒/毫秒时钟 · 双向转换 · 多时区与 ISO 8601 快捷格式",
		Keywords:    "timestamp unix time date epoch timezone iso8601 rfc2822 时间戳 日期 时区",
		Icon:        icons.Clock,
	},
	{
		ID:          ToolUUID,
		Category:    "时间与标识",
		Name:        "UUID / 随机串",
		ShortName:   "UUID / 随机串",
		Description: "UUID v4 / v7 · ULID · 自定义密码学安全随机串批量生成",
		Keywords:    "uuid uuidv7 ulid guid random secret key generator csprng 随机 密钥",
		Icon:        icons.Sparkles,
	},
}

// All 返回全部已注册的小工具定义列表。
func All() []ToolDef {
	out := make([]ToolDef, len(registry))
	copy(out, registry)
	return out
}

// CanonicalID 将旧版独立编解码工具 ID 归一化为统一工具 ID。
func CanonicalID(id string) string {
	switch id {
	case ToolBase64, ToolURL, ToolUnicode:
		return ToolCodec
	default:
		return id
	}
}

// Find 按 ID 查找小工具定义（自动兼容 base64/url/unicode 别名）。
func Find(id string) *ToolDef {
	cid := CanonicalID(id)
	for i := range registry {
		if registry[i].ID == cid {
			t := registry[i]
			return &t
		}
	}
	return nil
}

// LabelOf 返回小工具用于 Tab 显示的短名称。
func LabelOf(id string) string {
	switch id {
	case ToolURL:
		return "URL 编解码"
	case ToolUnicode:
		return "Unicode 转义"
	case ToolBase64:
		return "Base64 编解码"
	}
	if def := Find(id); def != nil {
		return def.ShortName
	}
	return "Tool"
}
