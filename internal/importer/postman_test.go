package importer

import (
	"testing"

	"rddpostman/internal/domain"
)

func TestParsePostmanCollection(t *testing.T) {
	data := []byte(`{
  "info": {
    "name": "Test Postman Collection",
    "description": "Integration test",
    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
  },
  "variable": [
    { "key": "baseUrl", "value": "https://httpbin.org" }
  ],
  "item": [
    {
      "name": "User Module",
      "description": "Folder for users",
      "item": [
        {
          "name": "Get Profile",
          "request": {
            "method": "GET",
            "header": [
              { "key": "Accept", "value": "application/json" }
            ],
            "url": {
              "raw": "https://httpbin.org/get?id=123",
              "query": [
                { "key": "id", "value": "123" }
              ]
            },
            "auth": {
              "type": "bearer",
              "bearer": [
                { "key": "token", "value": "jwt-token-xyz" }
              ]
            }
          }
        }
      ]
    },
    {
      "name": "Create Post",
      "request": {
        "method": "POST",
        "url": "https://httpbin.org/post",
        "body": {
          "mode": "raw",
          "raw": "{\"title\":\"Hello World\"}",
          "options": {
            "raw": { "language": "json" }
          }
        }
      }
    }
  ]
}`)

	coll, err := ParsePostmanCollection(data)
	if err != nil {
		t.Fatalf("ParsePostmanCollection error: %v", err)
	}

	if coll.Name != "Test Postman Collection" {
		t.Errorf("expected name Test Postman Collection, got %s", coll.Name)
	}
	if len(coll.Variables) != 1 || coll.Variables[0].Key != "baseUrl" || coll.Variables[0].Value != "https://httpbin.org" {
		t.Errorf("variable baseUrl mismatch: %+v", coll.Variables)
	}
	if len(coll.Items) != 2 {
		t.Fatalf("expected 2 root items, got %d", len(coll.Items))
	}

	// Folder item
	folder := coll.Items[0]
	if folder.Kind != domain.KindFolder || folder.Name != "User Module" {
		t.Errorf("expected folder User Module, got %+v", folder)
	}
	if len(folder.Children) != 1 {
		t.Fatalf("expected 1 child in folder, got %d", len(folder.Children))
	}
	reqNode := folder.Children[0]
	if reqNode.Kind != domain.KindRequest || reqNode.Name != "Get Profile" {
		t.Errorf("expected request Get Profile, got %+v", reqNode)
	}
	if reqNode.Request.Method != "GET" || reqNode.Auth.Type != domain.AuthBearer {
		t.Errorf("expected GET with Bearer auth, got %+v", reqNode.Request)
	}

	// Root Request item
	postReq := coll.Items[1]
	if postReq.Kind != domain.KindRequest || postReq.Name != "Create Post" {
		t.Errorf("expected Create Post, got %+v", postReq)
	}
	if postReq.Request.Body.Mode != domain.BodyRaw || postReq.Request.Body.Raw != `{"title":"Hello World"}` {
		t.Errorf("body mismatch: %+v", postReq.Request.Body)
	}

	// 2. 测试从 Collection 重新导出为 Postman Collection JSON
	exportedData, err := ExportPostmanCollection(coll)
	if err != nil {
		t.Fatalf("ExportPostmanCollection error: %v", err)
	}
	reParsed, err := ParsePostmanCollection(exportedData)
	if err != nil {
		t.Fatalf("re-parse exported collection error: %v", err)
	}
	if reParsed.Name != coll.Name || len(reParsed.Items) != len(coll.Items) {
		t.Errorf("exported and re-parsed collection mismatch: %+v", reParsed)
	}
}

func TestPostmanEnvironmentImportExport(t *testing.T) {
	rawJSON := []byte(`{
  "id": "env-123",
  "name": "Production Env",
  "values": [
    { "key": "host", "value": "https://api.prod.com", "enabled": true },
    { "key": "secret_key", "value": "sec_val_999", "type": "secret", "enabled": true },
    { "key": "disabled_var", "value": "unused", "enabled": false }
  ],
  "_postman_variable_scope": "environment"
}`)

	env, err := ParsePostmanEnvironment(rawJSON)
	if err != nil {
		t.Fatalf("ParsePostmanEnvironment error: %v", err)
	}
	if env.Name != "Production Env" {
		t.Errorf("expected name Production Env, got %s", env.Name)
	}
	if len(env.Vars) != 3 {
		t.Fatalf("expected 3 vars, got %d", len(env.Vars))
	}
	if !env.Vars[1].Secret || env.Vars[1].Key != "secret_key" {
		t.Errorf("expected secret var at index 1: %+v", env.Vars[1])
	}

	exported, err := ExportPostmanEnvironment(env)
	if err != nil {
		t.Fatalf("ExportPostmanEnvironment error: %v", err)
	}
	reParsedEnv, err := ParsePostmanEnvironment(exported)
	if err != nil {
		t.Fatalf("re-parse exported environment error: %v", err)
	}
	if reParsedEnv.Name != env.Name || len(reParsedEnv.Vars) != len(env.Vars) {
		t.Errorf("mismatch between original and re-parsed env: %+v", reParsedEnv)
	}
}
