package collection

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/transport"

	"github.com/egoist/mygo/ui"
)

func TestURLVariableParsingAndEvaluation(t *testing.T) {
	vars := map[string]domain.Variable{
		"baseUrl": {Key: "baseUrl", Value: "https://api.example.com", Source: domain.VarEnvironment},
		"token":   {Key: "token", Value: "secret_12345", Source: domain.VarCollection, Secret: true},
	}

	// 1. 测试解析变量（包含环境变量、密文字态、内置动态变量与未解析变量）
	rawURL := "{{baseUrl}}/users?token={{token}}&id={{$guid}}&unknown={{missingVar}}"
	extracted := extractURLVariables(rawURL, vars)
	if len(extracted) != 4 {
		t.Fatalf("extracted len = %d, want 4", len(extracted))
	}

	m := make(map[string]URLVarInfo)
	for _, it := range extracted {
		m[it.Name] = it
	}

	if !m["baseUrl"].Resolved || m["baseUrl"].Value != "https://api.example.com" || m["baseUrl"].Source != "environment" {
		t.Fatalf("unexpected baseUrl info: %+v", m["baseUrl"])
	}
	if !m["token"].Resolved || !m["token"].Secret || m["token"].Source != "collection" {
		t.Fatalf("unexpected token info: %+v", m["token"])
	}
	if !m["$guid"].Resolved || m["$guid"].Source != "dynamic" || len(m["$guid"].Value) < 10 {
		t.Fatalf("unexpected $guid dynamic info: %+v", m["$guid"])
	}
	if m["missingVar"].Resolved || m["missingVar"].Source != "unresolved" {
		t.Fatalf("unexpected missingVar info: %+v", m["missingVar"])
	}

	// 2. 测试 evaluateURL 实时求值与未解析统计
	evalResult, missing := evaluateURL(rawURL, vars)
	if len(missing) != 1 || missing[0] != "missingVar" {
		t.Fatalf("missing = %v, want [missingVar]", missing)
	}
	if !strings.HasPrefix(evalResult, "https://api.example.com/users?token=secret_12345&id=") {
		t.Fatalf("evalResult unexpected: %s", evalResult)
	}
	if !strings.Contains(evalResult, "&unknown={{missingVar}}") {
		t.Fatalf("missing var should keep placeholder in evalResult: %s", evalResult)
	}

	// 3. 测试 allAvailableVariables 聚合排序
	allAvail := allAvailableVariables(vars)
	if len(allAvail) < 6 {
		t.Fatalf("allAvailableVariables len = %d, want >= 6", len(allAvail))
	}
	// 环境变量排首位
	if allAvail[0].Source != "environment" {
		t.Fatalf("allAvail[0].Source = %s, want environment", allAvail[0].Source)
	}

	// 4. 测试 insertVariableToURL 智能插入与补全
	state := &reqSessionState{}
	insertVariableToURL(state, "baseUrl")
	if state.urlDraft != "{{baseUrl}}" {
		t.Fatalf("empty insert = %q, want {{baseUrl}}", state.urlDraft)
	}
	insertVariableToURL(state, "users")
	if state.urlDraft != "{{baseUrl}}/{{users}}" {
		t.Fatalf("append insert = %q, want {{baseUrl}}/{{users}}", state.urlDraft)
	}

	state.urlDraft = "https://example.com/{{"
	insertVariableToURL(state, "param")
	if state.urlDraft != "https://example.com/{{param}}" {
		t.Fatalf("trailing {{ replace insert = %q", state.urlDraft)
	}
}

