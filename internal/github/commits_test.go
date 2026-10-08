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

func TestCommitsInRangePaginatesAndDecodesAssociatedPullRequests(t *testing.T) {
	page1 := loadFixture(t, "compare_commits_page1.json")
	page2 := loadFixture(t, "compare_commits_page2.json")
	calls := 0
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		if body.Variables["cursor"] == "cursor-1" {
			_, _ = w.Write(page2)
			return
		}
		if body.Variables["cursor"] != nil {
			t.Fatalf("unexpected cursor on first page: %v", body.Variables["cursor"])
		}
		_, _ = w.Write(page1)
	})
	c := newTestClient(t, f)

	commits, err := c.CommitsInRange(context.Background(), "acme", "widget", "refs/tags/v1.0.0", "main", CommitFieldOptions{})
	if err != nil {
		t.Fatalf("CommitsInRange: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if len(commits) != 2 {
		t.Fatalf("commits = %d, want 2", len(commits))
	}
	if commits[0].OID != "aaa111" || len(commits[0].PullRequests) != 1 || commits[0].PullRequests[0].Number != 10 {
		t.Errorf("commits[0] = %+v", commits[0])
	}
	if commits[0].PullRequests[0].Author == nil || commits[0].PullRequests[0].Author.Login != "alice" {
		t.Errorf("commits[0].PullRequests[0].Author = %+v", commits[0].PullRequests[0].Author)
	}
	if commits[0].Author == nil || commits[0].Author.Login != "alice" {
		t.Errorf("commits[0].Author = %+v", commits[0].Author)
	}
	if commits[1].OID != "bbb222" || len(commits[1].PullRequests) != 0 {
		t.Errorf("commits[1] = %+v", commits[1])
	}
	if commits[1].Author.Login != "" || commits[1].Author.Name != "Bob" {
		t.Errorf("commits[1].Author (no linked user) = %+v", commits[1].Author)
	}
}

func TestCommitsInRangeFailsWhenRefOrCompareIsMissing(t *testing.T) {
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"repository":{"ref":null}}}`))
	})
	c := newTestClient(t, f)

	_, err := c.CommitsInRange(context.Background(), "acme", "widget", "refs/tags/missing", "main", CommitFieldOptions{})
	if err == nil {
		t.Fatal("expected an error when the base ref does not exist")
	}
}

func TestCommitsSinceWalksFullHistoryWhenSinceIsNil(t *testing.T) {
	var gotVars map[string]any
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotVars = body.Variables
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"repository":{"object":{"history":{
			"pageInfo":{"hasNextPage":false,"endCursor":""},
			"nodes":[{"oid":"ccc333","committedDate":"2026-01-01T00:00:00Z","message":"chore: first commit","author":{"name":"Carol","user":{"login":"carol"}},"authors":{"nodes":[]},"associatedPullRequests":{"nodes":[]}}]
		}}}}}`))
	})
	c := newTestClient(t, f)

	commits, err := c.CommitsSince(context.Background(), "acme", "widget", "main", nil, CommitFieldOptions{})
	if err != nil {
		t.Fatalf("CommitsSince: %v", err)
	}
	if len(commits) != 1 || commits[0].OID != "ccc333" {
		t.Fatalf("commits = %+v", commits)
	}
	if _, ok := gotVars["since"]; ok {
		t.Errorf("since variable sent as %v even though it was nil", gotVars["since"])
	}
}

func TestCommitsSincePassesTheSinceTimestamp(t *testing.T) {
	var gotVars map[string]any
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotVars = body.Variables
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"repository":{"object":{"history":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}}`))
	})
	c := newTestClient(t, f)

	since := "2026-01-01T00:00:00Z"
	if _, err := c.CommitsSince(context.Background(), "acme", "widget", "main", &since, CommitFieldOptions{}); err != nil {
		t.Fatalf("CommitsSince: %v", err)
	}
	if gotVars["since"] != since {
		t.Errorf("since = %v, want %s", gotVars["since"], since)
	}
}
