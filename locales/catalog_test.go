package locales

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

func loadCatalogMap(t *testing.T, filename string) map[string]string {
	t.Helper()
	raw, err := files.ReadFile(filename)
	if err != nil {
		t.Fatalf("read %s: %v", filename, err)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", filename, err)
	}
	return m
}

func TestLocaleCatalogParity(t *testing.T) {
	en := loadCatalogMap(t, "en.json")
	zh := loadCatalogMap(t, "zh-Hans.json")

	var missingInZh, extraInZh, emptyVals []string
	for k, v := range en {
		if k == "@@locale" {
			continue
		}
		if strings.TrimSpace(v) == "" {
			emptyVals = append(emptyVals, "en:"+k)
		}
		zv, ok := zh[k]
		if !ok {
			missingInZh = append(missingInZh, k)
		} else if strings.TrimSpace(zv) == "" {
			emptyVals = append(emptyVals, "zh-Hans:"+k)
		}
	}
	for k := range zh {
		if k == "@@locale" {
			continue
		}
		if _, ok := en[k]; !ok {
			extraInZh = append(extraInZh, k)
		}
	}

	sort.Strings(missingInZh)
	sort.Strings(extraInZh)
	sort.Strings(emptyVals)

	if len(missingInZh) > 0 {
		t.Errorf("keys missing in zh-Hans.json (%d): %v", len(missingInZh), missingInZh)
	}
	if len(extraInZh) > 0 {
		t.Errorf("extra keys in zh-Hans.json not in en.json (%d): %v", len(extraInZh), extraInZh)
	}
	if len(emptyVals) > 0 {
		t.Errorf("empty translation values (%d): %v", len(emptyVals), emptyVals)
	}
}

func TestLiveLocaleSwitch(t *testing.T) {
	en := In("en")
	if got := en.SettingsTitle(); got != "Settings" {
		t.Fatalf("en SettingsTitle() = %q, want %q", got, "Settings")
	}
	if got := en.ToolJSONName(); got != "JSON Format / Escape" {
		t.Fatalf("en ToolJSONName() = %q, want %q", got, "JSON Format / Escape")
	}
	if got := en.RunnerDoneHint(2); got != "Completed: 2 requests" {
		t.Fatalf("en RunnerDoneHint(2) = %q, want %q", got, "Completed: 2 requests")
	}

	zh := In("zh-Hans")
	if got := zh.SettingsTitle(); got != "设置" {
		t.Fatalf("zh-Hans SettingsTitle() = %q, want %q", got, "设置")
	}
	if got := zh.ToolJSONName(); got != "JSON 格式化 / 转义" {
		t.Fatalf("zh-Hans ToolJSONName() = %q, want %q", got, "JSON 格式化 / 转义")
	}
	if got := zh.RunnerDoneHint(2); got != "执行完成：共 2 个请求" {
		t.Fatalf("zh-Hans RunnerDoneHint(2) = %q, want %q", got, "执行完成：共 2 个请求")
	}
}
