package devtools

import (
	"strings"
	"testing"
	"time"
)

func TestJSONTools(t *testing.T) {
	raw := `{"b":1,"a":{"d":true,"c":"hi"}}`

	pretty, err := FormatJSON(raw, 2, false)
	if err != nil || !strings.Contains(pretty, "\n  \"b\": 1") {
		t.Fatalf("FormatJSON = %q, err=%v", pretty, err)
	}

	tabbed, err := FormatJSONWithPad(raw, "\t", false)
	if err != nil || !strings.Contains(tabbed, "\n\t\"b\": 1") {
		t.Fatalf("FormatJSONWithPad tab = %q, err=%v", tabbed, err)
	}

	sorted, err := FormatJSON(raw, 2, true)
	if err != nil {
		t.Fatalf("FormatJSON sortKeys err=%v", err)
	}
	if strings.Index(sorted, `"a"`) > strings.Index(sorted, `"b"`) {
		t.Fatalf("expected key 'a' before 'b' in sorted JSON:\n%s", sorted)
	}

	miniSorted, err := MinifyJSONSorted(raw, true)
	if err != nil || !strings.HasPrefix(miniSorted, `{"a":`) {
		t.Fatalf("MinifyJSONSorted = %q, err=%v", miniSorted, err)
	}

	mini, err := MinifyJSON(pretty)
	if err != nil || mini != raw {
		t.Fatalf("MinifyJSON = %q, want %q, err=%v", mini, raw, err)
	}

	_, err = FormatJSON("{\n  \"a\": 1\n  \"b\": 2\n}", 2, false)
	info := ExtractJSONError(err)
	if info == nil || info.Line != 3 {
		t.Fatalf("ExtractJSONError = %+v, want line 3", info)
	}

	escaped := EscapeJSON(`{"name":"守护者"}`)
	if escaped != `{\"name\":\"守护者\"}` {
		t.Fatalf("EscapeJSON = %q", escaped)
	}
	unescaped, err := UnescapeJSON(escaped)
	if err != nil || unescaped != `{"name":"守护者"}` {
		t.Fatalf("UnescapeJSON = %q, err=%v", unescaped, err)
	}
}

func TestEncodeDecodeTools(t *testing.T) {
	b64 := Base64Encode("hello 守护者", false)
	dec, err := Base64Decode(b64, false)
	if err != nil || dec != "hello 守护者" {
		t.Fatalf("Base64 roundtrip = %q, err=%v", dec, err)
	}

	rawURL := Base64EncodeOpts("token?a/b=1", true, true)
	if strings.ContainsAny(rawURL, "+/=") {
		t.Fatalf("Base64EncodeOpts urlSafe+omitPadding = %q", rawURL)
	}

	if cleaned := CleanBase64("eyJ1aWQi@#$OjE= \n"); cleaned != "eyJ1aWQiOjE=" {
		t.Fatalf("CleanBase64 = %q", cleaned)
	}

	// 兼容缺少 = 填充的 Base64
	if got, err := Base64Decode(strings.TrimRight(b64, "="), false); err != nil || got != "hello 守护者" {
		t.Fatalf("Base64 unpadded = %q, err=%v", got, err)
	}

	encURL := URLEncode("a=1&name=守护者")
	decURL, err := URLDecode(encURL)
	if err != nil || decURL != "a=1&name=守护者" {
		t.Fatalf("URL roundtrip = %q, err=%v", decURL, err)
	}

	qPrev := ParseQueryPreview("https://api.example.com/v1/search?q=接口调试&env=生产&tag=a%2Bb")
	if len(qPrev) != 3 || qPrev[0].Key != "q" || qPrev[0].RawValue != "接口调试" || qPrev[2].RawValue != "a+b" {
		t.Fatalf("ParseQueryPreview = %+v", qPrev)
	}

	uEsc := UnicodeEscape("Hi 中文")
	if uEsc != `Hi \u4e2d\u6587` {
		t.Fatalf("UnicodeEscape = %q", uEsc)
	}
	uDec, err := UnicodeUnescape(uEsc)
	if err != nil || uDec != "Hi 中文" {
		t.Fatalf("UnicodeUnescape = %q, err=%v", uDec, err)
	}
}

