package history

import (
	"testing"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/store"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

func TestHistorySidebarGroupingOpenAndClear(t *testing.T) {
	db, err := store.OpenAt(t.TempDir())
	if err != nil {
		t.Fatalf("store.OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ws := db.ActiveWorkspace()
	now := time.Now()
	_ = db.RecordHistory(ws.ID, domain.HistoryEntry{
		WorkspaceID: ws.ID,
		RequestID:   "req-today",
		Method:      "POST",
		URL:         "https://api.example.com/v1/today",
		Status:      201,
		DurationMS:  18,
		At:          now,
	})
	_ = db.RecordHistory(ws.ID, domain.HistoryEntry{
		WorkspaceID: ws.ID,
		RequestID:   "req-yest",
		Method:      "GET",
		URL:         "https://api.example.com/v1/yesterday",
		Status:      200,
		DurationMS:  25,
		At:          now.AddDate(0, 0, -1),
	})

	var openedID string
	view := func(c *ui.Context) {
		SidebarView(c, db, ws.ID, func(nodeID string) {
			openedID = nodeID
		})
	}

	tt := ui.NewTester(view, 320, 640)
	if !tt.HasText("Today") || !tt.HasText("Yesterday") {
		t.Fatalf("expected Today and Yesterday groups; got %q", tt.Texts())
	}
	if err := tt.Click("POST api.example.com/v1/today"); err != nil {
		t.Fatalf("Click history item: %v", err)
	}
	tt.Frame()
	if openedID != "req-today" {
		t.Fatalf("openedID = %q, want req-today", openedID)
	}

	if err := tt.Click(locales.HistClear()); err != nil {
		t.Fatalf("Click Clear history: %v", err)
	}
	tt.Frame()
	if !tt.HasText(locales.HistClearTitle()) || !tt.HasText(locales.HistToday()) {
		t.Fatalf("expected clear history confirm dialog before clearing; got %q", tt.Texts())
	}
	if err := tt.Click(locales.ConfirmClear()); err != nil {
		t.Fatalf("Click confirm clear history: %v", err)
	}
	tt.Frame()
	if !tt.HasText(locales.HistEmptyTitle()) {
		t.Fatalf("expected empty history state after clearing; got %q", tt.Texts())
	}
}
