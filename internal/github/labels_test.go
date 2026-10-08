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

func TestAddLabelsSendsEveryLabel(t *testing.T) {
	var gotLabels []string
	called := false
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost || r.URL.Path != "/repos/acme/widget/issues/7/labels" {
			t.Fatalf("method/path = %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotLabels)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	c := newTestClient(t, f)

	if err := c.AddLabels(context.Background(), "acme", "widget", 7, []string{"enhancement", "breaking"}); err != nil {
		t.Fatalf("AddLabels: %v", err)
	}
	if !called {
		t.Fatal("expected a request")
	}
	if len(gotLabels) != 2 || gotLabels[0] != "enhancement" || gotLabels[1] != "breaking" {
		t.Errorf("gotLabels = %v", gotLabels)
	}
}

func TestAddLabelsSkipsTheCallWhenThereAreNoLabels(t *testing.T) {
	called := false
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	c := newTestClient(t, f)

	if err := c.AddLabels(context.Background(), "acme", "widget", 7, nil); err != nil {
		t.Fatalf("AddLabels: %v", err)
	}
	if called {
		t.Fatal("expected no request when there are no labels to add")
	}
}
