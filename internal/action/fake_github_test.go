package action

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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// recordedRequest is one request fakeGitHub saw, kept for assertions.
type recordedRequest struct {
	Method string
	Path   string
	Body   []byte
}

// fakeGitHub stands in for both the REST and GraphQL GitHub APIs behind one
// httptest.Server, the way a real action only ever talks to one host. Each
// handler field is set by the test before exercising it; an unset handler
// answers with its zero value (empty list, 500 for a write the test did not
// expect).
type fakeGitHub struct {
	srv *httptest.Server
	mu  sync.Mutex

	requests []recordedRequest

	releases      []byte
	graphQL       func(query string, vars map[string]any) []byte
	createRelease func(body []byte) []byte
	updateRelease func(id string, body []byte) []byte
	addLabels     func(number string, body []byte)
	changedFiles  func(number string) []byte
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGitHub) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: body})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/graphql":
		var payload struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(body, &payload)
		if f.graphQL == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(f.graphQL(payload.Query, payload.Variables))
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/releases"):
		if f.releases == nil {
			_, _ = w.Write([]byte("[]"))
			return
		}
		_, _ = w.Write(f.releases)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/releases"):
		if f.createRelease == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(f.createRelease(body))
	case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/releases/"):
		if f.updateRelease == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(f.updateRelease(pathSegment(r.URL.Path, "releases"), body))
	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/labels"):
		if f.addLabels != nil {
			f.addLabels(pathSegment(r.URL.Path, "issues"), body)
		}
		_, _ = w.Write([]byte("[]"))
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/files"):
		if f.changedFiles != nil {
			_, _ = w.Write(f.changedFiles(pathSegment(r.URL.Path, "pulls")))
			return
		}
		_, _ = w.Write([]byte("[]"))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// pathSegment returns the path element right after the first occurrence of
// after, e.g. pathSegment("/repos/o/r/issues/42/labels", "issues") == "42".
func pathSegment(path, after string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		if p == after && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshaling %+v: %v", v, err)
	}
	return data
}

func graphQLEnvelope(t *testing.T, data any) []byte {
	return mustJSON(t, map[string]any{"data": data})
}

func releasesJSON(t *testing.T, id int64, tag, name, targetCommitish string, draft bool) []byte {
	return mustJSON(t, []map[string]any{{
		"id": id, "tag_name": tag, "name": name, "draft": draft, "prerelease": false,
		"target_commitish": targetCommitish, "created_at": "2026-01-01T00:00:00Z",
		"html_url":   fmt.Sprintf("https://github.com/o/r/releases/tag/%s", tag),
		"upload_url": fmt.Sprintf("https://uploads.github.com/repos/o/r/releases/%d/assets", id),
	}})
}

func emptyRecentMergedResponse(t *testing.T) []byte {
	return graphQLEnvelope(t, map[string]any{
		"repository": map[string]any{
			"pullRequests": map[string]any{"nodes": []any{}},
		},
	})
}
