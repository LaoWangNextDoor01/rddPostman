// Package scripting 提供轻量级测试断言与前后置脚本执行引擎。
package scripting

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"rddpostman/internal/domain"
)

// ScriptContext 是脚本运行上下文，包含响应数据、环境变量与测试断言结果。
type ScriptContext struct {
	StatusCode    int
	StatusText    string
	DurationMS    int
	Headers       map[string]string
	BodyString    string
	Environment   map[string]string
	Variables     map[string]string
	TestResults   []domain.TestResult
}

// NewScriptContext 基于当前响应创建测试执行上下文。
func NewScriptContext(statusCode int, statusText string, durationMS int, headers []domain.KV, body string, envVars map[string]string) *ScriptContext {
	hMap := make(map[string]string)
	for _, h := range headers {
		hMap[strings.ToLower(strings.TrimSpace(h.Key))] = h.Value
	}
	env := make(map[string]string)
	for k, v := range envVars {
		env[k] = v
	}
	return &ScriptContext{
		StatusCode:  statusCode,
		StatusText:  statusText,
		DurationMS:  durationMS,
		Headers:     hMap,
		BodyString:  body,
		Environment: env,
		Variables:   make(map[string]string),
	}
}

// RunPreRequestScript 执行请求前置脚本，支持 pm.environment.set 与 pm.variables.set。
func (ctx *ScriptContext) RunPreRequestScript(script string) error {
	lines := strings.Split(script, "\n")
	reEnvSet := regexp.MustCompile(`pm\.environment\.set\(\s*["']([^"']+)["']\s*,\s*["']?([^"')]+)["']?\s*\)`)
	reVarSet := regexp.MustCompile(`pm\.variables\.set\(\s*["']([^"']+)["']\s*,\s*["']?([^"')]+)["']?\s*\)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "//") || line == "" {
			continue
		}
		if matches := reEnvSet.FindStringSubmatch(line); len(matches) == 3 {
			ctx.Environment[matches[1]] = matches[2]
		}
		if matches := reVarSet.FindStringSubmatch(line); len(matches) == 3 {
			ctx.Variables[matches[1]] = matches[2]
		}
	}
	return nil
}

// RunTestsScript 执行后置测试脚本，解析并执行 pm.test 断言块。
func (ctx *ScriptContext) RunTestsScript(script string) []domain.TestResult {
	if strings.TrimSpace(script) == "" {
		return nil
	}

	// 提取 pm.environment.set 赋值
	reEnvSet := regexp.MustCompile(`pm\.environment\.set\(\s*["']([^"']+)["']\s*,\s*([^)]+)\s*\)`)
	for _, matches := range reEnvSet.FindAllStringSubmatch(script, -1) {
		if len(matches) == 3 {
			key := matches[1]
			rawVal := strings.TrimSpace(matches[2])
			if (strings.HasPrefix(rawVal, "\"") && strings.HasSuffix(rawVal, "\"")) ||
				(strings.HasPrefix(rawVal, "'") && strings.HasSuffix(rawVal, "'")) {
				ctx.Environment[key] = rawVal[1 : len(rawVal)-1]
			} else if strings.HasPrefix(rawVal, "jsonData.") || strings.HasPrefix(rawVal, "pm.response.json().") {
				jsonPath := strings.TrimPrefix(rawVal, "jsonData.")
				jsonPath = strings.TrimPrefix(jsonPath, "pm.response.json().")
				if val, err := extractJSONField(ctx.BodyString, jsonPath); err == nil {
					ctx.Environment[key] = val
				}
			} else {
				ctx.Environment[key] = rawVal
			}
		}
	}

	// 提取 pm.variables.set 赋值
	reVarSet := regexp.MustCompile(`pm\.variables\.set\(\s*["']([^"']+)["']\s*,\s*([^)]+)\s*\)`)
	for _, matches := range reVarSet.FindAllStringSubmatch(script, -1) {
		if len(matches) == 3 {
			key := matches[1]
			rawVal := strings.TrimSpace(matches[2])
			if (strings.HasPrefix(rawVal, "\"") && strings.HasSuffix(rawVal, "\"")) ||
				(strings.HasPrefix(rawVal, "'") && strings.HasSuffix(rawVal, "'")) {
				ctx.Variables[key] = rawVal[1 : len(rawVal)-1]
			} else if strings.HasPrefix(rawVal, "jsonData.") || strings.HasPrefix(rawVal, "pm.response.json().") {
				jsonPath := strings.TrimPrefix(rawVal, "jsonData.")
				jsonPath = strings.TrimPrefix(jsonPath, "pm.response.json().")
				if val, err := extractJSONField(ctx.BodyString, jsonPath); err == nil {
					ctx.Variables[key] = val
				}
			} else {
				ctx.Variables[key] = rawVal
			}
		}
	}

	// 匹配 pm.test("...", function() { ... })
	reTest := regexp.MustCompile(`pm\.test\(\s*["']([^"']+)["']\s*,\s*function\s*\(\s*\)\s*\{([^}]+)\}\s*\)`)
	matches := reTest.FindAllStringSubmatch(script, -1)

	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		testName := m[1]
		testBody := m[2]

		passed, errStr := ctx.evalTestBody(testBody)
		ctx.TestResults = append(ctx.TestResults, domain.TestResult{
			Name:   testName,
			Passed: passed,
			Error:  errStr,
		})
	}

	return ctx.TestResults
}

// evalTestBody 评估单个测试块内的断言
func (ctx *ScriptContext) evalTestBody(body string) (bool, string) {
	lines := strings.Split(body, ";")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// 1. pm.response.to.have.status(code)
		reStatus := regexp.MustCompile(`pm\.response\.to\.have\.status\(\s*(\d+)\s*\)`)
		if m := reStatus.FindStringSubmatch(line); len(m) == 2 {
			wantStatus, _ := strconv.Atoi(m[1])
			if ctx.StatusCode != wantStatus {
				return false, fmt.Sprintf("期望状态码 %d，但实际为 %d", wantStatus, ctx.StatusCode)
			}
		}

		// 2. pm.response.to.be.ok
		if strings.Contains(line, "pm.response.to.be.ok") {
			if ctx.StatusCode < 200 || ctx.StatusCode >= 300 {
				return false, fmt.Sprintf("期望 2xx OK 响应，但状态码为 %d", ctx.StatusCode)
			}
		}

		// 3. pm.expect(pm.response.responseTime).to.be.below(ms) 或 pm.response.to.have.responseTime.below(ms)
		reTime := regexp.MustCompile(`(?:pm\.expect\(pm\.response\.responseTime\)\.to\.be\.below|pm\.response\.to\.have\.responseTime\.below)\(\s*(\d+)\s*\)`)
		if m := reTime.FindStringSubmatch(line); len(m) == 2 {
			limit, _ := strconv.Atoi(m[1])
			if ctx.DurationMS >= limit {
				return false, fmt.Sprintf("响应耗时 (%d ms) 超出限制 (%d ms)", ctx.DurationMS, limit)
			}
		}

		// 4. pm.response.to.have.header("header-name")
		reHeader := regexp.MustCompile(`pm\.response\.to\.have\.header\(\s*["']([^"']+)["']\s*\)`)
		if m := reHeader.FindStringSubmatch(line); len(m) == 2 {
			hName := strings.ToLower(m[1])
			if _, ok := ctx.Headers[hName]; !ok {
				return false, fmt.Sprintf("响应头未包含 %s", m[1])
			}
		}

		// 5. pm.response.to.have.body("...") / include("...")
		reBodyInc := regexp.MustCompile(`pm\.expect\(pm\.response\.text\(\)\)\.to\.include\(\s*["']([^"']+)["']\s*\)`)
		if m := reBodyInc.FindStringSubmatch(line); len(m) == 2 {
			if !strings.Contains(ctx.BodyString, m[1]) {
				return false, fmt.Sprintf("响应体未包含子串: %s", m[1])
			}
		}

		// 6. JSON 字段断言: pm.expect(jsonData.xxx 或 pm.response.json().xxx).to.eq/equal/eql(val)
		reJsonEq := regexp.MustCompile(`pm\.expect\((?:jsonData|pm\.response\.json\(\))\.([a-zA-Z0-9_.]+)\)\.to\.(?:eq|equal|eql)\(\s*["']?([^"')]+)["']?\s*\)`)
		if m := reJsonEq.FindStringSubmatch(line); len(m) == 3 {
			keyPath, wantVal := m[1], m[2]
			actualVal, err := extractJSONField(ctx.BodyString, keyPath)
			if err != nil {
				return false, fmt.Sprintf("解析 JSON 字段 %s 失败: %v", keyPath, err)
			}
			if actualVal != wantVal {
				return false, fmt.Sprintf("JSON 字段 %s 期望为 %s，实际为 %s", keyPath, wantVal, actualVal)
			}
		}
	}
	return true, ""
}

func extractJSONField(body, path string) (string, error) {
	var val any
	if err := json.Unmarshal([]byte(body), &val); err != nil {
		return "", err
	}
	parts := strings.Split(path, ".")
	cur := val
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", fmt.Errorf("节点不是 JSON Object")
		}
		cur = m[p]
	}
	if cur == nil {
		return "", fmt.Errorf("字段不存在")
	}
	return fmt.Sprint(cur), nil
}
