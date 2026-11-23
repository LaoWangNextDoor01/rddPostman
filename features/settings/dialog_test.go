package settings

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSettingsDialogTabsAndThemeSwitch(t *testing.T) {
	open := true
	mode := ThemeSystem

	view := func(c *ui.Context) {
		Dialog(c, &open, DialogOptions{
			ThemeMode: mode,
			OnThemeMode: func(next ThemeMode) {
				mode = next
			},
			DataDir: "/tmp/rddpostman-data",
		})
	}

	tt := ui.NewTester(view, 1000, 700)
	if !tt.HasText("General") || !tt.HasText("Themes") || !tt.HasText("Shortcuts") {
		t.Fatalf("expected settings navigation tabs; got %q", tt.Texts())
	}

	if err := tt.Click("Themes"); err != nil {
		t.Fatalf("Click Themes: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Dark"); err != nil {
		t.Fatalf("Click Dark: %v", err)
	}
	tt.Frame()
	if mode != ThemeDark {
		t.Fatalf("mode = %q, want %q", mode, ThemeDark)
	}

	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatalf("Click Shortcuts: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Send Request") || !tt.HasText("Close Tab") {
		t.Fatalf("expected shortcut list entries; got %q", tt.Texts())
	}

	if err := tt.Click("Data"); err != nil {
		t.Fatalf("Click Data: %v", err)
	}
	tt.Frame()
	if !tt.HasText("/tmp/rddpostman-data") {
		t.Fatalf("expected DataDir displayed in Data tab; got %q", tt.Texts())
	}

	if err := tt.Click("Close settings"); err != nil {
		t.Fatalf("Click Close settings: %v", err)
	}
	tt.Frame()
	if open {
		t.Fatal("expected settings dialog closed after clicking close button")
	}
}