func TestHashAndHMACTools(t *testing.T) {
	h := ComputeHashes("hello")
	if h.MD5Lower != "5d41402abc4b2a76b9719d911017c592" {
		t.Fatalf("MD5Lower = %q", h.MD5Lower)
	}
	if h.MD516 != "bc4b2a76b9719d91" {
		t.Fatalf("MD516 = %q", h.MD516)
	}
	if len(h.SHA256) != 64 || len(h.SHA512) != 128 {
		t.Fatalf("unexpected SHA lengths: %d / %d", len(h.SHA256), len(h.SHA512))
	}

	if alg, ok := MatchHash(h, strings.ToUpper(h.SHA256)); !ok || alg != "SHA-256" {
		t.Fatalf("MatchHash = (%q, %v), want (SHA-256, true)", alg, ok)
	}

	sigHex, err := ComputeHMAC("sha256", "secret", "message", "hex")
	if err != nil || sigHex != "8b5f48702995c1598c573db1e21866a9b825d4a794d169d7060a03605796360b" {
		t.Fatalf("ComputeHMAC hex = %q, err=%v", sigHex, err)
	}
	sigB64, err := ComputeHMAC("sha256", "secret", "message", "base64")
	if err != nil || sigB64 == "" {
		t.Fatalf("ComputeHMAC base64 = %q, err=%v", sigB64, err)
	}
}

func TestJWTAndTimeTools(t *testing.T) {
	secret := "my-test-secret-256"
	signed := SignJWTHS256(`{"alg":"HS256","typ":"JWT"}`, `{"sub":"user_1","exp":1700000000}`, secret)
	dec, err := DecodeJWT("Bearer " + signed)
	if err != nil {
		t.Fatalf("DecodeJWT err=%v", err)
	}
	if dec.Alg != "HS256" || !strings.Contains(dec.PayloadJSON, "user_1") || !dec.HasExp || !dec.Expired {
		t.Fatalf("DecodeJWT unexpected: %+v", dec)
	}

	if ok, alg, err := VerifyJWTHS(signed, secret, false); err != nil || !ok || alg != "HS256" {
		t.Fatalf("VerifyJWTHS valid = (%v, %q, %v)", ok, alg, err)
	}
	if ok, _, err := VerifyJWTHS(signed, "wrong-secret", false); err != nil || ok {
		t.Fatalf("VerifyJWTHS wrong = (%v, %v)", ok, err)
	}

	diags := DiagnoseJWT("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.@@BAD")
	if len(diags) != 3 || !diags[0].OK || diags[1].OK || diags[2].OK {
		t.Fatalf("DiagnoseJWT = %+v", diags)
	}

	utc8 := time.FixedZone("UTC+8", 8*3600)
	ts, err := TimestampToDateTimeInZone("1700000000", "s", utc8)
	if err != nil || ts.Seconds != 1700000000 || ts.Milliseconds != 1700000000000 || ts.ISO8601 == "" || ts.RFC2822 == "" {
		t.Fatalf("TimestampToDateTimeInZone = %+v, err=%v", ts, err)
	}
	dt, err := DateTimeToTimestampInZone("2026-11-23 16:00:55", utc8)
	if err != nil || dt.Seconds == 0 {
		t.Fatalf("DateTimeToTimestampInZone = %+v, err=%v", dt, err)
	}

	u1, u2 := NewUUIDv4(), NewUUIDv4()
	if len(u1) != 36 || u1 == u2 {
		t.Fatalf("NewUUIDv4 unexpected: %q / %q", u1, u2)
	}
	v7 := NewUUIDv7()
	if len(v7) != 36 || v7[14] != '7' {
		t.Fatalf("NewUUIDv7 unexpected: %q", v7)
	}
	ulid := NewULID()
	if len(ulid) != 26 {
		t.Fatalf("NewULID len = %d (%q)", len(ulid), ulid)
	}
	if f := FormatUUID(u1, true, true); len(f) != 32 || strings.Contains(f, "-") {
		t.Fatalf("FormatUUID = %q", f)
	}

	rs := GenerateRandomString(RandomStringOpts{Length: 32, Upper: true, Lower: true, Digits: true, Symbols: true, ExcludeAmbiguous: true})
	if len(rs) != 32 || strings.ContainsAny(rs, "0O1lI") {
		t.Fatalf("GenerateRandomString = %q", rs)
	}
	if bits := CalcEntropyBits(RandomStringOpts{Length: 32, Upper: true, Lower: true, Digits: true, Symbols: true, ExcludeAmbiguous: true}); bits < 180 {
		t.Fatalf("CalcEntropyBits = %.1f, want >= 180", bits)
	}
	if r := RandomString(24, true); len(r) != 24 {
		t.Fatalf("RandomString len = %d, want 24", len(r))
	}
}
