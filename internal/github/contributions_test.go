package github

/*
ISC License

Copyright (c) 2026 Shane & Contributors

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
*/

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestNewContributorLoginsReturnsOnlyZeroCountAuthors(t *testing.T) {
	var gotVars map[string]any
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Variables map[string]any `json:"variables"`
			Query     string         `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotVars = body.Variables
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"author0":{"issueCount":0},"author1":{"issueCount":3}}}`))
	})
	c := newTestClient(t, f)

	candidates := []Candidate{
		{Login: "alice", MergedAt: "2026-02-01T00:00:00Z"},
		{Login: "bob", MergedAt: "2026-02-02T00:00:00Z"},
	}
	out, err := c.NewContributorLogins(context.Background(), "acme", "widget", candidates)
	if err != nil {
		t.Fatalf("NewContributorLogins: %v", err)
	}
	if !out["alice"] || out["bob"] {
		t.Fatalf("out = %v, want only alice", out)
	}
	if gotVars["query0"] != "repo:acme/widget is:pr is:merged author:alice merged:<2026-02-01T00:00:00Z" {
		t.Errorf("query0 = %v", gotVars["query0"])
	}
}

func TestNewContributorLoginsSkipsTheCallWhenThereAreNoCandidates(t *testing.T) {
	called := false
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	c := newTestClient(t, f)

	out, err := c.NewContributorLogins(context.Background(), "acme", "widget", nil)
	if err != nil {
		t.Fatalf("NewContributorLogins: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("out = %v, want empty", out)
	}
	if called {
		t.Fatal("expected no request when there are no candidates")
	}
}
