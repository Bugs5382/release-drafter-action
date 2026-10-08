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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// fakeServer is a minimal httptest-backed GitHub stand-in: tests register a
// handler per path and this records every request it saw. failFirst makes
// the first N requests (across every path) fail with a 502, to exercise
// retryTransport without a real flaky network.
type fakeServer struct {
	srv *httptest.Server
	mu  sync.Mutex

	requests  []*http.Request
	bodies    [][]byte
	failFirst int

	handle func(w http.ResponseWriter, r *http.Request)
}

func newFakeServer(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) *fakeServer {
	t.Helper()
	f := &fakeServer{handle: handle}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := readAll(r)
		r.Body = io.NopCloser(bytes.NewReader(body))
		f.mu.Lock()
		f.requests = append(f.requests, r)
		f.bodies = append(f.bodies, body)
		fail := f.failFirst > 0
		if fail {
			f.failFirst--
		}
		f.mu.Unlock()
		if fail {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		f.handle(w, r)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func readAll(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer func() { _ = r.Body.Close() }()
	return io.ReadAll(r.Body)
}

func noSleep(time.Duration) {}

func newTestClient(t *testing.T, f *fakeServer) *Client {
	t.Helper()
	return New(Options{RESTBaseURL: f.srv.URL, GraphQLURL: f.srv.URL + "/graphql", Token: "test-token", Sleep: noSleep})
}

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("loading fixture %s: %v", name, err)
	}
	return data
}

func TestRetryTransportRetriesOnServerErrorsAndGivesUp(t *testing.T) {
	calls := 0
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	})
	c := newTestClient(t, f)

	_, err := c.ListReleases(context.Background(), "o", "r")
	if err == nil {
		t.Fatal("expected an error after exhausting retries")
	}
	if calls != maxAttempts {
		t.Fatalf("calls = %d, want %d", calls, maxAttempts)
	}
}

func TestRetryTransportRecoversAfterTransientFailures(t *testing.T) {
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	})
	f.failFirst = maxAttempts - 1
	c := newTestClient(t, f)

	releases, err := c.ListReleases(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(releases) != 0 {
		t.Fatalf("releases = %v, want empty", releases)
	}
}

func TestDoGraphQLSendsQueryAndVariables(t *testing.T) {
	var gotBody map[string]any
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = fmt.Fprint(w, `{"data":{"ok":true}}`)
	})
	c := newTestClient(t, f)

	var out struct {
		OK bool `json:"ok"`
	}
	err := c.doGraphQL(context.Background(), "query q($x: String!) { ok }", map[string]any{"x": "y"}, &out)
	if err != nil {
		t.Fatalf("doGraphQL: %v", err)
	}
	if !out.OK {
		t.Fatalf("out.OK = false, want true")
	}
	if gotBody["query"] != "query q($x: String!) { ok }" {
		t.Errorf("query sent = %v", gotBody["query"])
	}
	vars, ok := gotBody["variables"].(map[string]any)
	if !ok || vars["x"] != "y" {
		t.Errorf("variables sent = %v", gotBody["variables"])
	}
}

func TestDoGraphQLSurfacesGraphQLErrors(t *testing.T) {
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"data":null,"errors":[{"message":"not found"}]}`)
	})
	c := newTestClient(t, f)

	var out struct{}
	err := c.doGraphQL(context.Background(), "query q { ok }", nil, &out)
	if err == nil {
		t.Fatal("expected a GraphQLError")
	}
	gqlErr, ok := err.(*GraphQLError)
	if !ok {
		t.Fatalf("err = %T, want *GraphQLError", err)
	}
	if len(gqlErr.Messages) != 1 || gqlErr.Messages[0] != "not found" {
		t.Errorf("Messages = %v", gqlErr.Messages)
	}
}

func TestDoGraphQLRejectsNonOKStatus(t *testing.T) {
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"message":"bad credentials"}`)
	})
	c := newTestClient(t, f)
	f.failFirst = 0

	var out struct{}
	err := c.doGraphQL(context.Background(), "query q { ok }", nil, &out)
	if err == nil {
		t.Fatal("expected an error for a 403 that is not retried past maxAttempts")
	}
}
