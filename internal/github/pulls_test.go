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

func TestRecentMergedPullRequestsDecodesMergeCommitOID(t *testing.T) {
	var gotVars map[string]any
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotVars = body.Variables
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequests":{"nodes":[
			{"title":"fix: squash merge","number":11,"author":{"__typename":"User","login":"dana","url":"https://example.invalid/dana"},
			 "baseRepository":{"nameWithOwner":"acme/widget"},"mergedAt":"2026-02-02T00:00:00Z","isCrossRepository":false,
			 "labels":{"nodes":[]},"merged":true,"mergeCommit":{"oid":"bbb222"}}
		]}}}}`))
	})
	c := newTestClient(t, f)

	baseRef := "main"
	prs, err := c.RecentMergedPullRequests(context.Background(), "acme", "widget", &baseRef, 5, CommitFieldOptions{})
	if err != nil {
		t.Fatalf("RecentMergedPullRequests: %v", err)
	}
	if len(prs) != 1 || prs[0].Number != 11 || prs[0].MergeCommitOID != "bbb222" {
		t.Fatalf("prs = %+v", prs)
	}
	if gotVars["baseRefName"] != "main" || gotVars["limit"] != float64(5) {
		t.Errorf("gotVars = %v", gotVars)
	}
}

func TestRecentMergedPullRequestsDefaultsLimitAndOmitsBaseRefName(t *testing.T) {
	var gotVars map[string]any
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotVars = body.Variables
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequests":{"nodes":[]}}}}`))
	})
	c := newTestClient(t, f)

	if _, err := c.RecentMergedPullRequests(context.Background(), "acme", "widget", nil, 0, CommitFieldOptions{}); err != nil {
		t.Fatalf("RecentMergedPullRequests: %v", err)
	}
	if gotVars["limit"] != float64(5) {
		t.Errorf("limit = %v, want default 5", gotVars["limit"])
	}
	if _, ok := gotVars["baseRefName"]; ok {
		t.Errorf("baseRefName sent as %v even though it was nil", gotVars["baseRefName"])
	}
}
