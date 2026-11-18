// Package transport 负责构建并发送 HTTP 请求、解析响应。
// 运行在后台 goroutine，结果交回 app 触发重绘，不阻塞主线程。
package transport

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/scripting"
)

// Response 是 HTTP 请求发送后返回的结构化响应数据。
type Response struct {
	StatusCode    int                 `json:"statusCode"`
	StatusText    string              `json:"statusText"`
	Proto         string              `json:"proto"`
	Duration      time.Duration       `json:"duration"`
	DurationMS    int                 `json:"durationMS"`
	Size          int64               `json:"size"`
	ContentType   string              `json:"contentType"`
	Headers       []domain.KV         `json:"headers"`
	Cookies       []ResponseCookie    `json:"cookies"`
	TestResults   []domain.TestResult `json:"testResults,omitempty"`
	UpdatedEnv    map[string]string   `json:"updatedEnv,omitempty"`
	Body          []byte              `json:"-"`
	BodyString    string              `json:"bodyString"`
	FormattedBody string              `json:"formattedBody"`
	Error         string              `json:"error,omitempty"`
	Timestamp     time.Time           `json:"timestamp"`
}

// ResponseCookie 描述响应中解析出的单个 Cookie。
type ResponseCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  string `json:"expires"`
	HttpOnly bool   `json:"httpOnly"`
	Secure   bool   `json:"secure"`
}

// Send 执行 HTTP 请求发送并返回结构化响应。
// 默认不抛出 Go error，而是将连接中断、超时等网络错误包装在 Response.Error 中返回，便于 UI 展示。
func Send(ctx context.Context, r domain.Request, auth domain.Auth) (*Response, error) {
	if r.PreRequestScript != "" {
		sCtx := scripting.NewScriptContext(0, "", 0, nil, "", nil)
		_ = sCtx.RunPreRequestScript(r.PreRequestScript)
		for k, v := range sCtx.Variables {
			r.URL = strings.ReplaceAll(r.URL, "{{"+k+"}}", v)
		}
		for k, v := range sCtx.Environment {
			r.URL = strings.ReplaceAll(r.URL, "{{"+k+"}}", v)
		}
	}

	req, err := BuildHTTPRequest(ctx, r, auth)
	if err != nil {
		return &Response{
			StatusCode: 0,
			StatusText: "Error",
			Error:      fmt.Sprintf("构建请求失败: %v", err),
			Timestamp:  time.Now(),
		}, nil
	}

	client := buildHTTPClient(r.Options)

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		errMsg := err.Error()
		if ctx.Err() == context.Canceled {
			errMsg = "请求已被用户取消"
		} else if strings.Contains(errMsg, "Client.Timeout") {
			errMsg = fmt.Sprintf("请求超时 (超过设置的 %d ms)", r.Options.TimeoutMS)
		}
		return &Response{
			StatusCode: 0,
			StatusText: "Error",
			Duration:   duration,
			DurationMS: int(duration.Milliseconds()),
			Error:      errMsg,
			Timestamp:  time.Now(),
		}, nil
	}
	defer resp.Body.Close()

	// 限制响应体最多读取 10MB，防止大文件导致内存耗尽
	limitReader := io.LimitReader(resp.Body, 10<<20)
	bodyBytes, readErr := io.ReadAll(limitReader)

	// 自动解压 gzip / deflate 响应体
	contentEncoding := strings.ToLower(resp.Header.Get("Content-Encoding"))
	if contentEncoding == "gzip" || (len(bodyBytes) > 2 && bodyBytes[0] == 0x1f && bodyBytes[1] == 0x8b) {
		if gzReader, err := gzip.NewReader(bytes.NewReader(bodyBytes)); err == nil {
			if decompressed, err := io.ReadAll(gzReader); err == nil {
				bodyBytes = decompressed
			}
			_ = gzReader.Close()
		}
	} else if contentEncoding == "deflate" {
		if flateReader := flate.NewReader(bytes.NewReader(bodyBytes)); flateReader != nil {
			if decompressed, err := io.ReadAll(flateReader); err == nil {
				bodyBytes = decompressed
			}
			_ = flateReader.Close()
		}
	}

	bodyStr := string(bodyBytes)
	formattedBody := bodyStr
	if len(bodyBytes) > 0 {
		var prettyBuf bytes.Buffer
		if json.Indent(&prettyBuf, bodyBytes, "", "  ") == nil {
			formattedBody = prettyBuf.String()
		}
	}

	// 提取 Content-Type
	contentType := resp.Header.Get("Content-Type")
	if idx := strings.Index(contentType, ";"); idx > 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	// 解析响应标头
	var respHeaders []domain.KV
	for k, vals := range resp.Header {
		for _, v := range vals {
			respHeaders = append(respHeaders, domain.KV{
				Key:     k,
				Value:   v,
				Enabled: true,
			})
		}
	}
	sort.Slice(respHeaders, func(i, j int) bool {
		return respHeaders[i].Key < respHeaders[j].Key
	})

	// 解析响应 Cookies
	var respCookies []ResponseCookie
	for _, ck := range resp.Cookies() {
		expiresStr := ""
		if !ck.Expires.IsZero() {
			expiresStr = ck.Expires.Format("2006-01-02 15:04:05")
		}
		respCookies = append(respCookies, ResponseCookie{
			Name:     ck.Name,
			Value:    ck.Value,
			Domain:   ck.Domain,
			Path:     ck.Path,
			Expires:  expiresStr,
			HttpOnly: ck.HttpOnly,
			Secure:   ck.Secure,
		})
	}

	var errMsg string
	if readErr != nil {
		errMsg = fmt.Sprintf("读取响应体发生错误: %v", readErr)
	}

	var testResults []domain.TestResult
	var updatedEnv map[string]string
	if r.TestsScript != "" {
		sCtx := scripting.NewScriptContext(resp.StatusCode, resp.Status, int(duration.Milliseconds()), respHeaders, formattedBody, nil)
		testResults = sCtx.RunTestsScript(r.TestsScript)
		if len(sCtx.Environment) > 0 {
			updatedEnv = sCtx.Environment
		}
	}

	return &Response{
		StatusCode:    resp.StatusCode,
		StatusText:    strings.TrimPrefix(resp.Status, fmt.Sprintf("%d ", resp.StatusCode)),
		Proto:         resp.Proto,
		Duration:      duration,
		DurationMS:    int(duration.Milliseconds()),
		Size:          int64(len(bodyBytes)),
		ContentType:   contentType,
		Headers:       respHeaders,
		Cookies:       respCookies,
		TestResults:   testResults,
		UpdatedEnv:    updatedEnv,
		Body:          bodyBytes,
		BodyString:    bodyStr,
		FormattedBody: formattedBody,
		Error:         errMsg,
		Timestamp:     time.Now(),
	}, nil
}

