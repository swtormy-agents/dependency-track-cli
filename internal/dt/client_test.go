package dt

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClientGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "secret" {
			t.Fatalf("missing key")
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, APIKey: "secret"}
	raw, _, err := c.Get(context.Background(), "/api/version", nil)
	if err != nil || string(raw) != `{"ok":true}` {
		t.Fatalf("err=%v raw=%s", err, raw)
	}
}

func TestRedact(t *testing.T) {
	c := &Client{APIKey: "abcdefghijklmnop"}
	if s := c.Redact("error abcdefghijklmnop here"); s != "error <API_KEY> here" {
		t.Fatal(s)
	}
	c.APIKey = "short"
	if s := c.Redact("error short here"); s != "error short here" {
		t.Fatal("short keys must not be redacted:", s)
	}
}

func TestHTTPErrorBodyRedactsAPIKey(t *testing.T) {
	key := "abcdefghijklmnop"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"bad key `+key+`"}`, http.StatusBadRequest)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, APIKey: key}
	_, _, err := c.Get(context.Background(), "/api/version", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	api, ok := err.(*APIError)
	if !ok {
		t.Fatalf("want APIError got %T", err)
	}
	if strings.Contains(api.Message, key) {
		t.Fatalf("HTTP error body leaked key: %q", api.Message)
	}
	if !strings.Contains(api.Message, "<API_KEY>") {
		t.Fatalf("expected redaction in message: %q", api.Message)
	}
}

type keyLeakTransport struct {
	key string
}

func (t keyLeakTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("transport failed with credential %s", t.key)
}

func TestTransportErrorRedactsAPIKey(t *testing.T) {
	key := "abcdefghijklmnop"
	c := &Client{
		BaseURL: "http://127.0.0.1:1",
		APIKey:  key,
		HTTP:    &http.Client{Transport: keyLeakTransport{key: key}},
	}
	_, _, err := c.Get(context.Background(), "/api/version", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, key) {
		t.Fatalf("transport error leaked key: %s", msg)
	}
	if !strings.Contains(msg, "<API_KEY>") {
		t.Fatalf("expected redaction in transport error: %s", msg)
	}
}

func TestUploadBOMErrorRedactsAPIKey(t *testing.T) {
	key := "abcdefghijklmnop"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"upload rejected for `+key+`"}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	dir := t.TempDir()
	bomPath := filepath.Join(dir, "bom.json")
	if err := os.WriteFile(bomPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	c := &Client{BaseURL: srv.URL, APIKey: key}
	_, _, err := c.UploadBOM(context.Background(), "project-uuid", bomPath)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, key) {
		t.Fatalf("UploadBOM error leaked key: %s", msg)
	}
	if !strings.Contains(msg, "<API_KEY>") {
		t.Fatalf("expected redaction in UploadBOM error: %s", msg)
	}
}
