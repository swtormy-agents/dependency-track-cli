package dt

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	c := &Client{APIKey: "abc123"}
	if s := c.Redact("error abc123 here"); s != "error <API_KEY> here" {
		t.Fatal(s)
	}
}
