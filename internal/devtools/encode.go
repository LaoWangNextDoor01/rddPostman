package devtools

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"rddpostman/locales"
)

// Base64URLSafeDiff 统计标准 Base64 转为 URL-Safe / 省略填充时的替换数量。
type Base64URLSafeDiff struct {
	PlusReplaced  int
	SlashReplaced int
	PadOmitted    int
}

// QueryParamPreview 表示 URL Query 参数自动解析预览的一行。
type QueryParamPreview struct {
	Key          string
	RawValue     string
	EncodedValue string
}

// Base64Encode 将原文编码为 Base64（支持标准与 URL-Safe 变体）。
func Base64Encode(src string, urlSafe bool) string {
	return Base64EncodeOpts(src, urlSafe, false)
}

// Base64EncodeOpts 将原文编码为 Base64，支持 URL-Safe 与省略 '=' 填充。
func Base64EncodeOpts(src string, urlSafe, omitPadding bool) string {
	if src == "" {
		return ""
	}
	switch {
	case urlSafe && omitPadding:
		return base64.RawURLEncoding.EncodeToString([]byte(src))
	case urlSafe:
		return base64.URLEncoding.EncodeToString([]byte(src))
	case omitPadding:
		return base64.RawStdEncoding.EncodeToString([]byte(src))
	default:
		return base64.StdEncoding.EncodeToString([]byte(src))
	}
}

// Base64DiffStats 统计将原文编码为 URL-Safe / 无填充时的字符替换情况。
func Base64DiffStats(src string, urlSafe, omitPadding bool) Base64URLSafeDiff {
	if src == "" {
		return Base64URLSafeDiff{}
	}
	std := base64.StdEncoding.EncodeToString([]byte(src))
	var d Base64URLSafeDiff
	if urlSafe {
		d.PlusReplaced = strings.Count(std, "+")
		d.SlashReplaced = strings.Count(std, "/")
	}
	if omitPadding {
		d.PadOmitted = strings.Count(std, "=")
	}
	return d
}

// Base64Decode 将 Base64 解码为原文（自动兼容标准/URL-Safe 及缺失 = 填充）。
func Base64Decode(src string, urlSafe bool) (string, error) {
	cleaned := strings.TrimSpace(src)
	if cleaned == "" {
		return "", nil
	}
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	if urlSafe {
		encodings = []*base64.Encoding{
			base64.URLEncoding,
			base64.RawURLEncoding,
			base64.StdEncoding,
			base64.RawStdEncoding,
		}
	}
	var lastErr error
	for _, enc := range encodings {
		if b, err := enc.DecodeString(cleaned); err == nil {
			return string(b), nil
		} else {
			lastErr = err
		}
	}
	if pos, ch := findInvalidBase64Char(cleaned); pos > 0 {
		return "", errors.New(locales.DtErrBase64InvalidChar(strconv.QuoteRune(ch), strconv.Itoa(pos)))
	}
	return "", errors.New(locales.DtErrBase64Invalid(lastErr.Error()))
}

func findInvalidBase64Char(s string) (int, rune) {
	for i, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			r == '+' || r == '/' || r == '-' || r == '_' || r == '=' {
			continue
		}
		return i + 1, r
	}
	return 0, 0
}

// CleanBase64 移除空白与所有非 Base64 字符，供解码出错时一键自动清理。
func CleanBase64(src string) string {
	var b strings.Builder
	for _, r := range src {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			r == '+' || r == '/' || r == '-' || r == '_' || r == '=' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// URLEncode 对字符串执行 URL Query 编码。
func URLEncode(src string) string {
	if src == "" {
		return ""
	}
	return url.QueryEscape(src)
}

// URLEncodeMode 支持 Query 参数编码（空格转 + / 全转义）与完整 URI 编码（保留协议/路径分隔符）。
func URLEncodeMode(src string, fullURI bool) string {
	if src == "" {
		return ""
	}
	if !fullURI {
		return url.QueryEscape(src)
	}
	u, err := url.Parse(src)
	if err != nil {
		return url.PathEscape(src)
	}
	if u.RawQuery != "" {
		// 逐项编码 query 参数值
		pairs := strings.Split(u.RawQuery, "&")
		for i, p := range pairs {
			if k, v, ok := strings.Cut(p, "="); ok {
				pairs[i] = url.QueryEscape(k) + "=" + url.QueryEscape(v)
			} else {
				pairs[i] = url.QueryEscape(p)
			}
		}
		u.RawQuery = strings.Join(pairs, "&")
	}
	return u.String()
}

// URLDecode 对 URL 编码字符串执行解码。
func URLDecode(src string) (string, error) {
	if src == "" {
		return "", nil
	}
	out, err := url.QueryUnescape(src)
	if err != nil {
		return "", errors.New(locales.DtErrURLDecode(err.Error()))
	}
	return out, nil
}

// ParseQueryPreview 从完整 URL 或裸 Query 串中解析键值对供预览表展示。
func ParseQueryPreview(raw string) []QueryParamPreview {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	qStr := s
	if idx := strings.Index(s, "?"); idx >= 0 {
		qStr = s[idx+1:]
	} else if !strings.Contains(s, "=") {
		return nil
	}
	if hashIdx := strings.Index(qStr, "#"); hashIdx >= 0 {
		qStr = qStr[:hashIdx]
	}
	parts := strings.Split(qStr, "&")
	out := make([]QueryParamPreview, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		k, v, hasEq := strings.Cut(p, "=")
		if !hasEq && k == "" {
			continue
		}
		decK, err := url.QueryUnescape(k)
		if err != nil {
			decK = k
		}
		decV, err := url.QueryUnescape(v)
		if err != nil {
			decV = v
		}
		out = append(out, QueryParamPreview{
			Key:          decK,
			RawValue:     decV,
			EncodedValue: url.QueryEscape(decV),
		})
	}
	return out
}

// UnicodeEscape 将非 ASCII 字符转为 \uXXXX（或 \UXXXXXXXX）形式。
func UnicodeEscape(src string) string {
	return UnicodeEscapeOpts(src, true)
}

// UnicodeEscapeOpts 支持仅转义非 ASCII 或全部字符转义。
func UnicodeEscapeOpts(src string, nonASCIIOnly bool) string {
	if src == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range src {
		if nonASCIIOnly && r < 128 {
			b.WriteRune(r)
		} else if r <= 0xFFFF {
			fmt.Fprintf(&b, `\u%04x`, r)
		} else {
			fmt.Fprintf(&b, `\U%08x`, r)
		}
	}
	return b.String()
}

// UnicodeUnescape 将包含 \uXXXX 转义序列的文本还原为可读 UTF-8 字符串。
func UnicodeUnescape(src string) (string, error) {
	if src == "" {
		return "", nil
	}
	escaped := strings.ReplaceAll(src, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\r", `\r`)
	escaped = strings.ReplaceAll(escaped, "\n", `\n`)
	out, err := strconv.Unquote(`"` + escaped + `"`)
	if err != nil {
		return "", errors.New(locales.DtErrUnicodeUnescape(err.Error()))
	}
	return out, nil
}
