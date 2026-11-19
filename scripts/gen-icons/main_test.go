package main

import "testing"

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