// BuildHTTPRequest 根据请求配置与认证凭证构建 Go 原生 *http.Request。
func BuildHTTPRequest(ctx context.Context, r domain.Request, auth domain.Auth) (*http.Request, error) {
	rawURL := strings.TrimSpace(r.URL)
	if rawURL == "" {
		return nil, fmt.Errorf("URL 不能为空")
	}

	// 1. 替换路径变量（如 /users/:id 或 /users/{id}）
	rawURL = ApplyPathVariables(rawURL, r.PathVariables)

	// 2. 自动补充 scheme（默认 http://）
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "http://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("无效的 URL: %w", err)
	}

	// 拼装 Query 参数（仅追加启用的参数）
	var queryPairs []string
	for _, q := range r.Query {
		if q.Enabled && strings.TrimSpace(q.Key) != "" {
			if q.Value != "" {
				queryPairs = append(queryPairs, url.QueryEscape(q.Key)+"="+url.QueryEscape(q.Value))
			} else {
				queryPairs = append(queryPairs, url.QueryEscape(q.Key))
			}
		}
	}
	if len(queryPairs) > 0 {
		extra := strings.Join(queryPairs, "&")
		if u.RawQuery != "" {
			u.RawQuery += "&" + extra
		} else {
			u.RawQuery = extra
		}
	}

	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	var defaultContentType string

	switch r.Body.Mode {
	case domain.BodyRaw:
		bodyReader = strings.NewReader(r.Body.Raw)
		trimmed := strings.TrimSpace(r.Body.Raw)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			defaultContentType = "application/json; charset=utf-8"
		} else {
			defaultContentType = "text/plain; charset=utf-8"
		}
	case domain.BodyURLEncoded:
		vals := url.Values{}
		for _, f := range r.Body.Fields {
			if f.Enabled && strings.TrimSpace(f.Key) != "" {
				vals.Add(f.Key, f.Value)
			}
		}
		bodyReader = strings.NewReader(vals.Encode())
		defaultContentType = "application/x-www-form-urlencoded"
	case domain.BodyFormData:
		var b bytes.Buffer
		mw := multipart.NewWriter(&b)
		for _, f := range r.Body.Fields {
			if !f.Enabled || strings.TrimSpace(f.Key) == "" {
				continue
			}
			isFile := f.Type == "file" || strings.HasPrefix(f.Value, "@")
			filePath := strings.TrimPrefix(f.Value, "@")
			if isFile && filePath != "" {
				file, err := os.Open(filePath)
				if err != nil {
					return nil, fmt.Errorf("打开表单上传文件失败 (%s): %w", filePath, err)
				}
				filename := filepath.Base(filePath)
				part, err := mw.CreateFormFile(f.Key, filename)
				if err != nil {
					_ = file.Close()
					return nil, fmt.Errorf("创建表单文件字段失败 (%s): %w", f.Key, err)
				}
				if _, err := io.Copy(part, file); err != nil {
					_ = file.Close()
					return nil, fmt.Errorf("读取表单上传文件失败: %w", err)
				}
				_ = file.Close()
			} else {
				_ = mw.WriteField(f.Key, f.Value)
			}
		}
		_ = mw.Close()
		bodyReader = &b
		defaultContentType = mw.FormDataContentType()
	case domain.BodyBinary:
		if r.Body.File != "" {
			data, err := os.ReadFile(r.Body.File)
			if err != nil {
				return nil, fmt.Errorf("打开二进制文件失败 (%s): %w", r.Body.File, err)
			}
			bodyReader = bytes.NewReader(data)
		} else if r.Body.Raw != "" {
			bodyReader = strings.NewReader(r.Body.Raw)
		}
		defaultContentType = "application/octet-stream"
	case domain.BodyGraphQL:
		if r.Body.GraphQL != nil {
			payload, _ := json.Marshal(map[string]any{
				"query":     r.Body.GraphQL.Query,
				"variables": r.Body.GraphQL.Variables,
			})
			bodyReader = bytes.NewReader(payload)
			defaultContentType = "application/json; charset=utf-8"
		}
	default:
		bodyReader = nil
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	// 写入默认 Content-Type
	if defaultContentType != "" {
		req.Header.Set("Content-Type", defaultContentType)
	}

	// 写入请求头（若用户已配置同名请求头，覆盖默认值）
	hasUserAgent := false
	for _, h := range r.Headers {
		if h.Enabled && strings.TrimSpace(h.Key) != "" {
			if strings.EqualFold(h.Key, "Host") {
				req.Host = h.Value
			} else {
				req.Header.Set(h.Key, h.Value)
			}
			if strings.EqualFold(h.Key, "User-Agent") {
				hasUserAgent = true
			}
		}
	}
	if !hasUserAgent {
		req.Header.Set("User-Agent", "rddPostman/1.0")
	}

	// 写入 Cookies
	for _, ck := range r.Cookies {
		if ck.Enabled && strings.TrimSpace(ck.Key) != "" {
			req.AddCookie(&http.Cookie{
				Name:  ck.Key,
				Value: ck.Value,
			})
		}
	}

	// 注入认证信息
	ApplyAuth(req, auth)

	return req, nil
}

