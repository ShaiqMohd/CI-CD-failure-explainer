package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type captured struct {
	Text   string `json:"text"`
	Blocks []struct {
		Text struct {
			Text string `json:"text"`
		} `json:"text"`
	} `json:"blocks"`
}

func (c captured) all() string {
	parts := []string{c.Text}
	for _, b := range c.Blocks {
		parts = append(parts, b.Text.Text)
	}
	return strings.Join(parts, "\n")
}

func newServer(t *testing.T, out *captured, status int, reply string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if out != nil {
			if err := json.NewDecoder(r.Body).Decode(out); err != nil {
				t.Errorf("slack payload was not valid JSON: %v", err)
			}
		}
		w.WriteHeader(status)
		w.Write([]byte(reply))
	}))
}

func TestNotifySendsEscapedPayload(t *testing.T) {
	var got captured
	srv := newServer(t, &got, http.StatusOK, "ok")
	defer srv.Close()

	err := New(srv.URL).Notify(context.Background(), Notification{
		Pipeline: "demo", Build: "7",
		Explanation: "a <b> & c", Fix: "run go build",
	})
	if err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}
	all := got.all()
	for _, want := range []string{"demo #7", "a &lt;b&gt; &amp; c", "run go build"} {
		if !strings.Contains(all, want) {
			t.Errorf("payload missing %q:\n%s", want, all)
		}
	}
}

func TestNotifyAIUnavailableShowsLogTail(t *testing.T) {
	var got captured
	srv := newServer(t, &got, http.StatusOK, "ok")
	defer srv.Close()

	err := New(srv.URL).Notify(context.Background(), Notification{
		Pipeline: "demo", Build: "8", AIUnavailable: true,
		LogTail: "main.go:3:1: syntax error",
	})
	if err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}
	all := got.all()
	for _, want := range []string{"unavailable", "syntax error"} {
		if !strings.Contains(all, want) {
			t.Errorf("payload missing %q:\n%s", want, all)
		}
	}
}

func TestNotifyErrors(t *testing.T) {
	t.Run("non-200 status", func(t *testing.T) {
		srv := newServer(t, nil, http.StatusBadRequest, "invalid_payload")
		defer srv.Close()
		err := New(srv.URL).Notify(context.Background(), Notification{Pipeline: "p"})
		if err == nil || !strings.Contains(err.Error(), "400") {
			t.Errorf("expected a 400 error, got %v", err)
		}
	})

	t.Run("connection error does not leak the webhook URL", func(t *testing.T) {
		srv := newServer(t, nil, http.StatusOK, "ok")
		base := srv.URL
		srv.Close() // nothing is listening any more
		err := New(base+"/services/SECRET-PATH").Notify(context.Background(), Notification{Pipeline: "p"})
		if err == nil {
			t.Fatal("expected a connection error")
		}
		if strings.Contains(err.Error(), "SECRET-PATH") {
			t.Errorf("error leaks the webhook URL: %v", err)
		}
	})
}
