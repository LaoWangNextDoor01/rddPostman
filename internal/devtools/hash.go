package devtools

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"

	"rddpostman/locales"
)

// HashSummary 汇总输入文本的常用摘要结果。
type HashSummary struct {
	MD5Lower string
	MD5Upper string
	MD516    string
	SHA1     string
	SHA256   string
	SHA512   string
}

// ComputeHashes 计算常见哈希摘要（MD5 / SHA-1 / SHA-256 / SHA-512）。
func ComputeHashes(src string) HashSummary {
	data := []byte(src)
	m := md5.Sum(data)
	md5Hex := hex.EncodeToString(m[:])

	s1 := sha1.Sum(data)
	s256 := sha256.Sum256(data)
	s512 := sha512.Sum512(data)

	return HashSummary{
		MD5Lower: md5Hex,
		MD5Upper: strings.ToUpper(md5Hex),
		MD516:    md5Hex[8:24],
		SHA1:     hex.EncodeToString(s1[:]),
		SHA256:   hex.EncodeToString(s256[:]),
		SHA512:   hex.EncodeToString(s512[:]),
	}
}

// MatchHash 将用户粘贴的期望摘要与当前计算结果进行大小写不敏感比对，返回匹配的算法名。
func MatchHash(summary HashSummary, expected string) (string, bool) {
	exp := strings.ToLower(strings.TrimSpace(expected))
	if exp == "" {
		return "", false
	}
	switch exp {
	case strings.ToLower(summary.MD5Lower):
		return locales.HashMD532(), true
	case strings.ToLower(summary.MD516):
		return locales.HashMD516(), true
	case strings.ToLower(summary.SHA1):
		return "SHA-1", true
	case strings.ToLower(summary.SHA256):
		return "SHA-256", true
	case strings.ToLower(summary.SHA512):
		return "SHA-512", true
	default:
		return "", false
	}
}

// ComputeHMAC 使用指定算法与密钥计算消息的 HMAC 签名，支持 hex / base64 输出格式。
func ComputeHMAC(algo, secret, message, format string) (string, error) {
	var fn func() hash.Hash
	switch strings.ToLower(strings.TrimSpace(algo)) {
	case "md5":
		fn = md5.New
	case "sha1", "sha-1":
		fn = sha1.New
	case "sha256", "sha-256", "":
		fn = sha256.New
	case "sha512", "sha-512":
		fn = sha512.New
	default:
		return "", fmt.Errorf("unsupported HMAC algorithm: %s", algo)
	}

	mac := hmac.New(fn, []byte(secret))
	_, _ = mac.Write([]byte(message))
	sum := mac.Sum(nil)

	if strings.EqualFold(strings.TrimSpace(format), "base64") {
		return base64.StdEncoding.EncodeToString(sum), nil
	}
	return hex.EncodeToString(sum), nil
}