// ApplyAuth 将解析后的最终 Auth 配置注入到原生 http.Request 中。
func ApplyAuth(req *http.Request, auth domain.Auth) {
	switch auth.Type {
	case domain.AuthBearer:
		token := findParam(auth.Params, "token")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	case domain.AuthBasic:
		username := findParam(auth.Params, "username")
		password := findParam(auth.Params, "password")
		if username != "" || password != "" {
			req.SetBasicAuth(username, password)
		}
	case domain.AuthAPIKey:
		key := findParam(auth.Params, "key")
		value := findParam(auth.Params, "value")
		in := strings.ToLower(findParam(auth.Params, "in"))
		if key != "" {
			if in == "query" {
				q := req.URL.Query()
				q.Set(key, value)
				req.URL.RawQuery = q.Encode()
			} else {
				req.Header.Set(key, value)
			}
		}
	}
}

func findParam(params []domain.KV, key string) string {
	for _, p := range params {
		if strings.EqualFold(p.Key, key) {
			return p.Value
		}
	}
	return ""
}

func buildHTTPClient(opts domain.Options) *http.Client {
	timeout := 30 * time.Second
	if opts.TimeoutMS > 0 {
		timeout = time.Duration(opts.TimeoutMS) * time.Millisecond
	}

	followRedirects := true
	if opts.FollowRedirects != nil {
		followRedirects = *opts.FollowRedirects
	}

	verifyTLS := true
	if opts.VerifyTLS != nil {
		verifyTLS = *opts.VerifyTLS
	}

	proxyFunc := http.ProxyFromEnvironment
	if opts.DisableProxy {
		proxyFunc = nil
	} else if opts.ProxyURL != "" {
		if pURL, err := url.Parse(opts.ProxyURL); err == nil {
			proxyFunc = http.ProxyURL(pURL)
		}
	}

	tr := &http.Transport{
		Proxy: proxyFunc,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !verifyTLS,
		},
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	if !opts.DisableCookieJar {
		client.Jar = getDefaultCookieJar()
	}

	if !followRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return client
}

var (
	defaultCookieJarOnce sync.Once
	defaultCookieJar     http.CookieJar
)

func getDefaultCookieJar() http.CookieJar {
	defaultCookieJarOnce.Do(func() {
		defaultCookieJar, _ = cookiejar.New(nil)
	})
	return defaultCookieJar
}

// FormatDuration 将 time.Duration 格式化为适合 Postman UI 的耗时字符串（如 42 ms 或 1.25 s）
func FormatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%d µs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%d ms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2f s", float64(d.Milliseconds())/1000.0)
}

// FormatSize 将字节数大小格式化为适合 UI 展示的字符串（如 512 B、1.85 KB 或 3.20 MB）
func FormatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(bytes)/1024.0)
	}
	return fmt.Sprintf("%.2f MB", float64(bytes)/(1024.0*1024.0))
}

// ApplyPathVariables 将 URL 中的路径变量（支持 :key 与 {key} 语法）替换为真实值。
func ApplyPathVariables(rawURL string, pathVars []domain.KV) string {
	return domain.ApplyPathVariables(rawURL, pathVars)
}
