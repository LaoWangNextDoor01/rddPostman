package curl

import (
	"strings"
	"testing"

	"rddpostman/internal/domain"
)

func TestToCurl(t *testing.T) {
	req := domain.Request{
		Method: "POST",
		URL:    "https://api.example.com/v1/users",
		Query: []domain.KV{
			{Key: "page", Value: "1", Enabled: true},
			{Key: "disabled", Value: "foo", Enabled: false},
		},
		Headers: []domain.KV{
			{Key: "X-Trace-ID", Value: "abc-123", Enabled: true},
			{Key: "Content-Type", Value: "application/json", Enabled: true},
		},
		Body: domain.Body{
			Mode: domain.BodyRaw,
			Raw:  `{"name":"Ada"}`,
		},
	}
	auth := domain.Auth{
		Type: domain.AuthBearer,
		Params: []domain.KV{
			{Key: "token", Value: "my-secret-token"},
		},
	}

	cmd := ToCurl(req, auth)
	if !strings.HasPrefix(cmd, "curl -X POST") {
		t.Errorf("expected starts with curl -X POST, got %s", cmd)
	}
	if !strings.Contains(cmd, "https://api.example.com/v1/users?page=1") {
		t.Errorf("URL with query missing: %s", cmd)
	}
	if !strings.Contains(cmd, `-H "X-Trace-ID: abc-123"`) {
		t.Errorf("Header missing: %s", cmd)
	}
	if !strings.Contains(cmd, `-H "Authorization: Bearer my-secret-token"`) {
		t.Errorf("Bearer token missing: %s", cmd)
	}
	if !strings.Contains(cmd, `--data-raw "{\"name\":\"Ada\"}"`) {
		t.Errorf("Body missing: %s", cmd)
	}
}

func TestFromCurl(t *testing.T) {
	curlCmd := `curl 'https://api.example.com/items?limit=10' \
  -X POST \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer test-token' \
  -b 'session_id=s123; user=alice' \
  --data-raw '{"item":"book"}'`

	req, auth, err := FromCurl(curlCmd)
	if err != nil {
		t.Fatalf("FromCurl err: %v", err)
	}
	if req.Method != "POST" {
		t.Errorf("expected POST, got %s", req.Method)
	}
	if req.URL != "https://api.example.com/items" {
		t.Errorf("expected base URL https://api.example.com/items, got %s", req.URL)
	}
	if len(req.Query) != 1 || req.Query[0].Key != "limit" || req.Query[0].Value != "10" {
		t.Errorf("expected query limit=10, got %+v", req.Query)
	}
	if auth.Type != domain.AuthBearer || auth.Params[0].Value != "test-token" {
		t.Errorf("expected auth bearer test-token, got %+v", auth)
	}
	if req.Body.Mode != domain.BodyRaw || req.Body.Raw != `{"item":"book"}` {
		t.Errorf("expected body raw, got %+v", req.Body)
	}
	if len(req.Cookies) != 2 || req.Cookies[0].Key != "session_id" || req.Cookies[0].Value != "s123" {
		t.Errorf("expected cookies parsed, got %+v", req.Cookies)
	}

	// 测试新增标志: -L, -k, -x, -A, -m, --compressed
	advancedCmd := `curl -L -k -x http://127.0.0.1:7890 -A "CustomAgent/1.0" -m 5 --compressed https://api.example.com/test`
	advReq, _, err := FromCurl(advancedCmd)
	if err != nil {
		t.Fatalf("advanced FromCurl err: %v", err)
	}
	if advReq.Options.FollowRedirects == nil || !*advReq.Options.FollowRedirects {
		t.Errorf("expected FollowRedirects=true")
	}
	if advReq.Options.VerifyTLS == nil || *advReq.Options.VerifyTLS {
		t.Errorf("expected VerifyTLS=false")
	}
	if advReq.Options.ProxyURL != "http://127.0.0.1:7890" {
		t.Errorf("expected ProxyURL=http://127.0.0.1:7890, got %s", advReq.Options.ProxyURL)
	}
	if advReq.Options.TimeoutMS != 5000 {
		t.Errorf("expected TimeoutMS=5000, got %d", advReq.Options.TimeoutMS)
	}
	hasUA := false
	for _, h := range advReq.Headers {
		if h.Key == "User-Agent" && h.Value == "CustomAgent/1.0" {
			hasUA = true
		}
	}
	if !hasUA {
		t.Errorf("expected User-Agent header from -A")
	}
}
