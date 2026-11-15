package main

import (
	"os"
	"testing"

	"rddpostman/internal/app"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// TestMain 把应用数据目录指到临时目录，测试不碰真实数据。
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "rddpostman-test-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("RDDPOSTMAN_DATA_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// The view runs without a window in tests, which click and type as a user
// would.
func TestView(t *testing.T) {
	a := app.New()
	tt := ui.NewTester(a.View, 1280, 800)
	if !tt.HasText(locales.SwitchWorkspace()) {
		t.Errorf("texts %q: the shell shows no workspace selector", tt.Texts())
	}
	if err := tt.Click(locales.RailHistory()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.HistorySubtitle()) {
		t.Errorf("texts %q: the router did not show the history page", tt.Texts())
	}
}
