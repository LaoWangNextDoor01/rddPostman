package transport

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rddpostman/internal/domain"
)

func TestTransportSendGET(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("expected page=1, got %s", r.URL.Query().Get("page"))
		}
		if r.Header.Get("X-Custom") != "test-val" {
			t.Errorf("expected X-Custom header")
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "12345"})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","count":42}`))
	}))
	defer ts.Close()

	req := domain.Request{
		Method: "GET",
		URL:    ts.URL,
		Query: []domain.KV{
			{Key: "page", Value: "1", Enabled: true},
			{Key: "unused", Value: "foo", Enabled: false},
		},
		Headers: []domain.KV{
			{Key: "X-Custom", Value: "test-val", Enabled: true},
		},
	}

	resp, err := Send(context.Background(), req, domain.Auth{})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.FormattedBody, "count") {
		t.Fatalf("expected body to contain count, got %s", resp.FormattedBody)
	}
	if len(resp.Cookies) != 1 || resp.Cookies[0].Name != "session" {
		t.Fatalf("expected cookie session, got %+v", resp.Cookies)
	}
	if resp.Duration <= 0 {
		t.Fatalf("expected positive duration, got %v", resp.Duration)
	}
}

func TestTransportSendPOSTWithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("invalid json body: %v", err)
		}
		if payload["name"] != "postman" {
			t.Errorf("expected name=postman, got %v", payload["name"])
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"created":true}`))
	}))
	defer ts.Close()

	req := domain.Request{
		Method: "POST",
		URL:    ts.URL + "/create",
		Body: domain.Body{
			Mode: domain.BodyRaw,
			Raw:  `{"name":"postman"}`,
		},
	}

	resp, err := Send(context.Background(), req, domain.Auth{})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.FormattedBody, "created") {
		t.Fatalf("unexpected body: %s", resp.FormattedBody)
	}
}

func TestTransportApplyAuth(t *testing.T) {
	// 1. Bearer Token
	reqBearer, _ := http.NewRequest("GET", "http://example.com", nil)
	ApplyAuth(reqBearer, domain.Auth{
		Type: domain.AuthBearer,
		Params: []domain.KV{
			{Key: "token", Value: "sec-token-123"},
		},
	})
	if reqBearer.Header.Get("Authorization") != "Bearer sec-token-123" {
		t.Fatalf("expected Bearer token, got %s", reqBearer.Header.Get("Authorization"))
	}

	// 2. Basic Auth
	reqBasic, _ := http.NewRequest("GET", "http://example.com", nil)
	ApplyAuth(reqBasic, domain.Auth{
		Type: domain.AuthBasic,
		Params: []domain.KV{
			{Key: "username", Value: "admin"},
			{Key: "password", Value: "secret"},
		},
	})
	u, p, ok := reqBasic.BasicAuth()
	if !ok || u != "admin" || p != "secret" {
		t.Fatalf("expected basic auth admin:secret, got %s:%s (ok=%v)", u, p, ok)
	}

	// 3. API Key in Header
	reqAPIKey, _ := http.NewRequest("GET", "http://example.com", nil)
	ApplyAuth(reqAPIKey, domain.Auth{
		Type: domain.AuthAPIKey,
		Params: []domain.KV{
			{Key: "key", Value: "X-API-KEY"},
			{Key: "value", Value: "key-999"},
			{Key: "in", Value: "header"},
		},
	})
	if reqAPIKey.Header.Get("X-API-KEY") != "key-999" {
		t.Fatalf("expected API Key in header, got %s", reqAPIKey.Header.Get("X-API-KEY"))
	}

	// 4. API Key in Query
	reqAPIKeyQuery, _ := http.NewRequest("GET", "http://example.com/api?a=1", nil)
	ApplyAuth(reqAPIKeyQuery, domain.Auth{
		Type: domain.AuthAPIKey,
		Params: []domain.KV{
			{Key: "key", Value: "token"},
			{Key: "value", Value: "xyz"},
			{Key: "in", Value: "query"},
		},
	})
	if reqAPIKeyQuery.URL.Query().Get("token") != "xyz" {
		t.Fatalf("expected API Key in query, got %s", reqAPIKeyQuery.URL.RawQuery)
	}
}

func TestFormatHelpers(t *testing.T) {
	if FormatDuration(50*time.Microsecond) != "50 µs" {
		t.Errorf("FormatDuration microsecond mismatch")
	}
	if FormatDuration(142*time.Millisecond) != "142 ms" {
		t.Errorf("FormatDuration millisecond mismatch")
	}
	if FormatDuration(1500*time.Millisecond) != "1.50 s" {
		t.Errorf("FormatDuration second mismatch")
	}

	if FormatSize(512) != "512 B" {
		t.Errorf("FormatSize B mismatch")
	}
	if FormatSize(1536) != "1.50 KB" {
		t.Errorf("FormatSize KB mismatch")
	}
}
