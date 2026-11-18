// Package curl 提供 cURL 命令与 domain.Request 之间的相互转换。
package curl

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"rddpostman/internal/domain"
)

// ToCurl 将 domain.Request 与 domain.Auth 序列化为可直接执行的 cURL 命令行。
func ToCurl(r domain.Request, auth domain.Auth) string {
	var parts []string
	parts = append(parts, "curl")

	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method == "" {
		method = "GET"
	}
	if method != "GET" {
		parts = append(parts, "-X", method)
	}

	// 拼装 URL 与 Query（包含路径变量替换）
	rawURL := strings.TrimSpace(r.URL)
	rawURL = domain.ApplyPathVariables(rawURL, r.PathVariables)
	if rawURL == "" {
		rawURL = "http://localhost"
	}
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
		sep := "?"
		if strings.Contains(rawURL, "?") {
			sep = "&"
		}
		rawURL += sep + strings.Join(queryPairs, "&")
	}
	parts = append(parts, quoteArg(rawURL))

	// Headers
	for _, h := range r.Headers {
		if h.Enabled && strings.TrimSpace(h.Key) != "" {
			parts = append(parts, "-H", quoteArg(fmt.Sprintf("%s: %s", h.Key, h.Value)))
		}
	}

	// Cookies
	var cookiePairs []string
	for _, ck := range r.Cookies {
		if ck.Enabled && strings.TrimSpace(ck.Key) != "" {
			cookiePairs = append(cookiePairs, fmt.Sprintf("%s=%s", ck.Key, ck.Value))
		}
	}
	if len(cookiePairs) > 0 {
		parts = append(parts, "-b", quoteArg(strings.Join(cookiePairs, "; ")))
	}

	// Auth
	switch auth.Type {
	case domain.AuthBearer:
		token := findParam(auth.Params, "token")
		if token != "" {
			parts = append(parts, "-H", quoteArg("Authorization: Bearer "+token))
		}
	case domain.AuthBasic:
		username := findParam(auth.Params, "username")
		password := findParam(auth.Params, "password")
		if username != "" || password != "" {
			parts = append(parts, "-u", quoteArg(fmt.Sprintf("%s:%s", username, password)))
		}
	case domain.AuthAPIKey:
		key := findParam(auth.Params, "key")
		val := findParam(auth.Params, "value")
		in := strings.ToLower(findParam(auth.Params, "in"))
		if key != "" && in != "query" {
			parts = append(parts, "-H", quoteArg(fmt.Sprintf("%s: %s", key, val)))
		}
	}

	// Body
	switch r.Body.Mode {
	case domain.BodyRaw:
		if r.Body.Raw != "" {
			parts = append(parts, "--data-raw", quoteArg(r.Body.Raw))
		}
	case domain.BodyURLEncoded:
		for _, f := range r.Body.Fields {
			if f.Enabled && strings.TrimSpace(f.Key) != "" {
				parts = append(parts, "--data-urlencode", quoteArg(fmt.Sprintf("%s=%s", f.Key, f.Value)))
			}
		}
	case domain.BodyFormData:
		for _, f := range r.Body.Fields {
			if f.Enabled && strings.TrimSpace(f.Key) != "" {
				val := f.Value
				if f.Type == "file" && !strings.HasPrefix(val, "@") {
					val = "@" + val
				}
				parts = append(parts, "-F", quoteArg(fmt.Sprintf("%s=%s", f.Key, val)))
			}
		}
	case domain.BodyBinary:
		if r.Body.File != "" {
			parts = append(parts, "--data-binary", quoteArg("@"+r.Body.File))
		} else if r.Body.Raw != "" {
			parts = append(parts, "--data-binary", quoteArg(r.Body.Raw))
		}
	case domain.BodyGraphQL:
		if r.Body.GraphQL != nil && r.Body.GraphQL.Query != "" {
			parts = append(parts, "-H", quoteArg("Content-Type: application/json"))
			parts = append(parts, "--data-raw", quoteArg(r.Body.GraphQL.Query))
		}
	}

	return strings.Join(parts, " ")
}

