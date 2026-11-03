// Copyright (c) Bogware. SPDX-License-Identifier: MPL-2.0
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultURL = "https://ah-api.merge.dev/api/v1"

type Client struct {
	base    *url.URL
	key     string
	http    *http.Client
	version string
}

// APIError intentionally excludes response bodies and server messages: they may echo credentials.
type APIError struct {
	Status    int
	RequestID string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Merge API returned HTTP %d (request ID: %s)", e.Status, e.RequestID)
}
func IsNotFound(err error) bool {
	var e *APIError
	return errors.As(err, &e) && e.Status == http.StatusNotFound
}

func New(baseURL, key, version string) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/")
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("api_url must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	if u.Scheme == "http" {
		ip := net.ParseIP(u.Hostname())
		if ip == nil || !ip.IsLoopback() {
			return nil, fmt.Errorf("api_url must use HTTPS; HTTP is allowed only for loopback IP addresses used by local fixtures")
		}
	}
	return &Client{base: u, key: key, version: version, http: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// resolve confines authenticated requests and pagination to the configured API origin and path.
func (c *Client) resolve(endpoint string) (*url.URL, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid API endpoint")
	}
	u = c.base.ResolveReference(u)
	if u.Scheme != c.base.Scheme || u.Host != c.base.Host || u.User != nil || !strings.HasPrefix(u.Path, c.base.Path) {
		return nil, fmt.Errorf("API endpoint or pagination link leaves the configured API base")
	}
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == ".." || segment == "." {
			return nil, fmt.Errorf("invalid API path segment")
		}
	}
	return u, nil
}

func (c *Client) Do(ctx context.Context, method, endpoint string, body any) (any, error) {
	u, err := c.resolve(endpoint)
	if err != nil {
		return nil, err
	}
	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("construct API request")
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "terraform-provider-mergeah/"+c.version)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Merge API request failed; check connectivity and api_url")
		}
		// Retry only reads. Writes have no general idempotency guarantee.
		if method == http.MethodGet && attempt < 3 && (res.StatusCode == 429 || res.StatusCode >= 500) {
			wait := retryDelay(res.Header.Get("Retry-After"), attempt, time.Now())
			res.Body.Close()
			if wait > 60*time.Second {
				return nil, &APIError{Status: res.StatusCode, RequestID: res.Header.Get("X-Request-ID")}
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(res.Body, 32<<20))
		res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, &APIError{Status: res.StatusCode, RequestID: res.Header.Get("X-Request-ID")}
		}
		if readErr != nil {
			return nil, fmt.Errorf("read Merge API response")
		}
		if len(bytes.TrimSpace(data)) == 0 {
			return nil, nil
		}
		var result any
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err := decoder.Decode(&result); err != nil {
			return nil, fmt.Errorf("Merge API returned invalid JSON")
		}
		return result, nil
	}
}

func retryDelay(header string, attempt int, now time.Time) time.Duration {
	wait := time.Second * time.Duration(1<<attempt)
	v := strings.TrimSpace(header)
	if secs, err := strconv.ParseUint(v, 10, 64); err == nil || errors.Is(err, strconv.ErrRange) {
		// Bound seconds before converting to nanoseconds, including integers
		// too large even for uint64. The caller refuses waits over 60 seconds.
		if secs > 60 || err != nil {
			return 61 * time.Second
		}
		return time.Duration(secs) * time.Second
	}
	if date, err := http.ParseTime(v); err == nil {
		wait = date.Sub(now)
		if wait < 0 {
			return 0
		}
	}
	return wait
}

// List supports the endpoint references' results/next shape, cursor pagination,
// and unpaginated arrays. It fails on loops rather than silently truncating data.
func (c *Client) List(ctx context.Context, endpoint string) ([]any, error) {
	items := []any{}
	seen := map[string]bool{}
	for endpoint != "" {
		u, err := c.resolve(endpoint)
		if err != nil {
			return nil, err
		}
		if seen[u.String()] {
			return nil, fmt.Errorf("Merge API pagination repeated a page")
		}
		seen[u.String()] = true
		if len(seen) > 10000 {
			return nil, fmt.Errorf("Merge API pagination exceeded 10000 pages")
		}
		result, err := c.Do(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		if list, ok := result.([]any); ok {
			return append(items, list...), nil
		}
		obj, ok := result.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected a paginated object or array")
		}
		list, ok := obj["results"].([]any)
		if !ok {
			list, ok = obj["data"].([]any)
		}
		if !ok {
			return nil, fmt.Errorf("list response is missing results/data array")
		}
		items = append(items, list...)
		endpoint = ""
		if more, _ := obj["has_more"].(bool); more {
			cursor, _ := obj["next_cursor"].(string)
			if cursor == "" {
				return nil, fmt.Errorf("pagination has_more without next_cursor")
			}
			q := u.Query()
			q.Set("cursor", cursor)
			u.RawQuery = q.Encode()
			endpoint = u.String()
		} else if next, _ := obj["next"].(string); next != "" {
			n, e := url.Parse(next)
			if e != nil {
				return nil, fmt.Errorf("invalid pagination link")
			}
			endpoint = u.ResolveReference(n).String()
		}
	}
	return items, nil
}
