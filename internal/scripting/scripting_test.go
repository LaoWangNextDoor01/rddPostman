package scripting

import (
	"testing"

	"rddpostman/internal/domain"
)

func TestPreRequestScript(t *testing.T) {
	ctx := NewScriptContext(200, "OK", 50, nil, "", nil)
	script := `
pm.environment.set("token", "secret-token-123");
pm.variables.set("timestamp", "1600000000");
`
	err := ctx.RunPreRequestScript(script)
	if err != nil {
		t.Fatalf("RunPreRequestScript err: %v", err)
	}
	if ctx.Environment["token"] != "secret-token-123" {
		t.Errorf("expected token secret-token-123, got %s", ctx.Environment["token"])
	}
	if ctx.Variables["timestamp"] != "1600000000" {
		t.Errorf("expected timestamp 1600000000, got %s", ctx.Variables["timestamp"])
	}
}

func TestTestsScriptAssertions(t *testing.T) {
	headers := []domain.KV{
		{Key: "Content-Type", Value: "application/json"},
	}
	body := `{"code":0,"data":{"userId":"u-888"}}`
	ctx := NewScriptContext(200, "OK", 80, headers, body, nil)

	script := `
pm.test("Status code is 200", function () {
    pm.response.to.have.status(200);
});

pm.test("Response is OK", function () {
    pm.response.to.be.ok;
});

pm.test("Response time is acceptable", function () {
    pm.expect(pm.response.responseTime).to.be.below(200);
});

pm.test("Has JSON Content-Type", function () {
    pm.response.to.have.header("Content-Type");
});

pm.test("User ID is u-888", function () {
    pm.expect(jsonData.data.userId).to.eq("u-888");
});

pm.test("Should Fail Status", function () {
    pm.response.to.have.status(404);
});
`
	results := ctx.RunTestsScript(script)
	if len(results) != 6 {
		t.Fatalf("expected 6 test results, got %d", len(results))
	}

	for i := 0; i < 5; i++ {
		if !results[i].Passed {
			t.Errorf("test %s failed unexpectedly: %s", results[i].Name, results[i].Error)
		}
	}

	// 最后一个应该失败
	if results[5].Passed {
		t.Errorf("expected test 5 to fail")
	}
}
