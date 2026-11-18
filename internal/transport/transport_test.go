package transport

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"rddpostman/internal/domain"
)

func TestTransportSendGET(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("expected page=1, got %s", r.URL.Query().Get("page"))
		}
		if r.Header.Get("X-Custom") != "test-val" {
			t.Errorf("expected X-Custom header")
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "12345"})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","count":42}`))
	}))
	defer ts.Close()

	req := domain.Request{
		Method: "GET",
		URL:    ts.URL,
		Query: []domain.KV{
			{Key: "page", Value: "1", Enabled: true},
			{Key: "unused", Value: "foo", Enabled: false},
		},
		Headers: []domain.KV{
			{Key: "X-Custom", Value: "test-val", Enabled: true},
		},
	}

	resp, err := Send(context.Background(), req, domain.Auth{})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.FormattedBody, "count") {
		t.Fatalf("expected body to contain count, got %s", resp.FormattedBody)
	}
	if len(resp.Cookies) != 1 || resp.Cookies[0].Name != "session" {
		t.Fatalf("expected cookie session, got %+v", resp.Cookies)
	}
	if resp.Duration <= 0 {
		t.Fatalf("expected positive duration, got %v", resp.Duration)
	}
}

func TestTransportSendPOSTWithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("invalid json body: %v", err)
		}
		if payload["name"] != "postman" {
			t.Errorf("expected name=postman, got %v", payload["name"])
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"created":true}`))
	}))
	defer ts.Close()

	req := domain.Request{
		Method: "POST",
		URL:    ts.URL + "/create",
		Body: domain.Body{
			Mode: domain.BodyRaw,
			Raw:  `{"name":"postman"}`,
		},
	}

	resp, err := Send(context.Background(), req, domain.Auth{})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.FormattedBody, "created") {
		t.Fatalf("unexpected body: %s", resp.FormattedBody)
	}
}

func TestTransportApplyAuth(t *testing.T) {
	// 1. Bearer Token
	reqBearer, _ := http.NewRequest("GET", "http://example.com", nil)
	ApplyAuth(reqBearer, domain.Auth{
		Type: domain.AuthBearer,
		Params: []domain.KV{
			{Key: "token", Value: "sec-token-123"},
		},
	})
	if reqBearer.Header.Get("Authorization") != "Bearer sec-token-123" {
		t.Fatalf("expected Bearer token, got %s", reqBearer.Header.Get("Authorization"))
	}

	// 2. Basic Auth
	reqBasic, _ := http.NewRequest("GET", "http://example.com", nil)
	ApplyAuth(reqBasic, domain.Auth{
		Type: domain.AuthBasic,
		Params: []domain.KV{
			{Key: "username", Value: "admin"},
			{Key: "password", Value: "secret"},
		},
	})
	u, p, ok := reqBasic.BasicAuth()
	if !ok || u != "admin" || p != "secret" {
		t.Fatalf("expected basic auth admin:secret, got %s:%s (ok=%v)", u, p, ok)
	}

	// 3. API Key in Header
	reqAPIKey, _ := http.NewRequest("GET", "http://example.com", nil)
	ApplyAuth(reqAPIKey, domain.Auth{
		Type: domain.AuthAPIKey,
		Params: []domain.KV{
			{Key: "key", Value: "X-API-KEY"},
			{Key: "value", Value: "key-999"},
			{Key: "in", Value: "header"},
		},
	})
	if reqAPIKey.Header.Get("X-API-KEY") != "key-999" {
		t.Fatalf("expected API Key in header, got %s", reqAPIKey.Header.Get("X-API-KEY"))
	}

	// 4. API Key in Query
	reqAPIKeyQuery, _ := http.NewRequest("GET", "http://example.com/api?a=1", nil)
	ApplyAuth(reqAPIKeyQuery, domain.Auth{
		Type: domain.AuthAPIKey,
		Params: []domain.KV{
			{Key: "key", Value: "token"},
			{Key: "value", Value: "xyz"},
			{Key: "in", Value: "query"},
		},
	})
	if reqAPIKeyQuery.URL.Query().Get("token") != "xyz" {
		t.Fatalf("expected API Key in query, got %s", reqAPIKeyQuery.URL.RawQuery)
	}
}

func TestFormatHelpers(t *testing.T) {
	if FormatDuration(50*time.Microsecond) != "50 µs" {
		t.Errorf("FormatDuration microsecond mismatch")
	}
	if FormatDuration(142*time.Millisecond) != "142 ms" {
		t.Errorf("FormatDuration millisecond mismatch")
	}
	if FormatDuration(1500*time.Millisecond) != "1.50 s" {
		t.Errorf("FormatDuration second mismatch")
	}

	if FormatSize(512) != "512 B" {
		t.Errorf("FormatSize B mismatch")
	}
	if FormatSize(1536) != "1.50 KB" {
		t.Errorf("FormatSize KB mismatch")
	}
}

func TestApplyPathVariables(t *testing.T) {
	url := "http://localhost:8080/api/v1/orgs/:orgId/users/{userId}/posts/:postId"
	pvs := []domain.KV{
		{Key: "orgId", Value: "my-org", Enabled: true},
		{Key: "userId", Value: "user 123", Enabled: true}, // 含空格需转义
		{Key: "postId", Value: "p-999", Enabled: true},
		{Key: "disabled", Value: "unused", Enabled: false},
	}
	result := ApplyPathVariables(url, pvs)
	expected := "http://localhost:8080/api/v1/orgs/my-org/users/user%20123/posts/p-999"
	if result != expected {
		t.Fatalf("ApplyPathVariables got %s, want %s", result, expected)
	}
}

func TestBodyBinaryAndFormData(t *testing.T) {
	// 创建临时测试文件
	tmpFile, err := os.CreateTemp("", "test-upload-*.txt")
	if err != nil {
		t.Fatalf("create temp file err: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	_, _ = tmpFile.WriteString("hello binary content")
	_ = tmpFile.Close()

	// 1. 测试 BodyBinary 文件读取
	reqBinary := domain.Request{
		Method: "POST",
		URL:    "http://example.com/upload-bin",
		Body: domain.Body{
			Mode: domain.BodyBinary,
			File: tmpFile.Name(),
		},
	}
	httpReq, err := BuildHTTPRequest(context.Background(), reqBinary, domain.Auth{})
	if err != nil {
		t.Fatalf("build binary req error: %v", err)
	}
	binBytes, _ := io.ReadAll(httpReq.Body)
	if string(binBytes) != "hello binary content" {
		t.Fatalf("expected binary content, got %s", string(binBytes))
	}
	if httpReq.Header.Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("expected binary content type application/octet-stream, got %s", httpReq.Header.Get("Content-Type"))
	}

	// 2. 测试 BodyFormData 混合文本与文件
	reqFormData := domain.Request{
		Method: "POST",
		URL:    "http://example.com/upload-form",
		Body: domain.Body{
			Mode: domain.BodyFormData,
			Fields: []domain.KV{
				{Key: "title", Value: "My Document", Enabled: true},
				{Key: "doc", Value: "@" + tmpFile.Name(), Enabled: true},
			},
		},
	}
	httpFormReq, err := BuildHTTPRequest(context.Background(), reqFormData, domain.Auth{})
	if err != nil {
		t.Fatalf("build form req error: %v", err)
	}
	if !strings.HasPrefix(httpFormReq.Header.Get("Content-Type"), "multipart/form-data; boundary=") {
		t.Fatalf("expected multipart content type, got %s", httpFormReq.Header.Get("Content-Type"))
	}
	err = httpFormReq.ParseMultipartForm(10 << 20)
	if err != nil {
		t.Fatalf("parse multipart error: %v", err)
	}
	if httpFormReq.FormValue("title") != "My Document" {
		t.Fatalf("expected form title=My Document, got %s", httpFormReq.FormValue("title"))
	}
	fileHeader, ok := httpFormReq.MultipartForm.File["doc"]
	if !ok || len(fileHeader) == 0 {
		t.Fatalf("expected multipart file doc")
	}
	uploaded, err := fileHeader[0].Open()
	if err != nil {
		t.Fatalf("open uploaded part error: %v", err)
	}
	defer uploaded.Close()
	content, _ := io.ReadAll(uploaded)
	if string(content) != "hello binary content" {
		t.Fatalf("expected uploaded file content, got %s", string(content))
	}
}

func TestTransportCookieJarAndSession(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "session_token", Value: "sec-9999", Path: "/"})
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"login":true}`))
			return
		}
		cookie, err := r.Cookie("session_token")
		if err != nil || cookie.Value != "sec-9999" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"authorized":false}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"authorized":true}`))
	}))
	defer ts.Close()

	// 1. 发起 /login 请求，服务器返回 Set-Cookie
	reqLogin := domain.Request{
		Method: "POST",
		URL:    ts.URL + "/login",
	}
	respLogin, err := Send(context.Background(), reqLogin, domain.Auth{})
	if err != nil || respLogin.StatusCode != 200 {
		t.Fatalf("login failed: %v, resp: %+v", err, respLogin)
	}

	// 2. 发起 /profile 请求，CookieJar 自动携带 session_token
	reqProfile := domain.Request{
		Method: "GET",
		URL:    ts.URL + "/profile",
	}
	respProfile, err := Send(context.Background(), reqProfile, domain.Auth{})
	if err != nil || respProfile.StatusCode != 200 {
		t.Fatalf("profile should be authorized with cookie jar, got: %+v", respProfile)
	}

	// 3. 禁用 CookieJar 时不自动携带
	reqNoJar := domain.Request{
		Method: "GET",
		URL:    ts.URL + "/profile",
		Options: domain.Options{
			DisableCookieJar: true,
		},
	}
	respNoJar, err := Send(context.Background(), reqNoJar, domain.Auth{})
	if err != nil || respNoJar.StatusCode != 401 {
		t.Fatalf("profile without cookie jar should be 401, got: %+v", respNoJar)
	}
}

func TestTransportPreRequestAndTestsScript(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/888" {
			t.Errorf("expected URL /users/888, got %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"data":{"id":888,"name":"Alice"}}`))
	}))
	defer ts.Close()

	req := domain.Request{
		Method: "GET",
		URL:    ts.URL + "/users/{{userId}}",
		PreRequestScript: `
			// 设置变量
			pm.variables.set("userId", "888");
		`,
		TestsScript: `
			pm.test("Status code is 200", function () {
				pm.response.to.have.status(200);
			});
			pm.test("Response time is reasonable", function () {
				pm.response.to.have.responseTime.below(2000);
			});
			pm.test("User name is Alice", function () {
				pm.expect(pm.response.json().data.name).to.eql("Alice");
			});
			pm.test("Failing test case", function () {
				pm.expect(pm.response.json().data.name).to.eql("Bob");
			});
		`,
	}

	resp, err := Send(context.Background(), req, domain.Auth{})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(resp.TestResults) != 4 {
		t.Fatalf("expected 4 test results, got %d: %+v", len(resp.TestResults), resp.TestResults)
	}
	if !resp.TestResults[0].Passed || resp.TestResults[0].Name != "Status code is 200" {
		t.Fatalf("test 0 should pass: %+v", resp.TestResults[0])
	}
	if !resp.TestResults[1].Passed {
		t.Fatalf("test 1 should pass: %+v", resp.TestResults[1])
	}
	if !resp.TestResults[2].Passed {
		t.Fatalf("test 2 should pass: %+v", resp.TestResults[2])
	}
	if resp.TestResults[3].Passed || !strings.Contains(resp.TestResults[3].Error, "期望为 Bob") {
		t.Fatalf("test 3 should fail: %+v", resp.TestResults[3])
	}
}

func TestTransportProxyConfig(t *testing.T) {
	proxyHit := false
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHit = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"proxied":true}`))
	}))
	defer proxyServer.Close()

	req := domain.Request{
		Method: "GET",
		URL:    "http://some.remote.fake.target/api",
		Options: domain.Options{
			ProxyURL: proxyServer.URL,
		},
	}

	resp, err := Send(context.Background(), req, domain.Auth{})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if !proxyHit {
		t.Fatalf("expected proxy server to be hit")
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