// FromCurl 解析一行或多行 cURL 命令，提取出 domain.Request 与 domain.Auth。
func FromCurl(curlCmd string) (domain.Request, domain.Auth, error) {
	tokens := tokenize(curlCmd)
	if len(tokens) == 0 {
		return domain.Request{}, domain.Auth{}, fmt.Errorf("cURL 命令为空")
	}

	// 忽略起始的 curl
	if strings.EqualFold(tokens[0], "curl") {
		tokens = tokens[1:]
	}

	req := domain.Request{
		Method:  "GET",
		Options: domain.Options{TimeoutMS: 10000},
	}
	var auth domain.Auth

	i := 0
	hasExplicitMethod := false
	for i < len(tokens) {
		tok := tokens[i]
		switch tok {
		case "-X", "--request":
			if i+1 < len(tokens) {
				req.Method = strings.ToUpper(tokens[i+1])
				hasExplicitMethod = true
				i += 2
				continue
			}
		case "-H", "--header":
			if i+1 < len(tokens) {
				h := tokens[i+1]
				colon := strings.Index(h, ":")
				if colon > 0 {
					key := strings.TrimSpace(h[:colon])
					val := strings.TrimSpace(h[colon+1:])
					if strings.EqualFold(key, "Authorization") && strings.HasPrefix(strings.ToLower(val), "bearer ") {
						auth.Type = domain.AuthBearer
						auth.Params = []domain.KV{{Key: "token", Value: strings.TrimSpace(val[7:])}}
					} else {
						req.Headers = append(req.Headers, domain.KV{
							ID:      domain.NewID(),
							Key:     key,
							Value:   val,
							Enabled: true,
						})
					}
				}
				i += 2
				continue
			}
		case "-u", "--user":
			if i+1 < len(tokens) {
				userPass := tokens[i+1]
				parts := strings.SplitN(userPass, ":", 2)
				auth.Type = domain.AuthBasic
				username, password := parts[0], ""
				if len(parts) > 1 {
					password = parts[1]
				}
				auth.Params = []domain.KV{
					{Key: "username", Value: username},
					{Key: "password", Value: password},
				}
				i += 2
				continue
			}
		case "-b", "--cookie":
			if i+1 < len(tokens) {
				cookieStr := tokens[i+1]
				for _, part := range strings.Split(cookieStr, ";") {
					part = strings.TrimSpace(part)
					if part == "" {
						continue
					}
					eq := strings.Index(part, "=")
					if eq > 0 {
						req.Cookies = append(req.Cookies, domain.KV{
							ID:      domain.NewID(),
							Key:     strings.TrimSpace(part[:eq]),
							Value:   strings.TrimSpace(part[eq+1:]),
							Enabled: true,
						})
					}
				}
				i += 2
				continue
			}
		case "-d", "--data", "--data-raw", "--data-ascii":
			if i+1 < len(tokens) {
				req.Body.Mode = domain.BodyRaw
				req.Body.Raw = tokens[i+1]
				if !hasExplicitMethod {
					req.Method = "POST"
				}
				i += 2
				continue
			}
		case "--data-urlencode":
			if i+1 < len(tokens) {
				req.Body.Mode = domain.BodyURLEncoded
				field := tokens[i+1]
				eq := strings.Index(field, "=")
				if eq > 0 {
					req.Body.Fields = append(req.Body.Fields, domain.KV{
						ID:      domain.NewID(),
						Key:     field[:eq],
						Value:   field[eq+1:],
						Enabled: true,
					})
				}
				if !hasExplicitMethod {
					req.Method = "POST"
				}
				i += 2
				continue
			}
		case "-F", "--form":
			if i+1 < len(tokens) {
				req.Body.Mode = domain.BodyFormData
				field := tokens[i+1]
				eq := strings.Index(field, "=")
				if eq > 0 {
					key := field[:eq]
					val := field[eq+1:]
					fType := "text"
					if strings.HasPrefix(val, "@") {
						fType = "file"
					}
					req.Body.Fields = append(req.Body.Fields, domain.KV{
						ID:      domain.NewID(),
						Key:     key,
						Value:   val,
						Type:    fType,
						Enabled: true,
					})
				}
				if !hasExplicitMethod {
					req.Method = "POST"
				}
				i += 2
				continue
			}
		case "--data-binary":
			if i+1 < len(tokens) {
				req.Body.Mode = domain.BodyBinary
				val := tokens[i+1]
				if strings.HasPrefix(val, "@") {
					req.Body.File = strings.TrimPrefix(val, "@")
				} else {
					req.Body.Raw = val
				}
				if !hasExplicitMethod {
					req.Method = "POST"
				}
				i += 2
				continue
			}
		case "-L", "--location":
			follow := true
			req.Options.FollowRedirects = &follow
			i++
			continue
		case "-k", "--insecure":
			verify := false
			req.Options.VerifyTLS = &verify
			i++
			continue
		case "-x", "--proxy":
			if i+1 < len(tokens) {
				req.Options.ProxyURL = tokens[i+1]
				i += 2
				continue
			}
		case "--compressed":
			i++
			continue
		case "-A", "--user-agent":
			if i+1 < len(tokens) {
				req.Headers = append(req.Headers, domain.KV{
					ID:      domain.NewID(),
					Key:     "User-Agent",
					Value:   tokens[i+1],
					Enabled: true,
				})
				i += 2
				continue
			}
		case "-m", "--max-time", "--connect-timeout":
			if i+1 < len(tokens) {
				if sec, err := strconv.ParseFloat(tokens[i+1], 64); err == nil && sec > 0 {
					req.Options.TimeoutMS = int(sec * 1000)
				}
				i += 2
				continue
			}
		default:
			// 可能是 URL 或未识别参数
			if !strings.HasPrefix(tok, "-") && req.URL == "" {
				req.URL = tok
			}
		}
		i++
	}

	// 拆分 URL 中的 Query 参数存入 req.Query
	if req.URL != "" {
		if u, err := url.Parse(req.URL); err == nil && u.RawQuery != "" {
			for k, vals := range u.Query() {
				for _, v := range vals {
					req.Query = append(req.Query, domain.KV{
						ID:      domain.NewID(),
						Key:     k,
						Value:   v,
						Enabled: true,
					})
				}
			}
			// 保留基础 URL（去除 query 串）
			u.RawQuery = ""
			req.URL = u.String()
		}
	}

	return req, auth, nil
}

func findParam(params []domain.KV, key string) string {
	for _, p := range params {
		if strings.EqualFold(p.Key, key) {
			return p.Value
		}
	}
	return ""
}

func quoteArg(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\n\r\"'\\#&<>|;*?$") {
		return s
	}
	return strconv.Quote(s)
}

// tokenize 拆分带单引号、双引号、反斜杠换行的 cURL 命令行
func tokenize(cmd string) []string {
	// 去除行末反斜杠拼接换行
	reBackslash := regexp.MustCompile(`\\\s*[\r\n]+`)
	cmd = reBackslash.ReplaceAllString(cmd, " ")

	var tokens []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]

		if escaped {
			cur.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}

		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}

		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}

		if (ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r') && !inSingle && !inDouble {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
			continue
		}

		cur.WriteByte(ch)
	}

	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}

	return tokens
}
