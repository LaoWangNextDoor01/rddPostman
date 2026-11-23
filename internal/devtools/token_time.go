package devtools

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"math"
	"strconv"
	"strings"
	"time"
)

// JWTDecoded 是解析后的 JWT 结构视图。
type JWTDecoded struct {
	Alg          string
	HeaderRaw    string
	PayloadRaw   string
	SignatureRaw string
	HeaderJSON   string
	PayloadJSON  string
	Signature    string
	IssuedAt     string
	IssuedAtUnix int64
	HasIat       bool
	NotBefore    string
	NotBeforeUnix int64
	HasNbf       bool
	ExpiresAt    string
	ExpiresAtUnix int64
	ExpiresRel   string
	IssuedRel    string
	HasExp       bool
	Expired      bool
}

// JWTSegmentDiag 表示 JWT 格式错误时各分段健康诊断状态。
type JWTSegmentDiag struct {
	Title  string
	OK     bool
	Badge  string
	Detail string
}

// DecodeJWT 解析 JWT（支持自动剥离 Bearer 前缀），格式化 Header/Payload 并提取时间字段。
func DecodeJWT(raw string) (*JWTDecoded, error) {
	token := stripBearer(raw)
	if token == "" {
		return nil, fmt.Errorf("请输入 JWT Token")
	}

	parts := strings.Split(token, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, fmt.Errorf("JWT 格式无效：标准 JWT 须由 '.' 分隔为 3 段，当前检测到 %d 段", len(parts))
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("解码 JWT Header 失败: %w", err)
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("解码 JWT Payload 失败: %w", err)
	}

	headerPretty, err := FormatJSON(string(headerBytes), 2, false)
	if err != nil {
		headerPretty = string(headerBytes)
	}
	payloadPretty, err := FormatJSON(string(payloadBytes), 2, false)
	if err != nil {
		payloadPretty = string(payloadBytes)
	}

	res := &JWTDecoded{
		HeaderRaw:   parts[0],
		PayloadRaw:  parts[1],
		HeaderJSON:  headerPretty,
		PayloadJSON: payloadPretty,
	}
	if len(parts) == 3 {
		res.Signature = parts[2]
		res.SignatureRaw = parts[2]
	}

	var hdr map[string]any
	if json.Unmarshal(headerBytes, &hdr) == nil {
		if alg, ok := hdr["alg"].(string); ok {
			res.Alg = alg
		}
	}

	now := time.Now()
	var claims map[string]any
	if json.Unmarshal(payloadBytes, &claims) == nil {
		if t, ok := parseJWTNumericTime(claims["iat"]); ok {
			res.HasIat = true
			res.IssuedAtUnix = t.Unix()
			res.IssuedAt = t.Local().Format("2006-01-02 15:04:05")
			res.IssuedRel = FormatRelativeDuration(t.Sub(now))
		}
		if t, ok := parseJWTNumericTime(claims["nbf"]); ok {
			res.HasNbf = true
			res.NotBeforeUnix = t.Unix()
			res.NotBefore = t.Local().Format("2006-01-02 15:04:05")
		}
		if t, ok := parseJWTNumericTime(claims["exp"]); ok {
			res.HasExp = true
			res.ExpiresAtUnix = t.Unix()
			res.ExpiresAt = t.Local().Format("2006-01-02 15:04:05")
			res.Expired = now.After(t)
			if res.Expired {
				res.ExpiresRel = "已过期 " + formatAbsDuration(now.Sub(t))
			} else {
				res.ExpiresRel = "剩余 " + formatAbsDuration(t.Sub(now))
			}
		}
	}

	return res, nil
}

