// Package dt is a minimal Dependency-Track REST API client.
package dt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultPageSize   = 100
	MinAPIKeyLength   = 16
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

type APIError struct {
	Status  int
	Message string
	Body    json.RawMessage
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("dependency-track HTTP %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("dependency-track HTTP %d", e.Status)
}

func (e *APIError) IsAuth() bool { return e.Status == 401 || e.Status == 403 }

func (c *Client) base() string {
	return strings.TrimRight(c.BaseURL, "/")
}

func (c *Client) http() *http.Client {
	if c.HTTP == nil {
		return http.DefaultClient
	}
	return c.HTTP
}

// Redact removes the API key from text that may reach logs or output.
// Keys shorter than 16 characters are not substituted to avoid false positives.
func (c *Client) Redact(s string) string {
	if c.APIKey == "" || len(c.APIKey) < MinAPIKeyLength {
		return s
	}
	return strings.ReplaceAll(s, c.APIKey, "<API_KEY>")
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	u := c.base() + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, errors.New(c.Redact(err.Error()))
	}
	req.Header.Set("X-Api-Key", c.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body io.Reader) (json.RawMessage, http.Header, error) {
	req, err := c.newRequest(ctx, method, path, query, body)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request %s %s failed: %s", method, path, c.Redact(err.Error()))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %s", c.Redact(err.Error()))
	}
	if resp.StatusCode >= 400 {
		msg := c.Redact(parseMessage(data))
		return nil, resp.Header, &APIError{Status: resp.StatusCode, Message: msg, Body: data}
	}
	if len(data) == 0 {
		return json.RawMessage("null"), resp.Header, nil
	}
	if !json.Valid(data) {
		return nil, resp.Header, &APIError{Status: resp.StatusCode, Message: "non-JSON response"}
	}
	return json.RawMessage(data), resp.Header, nil
}

func parseMessage(data []byte) string {
	var m struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
	if json.Unmarshal(data, &m) == nil {
		if m.Detail != "" {
			return m.Detail
		}
		if m.Title != "" {
			return m.Title
		}
	}
	return strings.TrimSpace(string(data))
}

// Get performs GET and returns the JSON body.
func (c *Client) Get(ctx context.Context, path string, query url.Values) (json.RawMessage, http.Header, error) {
	return c.Do(ctx, http.MethodGet, path, query, nil)
}

// JSON sends a JSON body for PUT/POST.
func (c *Client) JSON(ctx context.Context, method, path string, v any) (json.RawMessage, http.Header, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, nil, err
	}
	return c.Do(ctx, method, path, nil, bytes.NewReader(b))
}

// PaginateGET fetches pages until no more data or caps are hit.
func (c *Client) PaginateGET(ctx context.Context, path string, baseQuery url.Values, pageSize, maxItems int) (json.RawMessage, error) {
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if maxItems <= 0 {
		maxItems = 500
	}
	q := cloneValues(baseQuery)
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))
	var all []json.RawMessage
	page := 1
	for {
		q.Set("pageNumber", fmt.Sprintf("%d", page))
		raw, hdr, err := c.Get(ctx, path, q)
		if err != nil {
			return nil, err
		}
		items, more := splitPage(raw, hdr, pageSize)
		all = append(all, items...)
		if len(all) >= maxItems {
			all = all[:maxItems]
			break
		}
		if !more || len(items) == 0 {
			break
		}
		page++
	}
	b, err := json.Marshal(all)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func cloneValues(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for k, vv := range v {
		out[k] = append([]string(nil), vv...)
	}
	return out
}

func splitPage(raw json.RawMessage, hdr http.Header, pageSize int) ([]json.RawMessage, bool) {
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		return arr, len(arr) >= pageSize
	}
	var obj struct {
		Items []json.RawMessage `json:"items"`
	}
	if json.Unmarshal(raw, &obj) == nil && obj.Items != nil {
		return obj.Items, len(obj.Items) >= pageSize
	}
	return []json.RawMessage{raw}, false
}

func headerInt(hdr http.Header, key string) int {
	s := hdr.Get(key)
	if s == "" {
		return 0
	}
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

// UploadBOM sends a CycloneDX BOM for a project.
func (c *Client) UploadBOM(ctx context.Context, projectUUID, bomPath string) (json.RawMessage, http.Header, error) {
	f, err := os.Open(bomPath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("project", projectUUID)
	_ = w.WriteField("autoCreate", "false")
	part, err := w.CreateFormFile("bom", filepath.Base(bomPath))
	if err != nil {
		return nil, nil, err
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, nil, err
	}
	if err := w.Close(); err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.base()+"/api/v1/bom", &buf)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("X-Api-Key", c.APIKey)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, nil, errors.New(c.Redact(err.Error()))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %s", c.Redact(err.Error()))
	}
	if resp.StatusCode >= 400 {
		msg := c.Redact(parseMessage(data))
		return nil, resp.Header, &APIError{Status: resp.StatusCode, Message: msg, Body: data}
	}
	if len(data) == 0 {
		return json.RawMessage("null"), resp.Header, nil
	}
	return json.RawMessage(data), resp.Header, nil
}

// DryRun describes a request without credentials.
func DryRun(method, baseURL, path string, query url.Values, body any) map[string]any {
	u := strings.TrimRight(baseURL, "/") + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	out := map[string]any{
		"method": method,
		"url":    u,
		"header": map[string]string{"X-Api-Key": "<API_KEY>"},
	}
	if body != nil {
		out["body"] = body
	}
	return out
}