func TestURLSegmentsAndUnclosedDetection(t *testing.T) {
	vars := map[string]domain.Variable{
		"baseUrl": {Key: "baseUrl", Value: "https://api.example.com", Source: domain.VarEnvironment},
	}

	// 1. 测试 parseURLSegments
	rawURL := "{{baseUrl}}/api/v1/orders/{{orderId}}"
	segs := parseURLSegments(rawURL, vars)
	if len(segs) != 3 {
		t.Fatalf("segs len = %d, want 3", len(segs))
	}
	if segs[0].Kind != SegmentVariable || segs[0].Text != "baseUrl" || !segs[0].VarInfo.Resolved {
		t.Fatalf("seg[0] = %+v", segs[0])
	}
	if segs[1].Kind != SegmentLiteral || segs[1].Text != "/api/v1/orders/" {
		t.Fatalf("seg[1] = %+v", segs[1])
	}
	if segs[2].Kind != SegmentVariable || segs[2].Text != "orderId" || segs[2].VarInfo.Resolved {
		t.Fatalf("seg[2] (unresolved orderId) = %+v", segs[2])
	}

	// 2. 测试 detectUnclosedVariable
	p1, ok1 := detectUnclosedVariable("https://example.com")
	if ok1 || p1 != "" {
		t.Fatalf("expected no unclosed, got ok=%v, p=%q", ok1, p1)
	}

	p2, ok2 := detectUnclosedVariable("https://example.com/{{")
	if !ok2 || p2 != "" {
		t.Fatalf("expected unclosed empty prefix, got ok=%v, p=%q", ok2, p2)
	}

	p3, ok3 := detectUnclosedVariable("https://example.com/{{base")
	if !ok3 || p3 != "base" {
		t.Fatalf("expected unclosed prefix 'base', got ok=%v, p=%q", ok3, p3)
	}

	_, ok4 := detectUnclosedVariable("https://example.com/{{baseUrl}}")
	if ok4 {
		t.Fatalf("closed {{baseUrl}} should not be unclosed")
	}

	// 3. 测试 filterVariables
	allList := allAvailableVariables(vars)
	filtered := filterVariables(allList, "base")
	if len(filtered) == 0 || filtered[0].Name != "baseUrl" {
		t.Fatalf("filtered expected baseUrl, got %+v", filtered)
	}
}

func TestAddressBarVariableUI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	ws := newWorkspace()
	reqID := domain.ID("req-var-ui-test")
	collID := domain.ID("coll-var-ui-test")
	envID := domain.ID("env-dev")

	coll := &domain.Collection{
		ID:   collID,
		Name: "Variable Test Coll",
	}

	n := &domain.Node{
		ID:   reqID,
		Name: "Variable Test Request",
		Kind: domain.KindRequest,
		Request: &domain.Request{
			Method: "GET",
			URL:    "{{baseUrl}}/api/v1/users/{{userID}}",
		},
	}

	envVars := map[string]domain.Variable{
		"baseUrl": {Key: "baseUrl", Value: ts.URL, Source: domain.VarEnvironment},
		"apiKey":  {Key: "apiKey", Value: "secret_xyz", Source: domain.VarCollection, Secret: true},
	}

	props := pageProps{
		workspaceID:   "ws-1",
		workspaceName: "Default Workspace",
		collections:   []*domain.Collection{coll},
		environments: []domain.Environment{
			{ID: envID, Name: "Dev"},
		},
		selectedEnv: envID,
		state:       ws,
		actions: pageActions{
			resolveVariables: func(cID, nID domain.ID) map[string]domain.Variable {
				return envVars
			},
			sendRequest: func(cID, nID domain.ID, req domain.Request, auth domain.Auth, onDone func(*transport.Response)) func() {
				onDone(&transport.Response{StatusCode: 200, StatusText: "OK"})
				return func() {}
			},
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		requestTab(c, props, coll, n, string(reqID))
	}, 960, 600)

	// 1. 验证行内变量胶囊展示（画板 47）
	if !tt.HasText("{{baseUrl}}") || !tt.HasText("{{userID}}") {
		t.Fatalf("expected variable tokens in address bar, got texts: %q", tt.Texts())
	}

	// 2. 验证地址栏底部的已解析 URL 预览条已被彻底移除（用户明确要求）
	if tt.HasText("Resolved:") {
		t.Fatalf("expected NO 'Resolved:' row under address bar, got texts: %q", tt.Texts())
	}

	// 3. 验证多余的 {x} 按钮已被移除
	if tt.HasText("{x}") {
		t.Fatalf("expected NO '{x}' picker button in address bar, got texts: %q", tt.Texts())
	}

	// 4. 点击已解析变量胶囊 {{baseUrl}}，展开画板 48 名片浮层
	if err := tt.Click("{{baseUrl}}"); err != nil {
		t.Fatalf("Click {{baseUrl}} token pill: %v", err)
	}
	tt.Advance(100 * time.Millisecond)

	if !tt.HasText("CURRENT VALUE:") || !tt.HasText("ENVIRONMENT") {
		t.Fatalf("expected Hover/Click Popover to be opened for baseUrl, got: %q", tt.Texts())
	}

	// 5. 点击未定义变量胶囊 {{userID}}，展开画板 49 未定义警告浮层
	if err := tt.Click("{{userID}}"); err != nil {
		t.Fatalf("Click {{userID}} token pill: %v", err)
	}
	tt.Advance(100 * time.Millisecond)

	if !tt.HasText("UNRESOLVED") || !tt.HasText("Variable not defined in active environment.") {
		t.Fatalf("expected Unresolved Popover for userID, got: %q", tt.Texts())
	}
}