// DiagnoseJWT 对损坏或格式错误的 JWT 执行三段式逐段诊断。
func DiagnoseJWT(raw string) []JWTSegmentDiag {
	token := stripBearer(raw)
	parts := strings.Split(token, ".")
	out := make([]JWTSegmentDiag, 3)

	// Segment 1: Header
	if len(parts) >= 1 && parts[0] != "" {
		if b, err := base64.RawURLEncoding.DecodeString(parts[0]); err == nil && json.Valid(b) {
			out[0] = JWTSegmentDiag{Title: "Segment 1 · HEADER", OK: true, Badge: "解析正常", Detail: string(b)}
		} else {
			out[0] = JWTSegmentDiag{Title: "Segment 1 · HEADER", OK: false, Badge: "Base64Url / JSON 损坏", Detail: "第 1 段无法解码为有效 JSON Header"}
		}
	} else {
		out[0] = JWTSegmentDiag{Title: "Segment 1 · HEADER", OK: false, Badge: "缺失", Detail: "未检测到 Header 内容"}
	}

	// Segment 2: Payload
	if len(parts) >= 2 && parts[1] != "" {
		if b, err := base64.RawURLEncoding.DecodeString(parts[1]); err == nil && json.Valid(b) {
			out[1] = JWTSegmentDiag{Title: "Segment 2 · PAYLOAD", OK: true, Badge: "解析正常", Detail: string(b)}
		} else {
			out[1] = JWTSegmentDiag{Title: "Segment 2 · PAYLOAD", OK: false, Badge: "Base64Url 损坏", Detail: "第 2 段包含非法 Base64Url 字符或非有效 JSON 载荷"}
		}
	} else {
		out[1] = JWTSegmentDiag{Title: "Segment 2 · PAYLOAD", OK: false, Badge: "缺失", Detail: "未检测到 '.' 分隔符与 Payload 载荷"}
	}

	// Segment 3: Signature
	if len(parts) == 3 && parts[2] != "" {
		out[2] = JWTSegmentDiag{Title: "Segment 3 · SIGNATURE", OK: true, Badge: "结构存在", Detail: parts[2]}
	} else if len(parts) > 3 {
		out[2] = JWTSegmentDiag{Title: "Segment 3 · SIGNATURE", OK: false, Badge: "段数超出", Detail: fmt.Sprintf("检测到 %d 段，标准 JWT 仅包含 3 段", len(parts))}
	} else {
		out[2] = JWTSegmentDiag{Title: "Segment 3 · SIGNATURE", OK: false, Badge: "缺失", Detail: "未检测到第二个 '.' 分隔符与签名内容"}
	}
	return out
}

// VerifyJWTHS 在本地校验 HS256 / HS384 / HS512 签名。
func VerifyJWTHS(rawToken, secret string, secretBase64 bool) (bool, string, error) {
	token := stripBearer(rawToken)
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[2] == "" {
		return false, "", fmt.Errorf("令牌缺少签名段，无法验签")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false, "", err
	}
	var hdr map[string]any
	if err := json.Unmarshal(headerBytes, &hdr); err != nil {
		return false, "", err
	}
	alg, _ := hdr["alg"].(string)
	var fn func() hash.Hash
	switch strings.ToUpper(alg) {
	case "HS256":
		fn = sha256.New
	case "HS384":
		fn = sha512.New384
	case "HS512":
		fn = sha512.New
	default:
		return false, alg, fmt.Errorf("本地对称验签仅支持 HS256/HS384/HS512（当前算法: %s）", alg)
	}

	keyBytes := []byte(secret)
	if secretBase64 {
		decodedKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(secret))
		if err != nil {
			decodedKey, err = base64.RawURLEncoding.DecodeString(strings.TrimSpace(secret))
			if err != nil {
				return false, alg, fmt.Errorf("密钥 Base64 解码失败: %w", err)
			}
		}
		keyBytes = decodedKey
	}

	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(fn, keyBytes)
	_, _ = mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false, alg, nil
	}
	return hmac.Equal(expectedSig, actualSig), alg, nil
}

// SignJWTHS256 生成用于示例展示的合法 HS256 JWT。
func SignJWTHS256(headerJSON, payloadJSON, secret string) string {
	h := base64.RawURLEncoding.EncodeToString([]byte(headerJSON))
	p := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(h + "." + p))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return h + "." + p + "." + sig
}

func stripBearer(raw string) string {
	token := strings.TrimSpace(raw)
	if len(token) >= 7 && strings.EqualFold(token[:7], "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	return token
}

func parseJWTNumericTime(v any) (time.Time, bool) {
	switch n := v.(type) {
	case float64:
		return time.Unix(int64(n), 0), true
	case int64:
		return time.Unix(n, 0), true
	case json.Number:
		if sec, err := n.Int64(); err == nil {
			return time.Unix(sec, 0), true
		}
	}
	return time.Time{}, false
}

// TimestampInfo 表示时间戳与日期时间的互转结果。
type TimestampInfo struct {
	Seconds      int64
	Milliseconds int64
	Local        string
	UTC          string
	RFC3339      string
	ISO8601      string
	RFC2822      string
	Relative     string
}

// TimestampToDateTime 将 Unix 秒或毫秒时间戳转为多种可读时间格式。
func TimestampToDateTime(raw string) (TimestampInfo, error) {
	return TimestampToDateTimeInZone(raw, "auto", time.Local)
}

// TimestampToDateTimeInZone 将 Unix 秒或毫秒时间戳在指定时区下转为多种可读格式。
// unit 支持 "auto" | "s" | "ms"。
func TimestampToDateTimeInZone(raw, unit string, loc *time.Location) (TimestampInfo, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return TimestampInfo{}, fmt.Errorf("请输入时间戳")
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return TimestampInfo{}, fmt.Errorf("无效的整数时间戳: %w", err)
	}
	var t time.Time
	switch unit {
	case "ms":
		t = time.UnixMilli(val)
	case "s":
		t = time.Unix(val, 0)
	default:
		if val >= 1_000_000_000_000 || val <= -1_000_000_000_000 {
			t = time.UnixMilli(val)
		} else {
			t = time.Unix(val, 0)
		}
	}
	return FromTimeInZone(t, loc), nil
}

