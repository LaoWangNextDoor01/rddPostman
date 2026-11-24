// Package tools 实现工作台开发者小工具侧栏与多 Tab 工具编辑视图。
package tools

import (
	"rddpostman/components/v1/icons"
	"rddpostman/locales"

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
	CategoryKey string
	Category    string
	Name        string
	ShortName   string
	Description string
	Keywords    string
	Icon        *ui.SVG
}

// All 返回全部已注册的小工具定义列表（按当前语言动态生成）。
func All() []ToolDef {
	catData := locales.ToolsCatData()
	catSecurity := locales.ToolsCatSecurity()
	catTimeID := locales.ToolsCatTimeID()
	return []ToolDef{
		{
			ID:          ToolJSON,
			CategoryKey: "data",
			Category:    catData,
			Name:        locales.ToolJSONName(),
			ShortName:   locales.ToolJSONShort(),
			Description: locales.ToolJSONDesc(),
			Keywords:    "json format pretty minify escape unescape sort 格式化 压缩 转义 排序",
			Icon:        icons.Code,
		},
		{
			ID:          ToolCodec,
			CategoryKey: "data",
			Category:    catData,
			Name:        locales.ToolCodecName(),
			ShortName:   locales.ToolCodecName(),
			Description: locales.ToolCodecDesc(),
			Keywords:    "base64 url unicode encode decode urlsafe query escape unescape 编码 解码 转义",
			Icon:        icons.Link,
		},
		{
			ID:          ToolHash,
			CategoryKey: "security",
			Category:    catSecurity,
			Name:        locales.ToolHashName(),
			ShortName:   locales.ToolHashShort(),
			Description: locales.ToolHashDesc(),
			Keywords:    "md5 sha1 sha256 sha512 hmac hash digest sign compare 哈希 摘要 签名 比对",
			Icon:        icons.Lock,
		},
		{
			ID:          ToolJWT,
			CategoryKey: "security",
			Category:    catSecurity,
			Name:        locales.ToolJWTName(),
			ShortName:   locales.ToolJWTName(),
			Description: locales.ToolJWTDesc(),
			Keywords:    "jwt token bearer decode header payload signature verify exp 令牌 解析 验签",
			Icon:        icons.Eye,
		},
		{
			ID:          ToolTimestamp,
			CategoryKey: "time_id",
			Category:    catTimeID,
			Name:        locales.ToolTimestampName(),
			ShortName:   locales.ToolTimestampName(),
			Description: locales.ToolTimestampDesc(),
			Keywords:    "timestamp unix time date epoch timezone iso8601 rfc2822 时间戳 日期 时区",
			Icon:        icons.Clock,
		},
		{
			ID:          ToolUUID,
			CategoryKey: "time_id",
			Category:    catTimeID,
			Name:        locales.ToolUUIDName(),
			ShortName:   locales.ToolUUIDName(),
			Description: locales.ToolUUIDDesc(),
			Keywords:    "uuid uuidv7 ulid guid random secret key generator csprng 随机 密钥",
			Icon:        icons.Sparkles,
		},
	}
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
	for _, t := range All() {
		if t.ID == cid {
			cp := t
			return &cp
		}
	}
	return nil
}

// LabelOf 返回小工具用于 Tab 显示的短名称。
func LabelOf(id string) string {
	switch id {
	case ToolURL:
		return locales.ToolAliasURL()
	case ToolUnicode:
		return locales.ToolAliasUnicode()
	case ToolBase64:
		return locales.ToolAliasBase64()
	}
	if def := Find(id); def != nil {
		return def.ShortName
	}
	return locales.ToolDefaultLabel()
}