func TestVariableAutocompleteUI(t *testing.T) {
	ws := newWorkspace()
	reqID := domain.ID("req-auto-ui-test")
	collID := domain.ID("coll-auto-ui-test")
	envID := domain.ID("env-dev")

	coll := &domain.Collection{
		ID:   collID,
		Name: "Autocomplete Coll",
	}

	n := &domain.Node{
		ID:   reqID,
		Name: "Autocomplete Request",
		Kind: domain.KindRequest,
		Request: &domain.Request{
			Method: "GET",
			URL:    "https://api.example.com/{{",
		},
	}

	envVars := map[string]domain.Variable{
		"baseUrl": {Key: "baseUrl", Value: "https://api.dev.company.com", Source: domain.VarEnvironment},
		"orderId": {Key: "orderId", Value: "ORD-9021", Source: domain.VarEnvironment},
	}

	props := pageProps{
		workspaceID:   "ws-1",
		workspaceName: "Default Workspace",
		collections:   []*domain.Collection{coll},
		environments: []domain.Environment{
			{ID: envID, Name: "Dev"},
		},
		selectedEnv: envID,
		state:       ws,
		actions: pageActions{
			resolveVariables: func(cID, nID domain.ID) map[string]domain.Variable {
				return envVars
			},
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		requestTab(c, props, coll, n, string(reqID))
	}, 960, 600)

	// 验证未闭合 {{ 触发了自动补全下拉浮层（画板 50）
	if !tt.HasText("AVAILABLE VARIABLES") {
		t.Fatalf("expected autocomplete dropdown on unclosed {{, got texts: %q", tt.Texts())
	}

	// 验证包含环境变量候选项
	if !tt.HasText("baseUrl") || !tt.HasText("orderId") {
		t.Fatalf("expected baseUrl and orderId in autocomplete list, got: %q", tt.Texts())
	}

	// 点击 baseUrl 进行补全
	if err := tt.Click("baseUrl"); err != nil {
		t.Fatalf("Click baseUrl in autocomplete: %v", err)
	}
	tt.Advance(50 * time.Millisecond)

	// 验证补全后闭合了变量并呈现为变量胶囊
	if !tt.HasText("{{baseUrl}}") {
		t.Fatalf("expected completed variable token '{{baseUrl}}' in address bar, got texts: %q", tt.Texts())
	}

	// 验证标记为未保存
	if !ws.unsaved[string(reqID)] {
		t.Fatalf("expected tab marked unsaved after autocomplete completion")
	}
}