// DateTimeToTimestamp 解析常见日期时间格式并转为 Unix 秒与毫秒。
func DateTimeToTimestamp(raw string) (TimestampInfo, error) {
	return DateTimeToTimestampInZone(raw, time.Local)
}

// DateTimeToTimestampInZone 按指定时区解析日期时间并转为 Unix 秒与毫秒。
func DateTimeToTimestampInZone(raw string, loc *time.Location) (TimestampInfo, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return TimestampInfo{}, fmt.Errorf("请输入日期时间")
	}
	if loc == nil {
		loc = time.Local
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		var (
			t   time.Time
			err error
		)
		if layout == time.RFC3339 || layout == time.RFC3339Nano {
			t, err = time.Parse(layout, s)
		} else {
			t, err = time.ParseInLocation(layout, s, loc)
		}
		if err == nil {
			return FromTimeInZone(t, loc), nil
		}
	}
	return TimestampInfo{}, fmt.Errorf("无法识别的时间格式（示例: 2026-11-23 16:00:55 或 RFC3339）")
}

// FromTime 将 time.Time 规范化为 TimestampInfo。
func FromTime(t time.Time) TimestampInfo {
	return FromTimeInZone(t, time.Local)
}

// FromTimeInZone 按目标时区格式化 TimestampInfo。
func FromTimeInZone(t time.Time, loc *time.Location) TimestampInfo {
	if loc == nil {
		loc = time.Local
	}
	inLoc := t.In(loc)
	return TimestampInfo{
		Seconds:      t.Unix(),
		Milliseconds: t.UnixMilli(),
		Local:        inLoc.Format("2006-01-02 15:04:05"),
		UTC:          t.UTC().Format("2006-01-02 15:04:05 UTC"),
		RFC3339:      t.UTC().Format(time.RFC3339),
		ISO8601:      t.UTC().Format("2006-01-02T15:04:05.000Z"),
		RFC2822:      inLoc.Format(time.RFC1123Z),
		Relative:     FormatRelativeDuration(t.Sub(time.Now())),
	}
}

// FormatRelativeDuration 将时间差格式化为中文相对时间表达（如 "刚刚"、"2 小时前"、"38 天 7 小时后"）。
func FormatRelativeDuration(d time.Duration) string {
	abs := d
	future := d > 0
	if abs < 0 {
		abs = -abs
	}
	if abs < 5*time.Second {
		return "刚刚 (0 秒前)"
	}
	suffix := "前"
	if future {
		suffix = "后"
	}
	return formatAbsDuration(abs) + suffix
}

func formatAbsDuration(abs time.Duration) string {
	if abs < time.Minute {
		return fmt.Sprintf("%d 秒", int(abs.Seconds()))
	}
	if abs < time.Hour {
		m := int(abs.Minutes())
		s := int(abs.Seconds()) % 60
		if s == 0 {
			return fmt.Sprintf("%d 分钟", m)
		}
		return fmt.Sprintf("%d 分 %d 秒", m, s)
	}
	if abs < 24*time.Hour {
		h := int(abs.Hours())
		m := int(abs.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%d 小时", h)
		}
		return fmt.Sprintf("%d 小时 %d 分钟", h, m)
	}
	days := int(abs.Hours()) / 24
	hours := int(abs.Hours()) % 24
	if hours == 0 {
		return fmt.Sprintf("%d 天", days)
	}
	return fmt.Sprintf("%d 天 %d 小时", days, hours)
}

