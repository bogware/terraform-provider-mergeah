package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthenticationAndErrorRedaction(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-fixture" {
			t.Error("missing bearer auth")
		}
		if !strings.Contains(r.Header.Get("User-Agent"), "terraform-provider-mergeah") {
			t.Error("missing user agent")
		}
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"message":"secret-fixture"}}`))
	}))
	defer s.Close()
	c, _ := New(s.URL+"/api/v1", "secret-fixture", "test")
	_, err := c.Do(context.Background(), "POST", "tool-packs/", map[string]any{"name": "test"})
	if err == nil || strings.Contains(err.Error(), "secret-fixture") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestRequireHTTPSExceptLoopbackIP(t *testing.T) {
	for _, u := range []string{"http://ah-api.merge.dev/api/v1", "http://192.168.1.1/api/v1", "http://localhost/api/v1", "http://127.0.0.1.attacker.example/api/v1", "http://[::]/api/v1"} {
		if _, err := New(u, "key", "test"); err == nil {
			t.Errorf("accepted insecure URL %s", u)
		}
	}
	for _, u := range []string{DefaultURL, "https://proxy.example/api/v1", "http://127.0.0.1:1234/api/v1", "http://[::1]:1234/api/v1"} {
		if _, err := New(u, "key", "test"); err != nil {
			t.Errorf("rejected allowed URL %s: %v", u, err)
		}
	}
}

func TestRetryAfterBounds(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		header string
		want   time.Duration
	}{
		{"0", 0}, {"60", 60 * time.Second}, {"61", 61 * time.Second},
		{"9223372036854775807", 61 * time.Second},
		{"999999999999999999999999999999999999999", 61 * time.Second},
		{"-1", 2 * time.Second}, {"invalid", 2 * time.Second},
		{now.Add(-time.Minute).Format(http.TimeFormat), 0},
		{now.Add(30 * time.Second).Format(http.TimeFormat), 30 * time.Second},
	} {
		if got := retryDelay(tc.header, 1, now); got != tc.want {
			t.Errorf("retryDelay(%q)=%v; want %v", tc.header, got, tc.want)
		}
	}
	for _, header := range []string{"9223372036854775807", "99999999999999999999999999999", now.AddDate(100, 0, 0).Format(http.TimeFormat)} {
		t.Run(header, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Retry-After", header)
				w.WriteHeader(429)
			}))
			defer s.Close()
			c, _ := New(s.URL+"/api/v1", "fixture", "test")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := c.Do(ctx, http.MethodGet, "items/", nil)
			if _, ok := err.(*APIError); !ok || calls != 1 {
				t.Fatalf("oversized delay retried or waited: calls=%d err=%v", calls, err)
			}
		})
	}
}
func TestListPagination(t *testing.T) {
	for _, shape := range []string{"pages", "cursor", "array"} {
		t.Run(shape, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				switch shape {
				case "pages":
					if r.URL.Query().Get("page") == "2" {
						w.Write([]byte(`{"results":[{"id":"b"}],"next":null}`))
					} else {
						w.Write([]byte(`{"results":[{"id":"a"}],"next":"?page=2"}`))
					}
				case "cursor":
					if r.URL.Query().Get("cursor") == "next" {
						w.Write([]byte(`{"data":[{"id":"b"}],"has_more":false}`))
					} else {
						w.Write([]byte(`{"data":[{"id":"a"}],"has_more":true,"next_cursor":"next"}`))
					}
				case "array":
					w.Write([]byte(`[{"id":"a"},{"id":"b"}]`))
				}
			}))
			defer s.Close()
			c, _ := New(s.URL+"/api/v1", "key", "test")
			v, err := c.List(context.Background(), "things/")
			if err != nil || len(v) != 2 {
				t.Fatalf("items %v error %v", v, err)
			}
			if shape != "array" && calls != 2 {
				t.Fatalf("got %d requests", calls)
			}
		})
	}
}
func TestRejectUnsafePaginationAndLoops(t *testing.T) {
	for _, next := range []string{"https://attacker.example/api/v1/items/", "/outside/", "?loop=1"} {
		t.Run(next, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "next": next})
			}))
			defer s.Close()
			c, _ := New(s.URL+"/api/v1", "key", "test")
			if _, err := c.List(context.Background(), "items/"); err == nil {
				t.Fatal("expected unsafe or looping pagination error")
			}
		})
	}
}
func TestRetriesOnlyReads(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PATCH", "PUT", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Retry-After", "0")
				if calls == 1 {
					w.WriteHeader(429)
				} else {
					w.Write([]byte(`{}`))
				}
			}))
			defer s.Close()
			c, _ := New(s.URL+"/api/v1", "key", "test")
			_, err := c.Do(context.Background(), method, "items/", nil)
			if method == "GET" {
				if err != nil || calls != 2 {
					t.Fatalf("read not retried: %d %v", calls, err)
				}
			} else if err == nil || calls != 1 {
				t.Fatalf("write retried: %d %v", calls, err)
			}
		})
	}
}
func TestRejectRedirects(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "https://attacker.example", 302) }))
	defer s.Close()
	c, _ := New(s.URL+"/api/v1", "key", "test")
	if _, err := c.Do(context.Background(), "GET", "items/", nil); err == nil {
		t.Fatal("expected redirect rejection")
	}
}
func TestRejectInvalidBaseURL(t *testing.T) {
	for _, u := range []string{"", "file:///tmp", "https://user:password@example.com/api/v1", "https://example.com/api/v1?token=secret"} {
		if _, err := New(u, "key", "test"); err == nil {
			t.Errorf("accepted %s", u)
		}
	}
}
