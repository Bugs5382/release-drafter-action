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
	"net/http"
	"testing"
)

func TestResolveCommitishReturnsTheOID(t *testing.T) {
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"repository":{"object":{"__typename":"Commit","oid":"deadbeef"}}}}`))
	})
	c := newTestClient(t, f)

	oid, err := c.ResolveCommitish(context.Background(), "acme", "widget", "refs/tags/v1.0.0^{commit}")
	if err != nil {
		t.Fatalf("ResolveCommitish: %v", err)
	}
	if oid != "deadbeef" {
		t.Errorf("oid = %s, want deadbeef", oid)
	}
}

func TestResolveCommitishFailsWhenTheExpressionIsNotACommit(t *testing.T) {
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"repository":{"object":null}}}`))
	})
	c := newTestClient(t, f)

	if _, err := c.ResolveCommitish(context.Background(), "acme", "widget", "refs/tags/missing^{commit}"); err == nil {
		t.Fatal("expected an error when the expression does not resolve")
	}
}