// NewUUIDv4 生成标准 RFC 4122 UUID v4。
func NewUUIDv4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// NewUUIDv7 生成 RFC 9562 时序有序 UUID v7（48-bit 毫秒时间戳 + 74-bit 随机数）。
func NewUUIDv7() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	ms := uint64(time.Now().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	b[6] = (b[6] & 0x0f) | 0x70
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// NewULID 生成 26 位 Crockford Base32 ULID（48-bit 时间戳 + 80-bit 随机熵）。
func NewULID() string {
	const enc = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	var raw [16]byte
	ms := uint64(time.Now().UnixMilli())
	raw[0] = byte(ms >> 40)
	raw[1] = byte(ms >> 32)
	raw[2] = byte(ms >> 24)
	raw[3] = byte(ms >> 16)
	raw[4] = byte(ms >> 8)
	raw[5] = byte(ms)
	_, _ = rand.Read(raw[6:])

	var out [26]byte
	// 48-bit timestamp -> 10 chars
	t := binary.BigEndian.Uint64([]byte{0, 0, raw[0], raw[1], raw[2], raw[3], raw[4], raw[5]})
	for i := 9; i >= 0; i-- {
		out[i] = enc[t&31]
		t >>= 5
	}
	// 80-bit randomness -> 16 chars (2 x 40-bit chunks)
	hi := (uint64(raw[6]) << 32) | (uint64(raw[7]) << 24) | (uint64(raw[8]) << 16) | (uint64(raw[9]) << 8) | uint64(raw[10])
	lo := (uint64(raw[11]) << 32) | (uint64(raw[12]) << 24) | (uint64(raw[13]) << 16) | (uint64(raw[14]) << 8) | uint64(raw[15])
	for i := 17; i >= 10; i-- {
		out[i] = enc[hi&31]
		hi >>= 5
	}
	for i := 25; i >= 18; i-- {
		out[i] = enc[lo&31]
		lo >>= 5
	}
	return string(out[:])
}

// FormatUUID 根据大写与无连字符选项格式化 UUID 字符串。
func FormatUUID(u string, upper, noHyphens bool) string {
	if noHyphens {
		u = strings.ReplaceAll(u, "-", "")
	}
	if upper {
		u = strings.ToUpper(u)
	} else {
		u = strings.ToLower(u)
	}
	return u
}

// RandomStringOpts 配置自定义高熵随机字符串生成器。
type RandomStringOpts struct {
	Length           int
	Upper            bool
	Lower            bool
	Digits           bool
	Symbols          bool
	ExcludeAmbiguous bool
}

// BuildAlphabet 根据选项拼装字符集。
func BuildAlphabet(opts RandomStringOpts) string {
	var sb strings.Builder
	if opts.Upper {
		sb.WriteString("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	}
	if opts.Lower {
		sb.WriteString("abcdefghijklmnopqrstuvwxyz")
	}
	if opts.Digits {
		sb.WriteString("0123456789")
	}
	if opts.Symbols {
		sb.WriteString("!@#$%^&*_-+=?")
	}
	alpha := sb.String()
	if alpha == "" {
		alpha = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	}
	if opts.ExcludeAmbiguous {
		var clean strings.Builder
		for _, r := range alpha {
			if !strings.ContainsRune("0O1lI", r) {
				clean.WriteRune(r)
			}
		}
		if clean.Len() > 0 {
			alpha = clean.String()
		}
	}
	return alpha
}

// CalcEntropyBits 计算随机字符串密码学熵值（bits = length * log2(|alphabet|)）。
func CalcEntropyBits(opts RandomStringOpts) float64 {
	n := opts.Length
	if n <= 0 {
		n = 32
	}
	alpha := BuildAlphabet(opts)
	if len(alpha) <= 1 {
		return 0
	}
	return float64(n) * math.Log2(float64(len(alpha)))
}

// GenerateRandomString 使用 crypto/rand 拒绝采样生成无偏差高熵随机字符串。
func GenerateRandomString(opts RandomStringOpts) string {
	n := opts.Length
	if n <= 0 {
		n = 32
	}
	alpha := BuildAlphabet(opts)
	m := len(alpha)
	maxByte := 256 - (256 % m)

	out := make([]byte, 0, n)
	buf := make([]byte, n*2)
	for len(out) < n {
		_, _ = rand.Read(buf)
		for _, b := range buf {
			if int(b) < maxByte {
				out = append(out, alpha[int(b)%m])
				if len(out) == n {
					break
				}
			}
		}
	}
	return string(out)
}

// RandomString 生成指定长度的安全随机字符串（hexOnly 为 true 时仅含十六进制字符）。
func RandomString(length int, hexOnly bool) string {
	if length <= 0 {
		length = 32
	}
	if hexOnly {
		buf := make([]byte, (length+1)/2)
		_, _ = rand.Read(buf)
		return hex.EncodeToString(buf)[:length]
	}
	return GenerateRandomString(RandomStringOpts{
		Length: length,
		Upper:  true,
		Lower:  true,
		Digits: true,
	})
}
