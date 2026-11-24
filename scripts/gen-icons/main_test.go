package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPascal 验证 kebab → Pascal 的转换。
func TestPascal(t *testing.T) {
	cases := map[string]string{
		"user-round":         "UserRound",
		"sliders-horizontal": "SlidersHorizontal",
		"x":                  "X",
		"square-terminal":    "SquareTerminal",
	}
	for in, want := range cases {
		if got := pascal(in); got != want {
			t.Errorf("pascal(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestResolveVarName 验证保留字冲突时自动加 Icon 后缀。
func TestResolveVarName(t *testing.T) {
	if name, renamed := resolveVarName("settings"); name != "Settings" || renamed {
		t.Errorf("resolveVarName(settings) = %q, renamed=%v", name, renamed)
	}
	if name, renamed := resolveVarName("get"); name != "GetIcon" || !renamed {
		t.Errorf(`resolveVarName(get) 应为 GetIcon 且 renamed=true，得到 %q, %v`, name, renamed)
	}
	if name, _ := resolveVarName("names"); name != "NamesIcon" {
		t.Errorf("resolveVarName(names) = %q, want NamesIcon", name)
	}
}

func TestParseIconManifest(t *testing.T) {
	raw := `
# comment
send = plain-linear
layers-bold-duotone
  search = magnifier-outline  
`
	got, err := parseIconManifest(raw)
	if err != nil {
		t.Fatalf("parseIconManifest failed: %v", err)
	}
	want := []iconEntry{
		{Local: "send", Solar: "plain-linear"},
		{Local: "layers-bold-duotone", Solar: "layers-bold-duotone"},
		{Local: "search", Solar: "magnifier-outline"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	if _, err := parseIconManifest("send = plain-linear\nsend = plain-outline\n"); err == nil {
		t.Fatalf("expected error on duplicate local icon name")
	}
}

func TestBuildIconSVGAndAlias(t *testing.T) {
	set := &iconifySet{
		Prefix: "solar",
		Width:  24,
		Height: 24,
		Icons: map[string]iconifyIcon{
			"plain-linear": {Body: `<path d="M0 0"/>`},
		},
		Aliases: map[string]iconifyAliasEntry{
			"plain-alias": {Parent: "plain-linear"},
		},
	}
	got, err := buildIconSVG(set, "plain-alias")
	if err != nil {
		t.Fatalf("buildIconSVG alias failed: %v", err)
	}
	want := `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path d="M0 0"/></svg>`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := buildIconSVG(set, "nonexistent"); err == nil {
		t.Fatalf("expected error for missing icon")
	}
}

// TestManifestMatchesAssets 验证 scripts/icons.txt 与 components/v1/icons/assets/*.svg 严格一致。
func TestManifestMatchesAssets(t *testing.T) {
	root := filepath.Join("..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "icons.txt"))
	if err != nil {
		t.Fatalf("read icons.txt: %v", err)
	}
	entries, err := parseIconManifest(string(raw))
	if err != nil {
		t.Fatalf("parse icons.txt: %v", err)
	}
	want := map[string]bool{}
	for _, e := range entries {
		want[e.Local] = true
	}

	files, err := os.ReadDir(filepath.Join(root, "components", "v1", "icons", "assets"))
	if err != nil {
		t.Fatalf("read assets: %v", err)
	}
	got := map[string]bool{}
	for _, f := range files {
		if name, ok := strings.CutSuffix(f.Name(), ".svg"); ok {
			got[name] = true
		}
	}
	for k := range want {
		if !got[k] {
			t.Errorf("icons.txt 包含 %q，但 assets/%s.svg 不存在", k, k)
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("assets/%s.svg 存在，但未登记在 scripts/icons.txt 中", k)
		}
	}
}
