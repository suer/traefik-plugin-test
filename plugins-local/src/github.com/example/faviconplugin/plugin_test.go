package faviconplugin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/example/faviconplugin"
)

func writeFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func newHandler(t *testing.T) http.Handler {
	t.Helper()

	cfg := faviconplugin.CreateConfig()
	cfg.Rules = []faviconplugin.Rule{
		{Regex: `^https?://example\.com/favicon\.ico$`, FilePath: writeFile(t, "favicon.ico", "ICON"), ContentType: "image/x-icon"},
		{Regex: `^https?://[^/]+/robots\.txt$`, FilePath: writeFile(t, "robots.txt", "ROBOTS")},
		{Regex: `^http://catchall\.example\.com/`, FilePath: writeFile(t, "all.txt", "CATCHALL")},
	}

	next := http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		_, _ = rw.Write([]byte("next"))
	})

	h, err := faviconplugin.New(context.Background(), next, cfg, "static")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestServeHTTP(t *testing.T) {
	tests := []struct {
		name, method, target, wantBody, wantType string
	}{
		{"first rule", http.MethodGet, "http://example.com/favicon.ico", "ICON", "image/x-icon"},
		{"https", http.MethodGet, "https://example.com/favicon.ico", "ICON", "image/x-icon"},
		{"query is part of url", http.MethodGet, "http://example.com/favicon.ico?v=1", "next", ""},
		{"other domain", http.MethodGet, "http://other.example.org/favicon.ico", "next", ""},
		{"any domain via regex", http.MethodGet, "http://other.example.org/robots.txt", "ROBOTS", "text/plain; charset=utf-8"},
		{"head has no body", http.MethodHead, "http://example.com/favicon.ico", "", "image/x-icon"},
		{"second rule with type by extension", http.MethodGet, "http://example.com/robots.txt", "ROBOTS", "text/plain; charset=utf-8"},
		{"catch-all only for its domain", http.MethodGet, "http://catchall.example.com/favicon.ico", "CATCHALL", "text/plain; charset=utf-8"},
		{"catch-all", http.MethodGet, "http://catchall.example.com/foo", "CATCHALL", "text/plain; charset=utf-8"},
		{"post passes through", http.MethodPost, "http://example.com/favicon.ico", "next", ""},
	}

	h := newHandler(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.target, nil))

			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if got := rec.Header().Get("Content-Type"); tt.wantType != "" && got != tt.wantType {
				t.Errorf("content-type = %q, want %q", got, tt.wantType)
			}
		})
	}
}

func TestNoMatchPassesThrough(t *testing.T) {
	cfg := faviconplugin.CreateConfig()
	cfg.Rules = []faviconplugin.Rule{
		{Regex: `^http://example\.com/favicon\.ico$`, FilePath: writeFile(t, "favicon.ico", "ICON")},
	}
	next := http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		_, _ = rw.Write([]byte("next"))
	})

	h, err := faviconplugin.New(context.Background(), next, cfg, "static")
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://example.com/foo/favicon.ico", nil))
	if got := rec.Body.String(); got != "next" {
		t.Errorf("body = %q, want next", got)
	}
}

func TestNewErrors(t *testing.T) {
	file := writeFile(t, "a.txt", "A")

	tests := map[string][]faviconplugin.Rule{
		"no rules":      nil,
		"empty regex":   {{FilePath: file}},
		"empty file":    {{Regex: `^/a$`}},
		"invalid regex": {{Regex: `(`, FilePath: file}},
		"missing file":  {{Regex: `^/a$`, FilePath: "/nonexistent"}},
	}

	for name, rules := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := faviconplugin.CreateConfig()
			cfg.Rules = rules
			if _, err := faviconplugin.New(context.Background(), nil, cfg, "static"); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
