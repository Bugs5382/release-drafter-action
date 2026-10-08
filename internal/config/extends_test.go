package config

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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

const sharedConfig = "template: shared $CHANGES\n"

// fakeGitHub serves the extends endpoints for cfg/shared: the tag lookup,
// the release+asset lookup and download, the Contents API fallback, and a
// separate storage host the asset download redirects to, as GitHub does.
type fakeGitHub struct {
	api, storage *httptest.Server
	mu           sync.Mutex
	// requests is every API path, with the Authorization header it carried.
	requests     []string
	storageAuth  []string
	asset        string
	digest       string
	assetName    string
	requireToken string
	noTag        bool
	noRelease    bool
	failFirst    int
	// storageQuery, when set, is appended to the storage redirect Location
	// as a query string, standing in for a signed credential.
	storageQuery string
	// contents maps a ref to the file the Contents API fallback returns for
	// it. refNotFound, when true, makes every Contents API call report the
	// ref itself as missing (422), rather than the file (404).
	contents    map[string]string
	refNotFound bool
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{asset: sharedConfig, digest: "sha256:" + sha256Hex(sharedConfig), assetName: "release-drafter.yml"}
	f.storage = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.storageAuth = append(f.storageAuth, r.Header.Get("Authorization"))
		f.mu.Unlock()
		_, _ = fmt.Fprint(w, f.asset)
	}))
	f.api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.URL.Path+"?"+r.URL.RawQuery+" "+r.Header.Get("Authorization"))
		fail := f.failFirst > 0
		if fail {
			f.failFirst--
		}
		f.mu.Unlock()
		if fail {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if f.requireToken != "" && r.Header.Get("Authorization") != "Bearer "+f.requireToken {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}
		switch r.URL.Path {
		case "/repos/cfg/shared/git/ref/tags/v1.0.0":
			if f.noTag {
				http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
				return
			}
			_, _ = fmt.Fprint(w, `{"ref":"refs/tags/v1.0.0","object":{"type":"commit","sha":"abc"}}`)
		case "/repos/cfg/shared/releases/tags/v1.0.0":
			if f.noRelease {
				http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
				return
			}
			_, _ = fmt.Fprintf(w, `{"tag_name":"v1.0.0","assets":[{"id":1,"name":"notes.txt","size":3,"digest":"sha256:00"},{"id":7,"name":%q,"size":%d,"digest":%q}]}`, f.assetName, len(f.asset), f.digest)
		case "/repos/cfg/shared/releases/assets/7":
			if r.Header.Get("Accept") != "application/octet-stream" {
				http.Error(w, "want octet-stream", http.StatusUnsupportedMediaType)
				return
			}
			loc := f.storage.URL + "/blob/7"
			if f.storageQuery != "" {
				loc += "?" + f.storageQuery
			}
			http.Redirect(w, r, loc, http.StatusFound)
		case "/repos/cfg/shared/contents/.github/release-drafter.yml":
			if r.Header.Get("Accept") != "application/vnd.github.raw+json" {
				http.Error(w, "want raw json accept", http.StatusUnsupportedMediaType)
				return
			}
			ref := r.URL.Query().Get("ref")
			if f.refNotFound {
				http.Error(w, fmt.Sprintf(`{"message":"No commit found for the ref %s"}`, ref), http.StatusUnprocessableEntity)
				return
			}
			body, ok := f.contents[ref]
			if !ok {
				http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
				return
			}
			_, _ = fmt.Fprint(w, body)
		default:
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		}
	}))
	t.Cleanup(f.api.Close)
	t.Cleanup(f.storage.Close)
	return f
}

func (f *fakeGitHub) opts(token string) ExtendsOptions {
	return ExtendsOptions{APIURL: f.api.URL, Token: token, Sleep: func(time.Duration) {}}
}

func extendsMsg(t *testing.T, err error) string {
	t.Helper()
	var ee *ExtendsError
	if !errors.As(err, &ee) {
		t.Fatalf("err = %v; want *ExtendsError", err)
	}
	return ee.Error()
}

func TestParseExtends(t *testing.T) {
	ref, err := ParseExtends("the-rabbit-hole-tech/release-drafter-config@v1.0.0")
	if err != nil || ref.Owner != "the-rabbit-hole-tech" || ref.Repo != "release-drafter-config" || ref.Ref != "v1.0.0" {
		t.Errorf("ParseExtends = %+v, %v", ref, err)
	}
	if ref, err := ParseExtends("o/r@2026-09-26"); err != nil || ref.Ref != "2026-09-26" {
		t.Errorf("date tag: %+v, %v", ref, err)
	}
	// Branches and the major/minor tags that once had to be rejected now
	// parse fine: FetchExtends, not ParseExtends, decides what kind of ref
	// each one turns out to be.
	for _, in := range []string{"o/r@main", "o/r@v1", "o/r@v1.2", "o/r@1", "o/r@feature/thing"} {
		if _, err := ParseExtends(in); err != nil {
			t.Errorf("ParseExtends(%q) = %v; want it to parse", in, err)
		}
	}
	for in, part := range map[string]string{
		"o/r":                 "expected owner/repo@ref",
		"o@v1.0.0":            "expected owner/repo@ref",
		"o/r/x@v1.0.0":        "expected owner/repo@ref",
		"o/r@":                "not a valid branch, tag or SHA",
		"o/r@refs/heads/main": "not a fully qualified ref",
		"o w/r@v1.0.0":        "letters, digits",
		"../r@v1.0.0":         "letters, digits",
		"o/..@v1.0.0":         "letters, digits",
		"./.@v1.0.0":          "letters, digits",
		"-o/r@v1.0.0":         "letters, digits",
	} {
		_, err := ParseExtends(in)
		if msg := extendsMsg(t, err); !strings.Contains(msg, part) {
			t.Errorf("ParseExtends(%q) = %q; want it to mention %q", in, msg, part)
		}
	}
}

func TestFetchExtendsDownloadsAndVerifies(t *testing.T) {
	f := newFakeGitHub(t)
	src, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", f.opts("t0ken"))
	if err != nil {
		t.Fatal(err)
	}
	if string(src.Text) != sharedConfig || src.Kind != KindExtends || src.Origin != "extends cfg/shared@v1.0.0 (asset release-drafter.yml)" {
		t.Errorf("source = %+v", src)
	}
	want := []string{
		"/repos/cfg/shared/git/ref/tags/v1.0.0? Bearer t0ken",
		"/repos/cfg/shared/releases/tags/v1.0.0? Bearer t0ken",
		"/repos/cfg/shared/releases/assets/7? Bearer t0ken",
	}
	if strings.Join(f.requests, "\n") != strings.Join(want, "\n") {
		t.Errorf("requests =\n%s", strings.Join(f.requests, "\n"))
	}
	if len(f.storageAuth) != 1 || f.storageAuth[0] != "" {
		t.Errorf("the token must not follow the redirect to storage: %q", f.storageAuth)
	}
}

func TestFetchExtendsNamedAsset(t *testing.T) {
	f := newFakeGitHub(t)
	f.assetName = "drafter.json"
	f.asset = `{"template":"$CHANGES"}`
	f.digest = "sha256:" + sha256Hex(f.asset)
	o := f.opts("")
	o.Asset = "drafter.json"
	src, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", o)
	if err != nil || string(src.Text) != f.asset {
		t.Fatalf("src = %+v, %v", src, err)
	}
	cfg, err := Parse(src.Text, src.Origin)
	if err != nil || cfg.Template != "$CHANGES" {
		t.Errorf("cfg = %+v, %v", cfg, err)
	}
}

func TestFetchExtendsWrongDigest(t *testing.T) {
	f := newFakeGitHub(t)
	f.digest = "sha256:" + strings.Repeat("0", 64)
	_, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", f.opts(""))
	if msg := extendsMsg(t, err); !strings.Contains(msg, "failed verification") || !strings.Contains(msg, strings.Repeat("0", 64)) {
		t.Errorf("err = %s", msg)
	}
}

func TestFetchExtendsMissingDigest(t *testing.T) {
	f := newFakeGitHub(t)
	f.digest = ""
	_, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", f.opts(""))
	if msg := extendsMsg(t, err); !strings.Contains(msg, "no sha256 digest") {
		t.Errorf("err = %s", msg)
	}
	if len(f.storageAuth) != 0 {
		t.Error("an unverifiable asset must not be downloaded")
	}
}

// TestFetchExtendsTagAssetMissingFallsBackToContents covers a tag whose
// release has no asset with the configured name: FetchExtends now reads
// the file at that tag through the Contents API instead of failing.
func TestFetchExtendsTagAssetMissingFallsBackToContents(t *testing.T) {
	f := newFakeGitHub(t)
	f.assetName = "other.yml"
	f.contents = map[string]string{"v1.0.0": sharedConfig}
	src, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", f.opts(""))
	if err != nil || string(src.Text) != sharedConfig || src.Kind != KindExtends {
		t.Fatalf("src = %+v, %v", src, err)
	}
	want := []string{
		"/repos/cfg/shared/git/ref/tags/v1.0.0? ",
		"/repos/cfg/shared/releases/tags/v1.0.0? ",
		"/repos/cfg/shared/contents/.github/release-drafter.yml?ref=v1.0.0 ",
	}
	if strings.Join(f.requests, "\n") != strings.Join(want, "\n") {
		t.Errorf("requests =\n%s", strings.Join(f.requests, "\n"))
	}
}

// TestFetchExtendsTagWithoutReleaseFallsBackToContents covers a tag with no
// published release at all: same fallback as a missing asset.
func TestFetchExtendsTagWithoutReleaseFallsBackToContents(t *testing.T) {
	f := newFakeGitHub(t)
	f.noRelease = true
	f.contents = map[string]string{"v1.0.0": sharedConfig}
	src, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", f.opts(""))
	if err != nil || string(src.Text) != sharedConfig {
		t.Fatalf("src = %+v, %v", src, err)
	}
}

func TestFetchExtendsBranch(t *testing.T) {
	f := newFakeGitHub(t)
	f.contents = map[string]string{"main": sharedConfig}
	src, err := FetchExtends(context.Background(), "cfg/shared@main", f.opts("t0ken"))
	if err != nil || string(src.Text) != sharedConfig || src.Kind != KindExtends {
		t.Fatalf("src = %+v, %v", src, err)
	}
	want := []string{
		"/repos/cfg/shared/git/ref/tags/main? Bearer t0ken",
		"/repos/cfg/shared/contents/.github/release-drafter.yml?ref=main Bearer t0ken",
	}
	if strings.Join(f.requests, "\n") != strings.Join(want, "\n") {
		t.Errorf("requests =\n%s", strings.Join(f.requests, "\n"))
	}
}

func TestFetchExtendsSHA(t *testing.T) {
	f := newFakeGitHub(t)
	sha := "0123456789abcdef0123456789abcdef01234567"
	f.contents = map[string]string{sha: sharedConfig}
	src, err := FetchExtends(context.Background(), "cfg/shared@"+sha, f.opts(""))
	if err != nil || string(src.Text) != sharedConfig {
		t.Fatalf("src = %+v, %v", src, err)
	}
}

// TestFetchExtendsContentsNotFound covers a ref that exists but has no
// .github/<asset> file: a clear error, not a generic one.
func TestFetchExtendsContentsNotFound(t *testing.T) {
	f := newFakeGitHub(t)
	// No entry in f.contents for "main": the ref resolves, the file does not.
	_, err := FetchExtends(context.Background(), "cfg/shared@main", f.opts(""))
	if msg := extendsMsg(t, err); !strings.Contains(msg, `.github/release-drafter.yml does not exist at ref "main"`) || !strings.Contains(msg, "add the file") {
		t.Errorf("err = %s", msg)
	}
}

// TestFetchExtendsRefNotFound covers a ref that does not exist at all
// (neither a tag nor a resolvable branch/SHA): the Contents API reports the
// ref itself as unprocessable, and that is reported clearly too.
func TestFetchExtendsRefNotFound(t *testing.T) {
	f := newFakeGitHub(t)
	f.refNotFound = true
	_, err := FetchExtends(context.Background(), "cfg/shared@does-not-exist", f.opts(""))
	if msg := extendsMsg(t, err); !strings.Contains(msg, `ref "does-not-exist" does not exist in cfg/shared`) {
		t.Errorf("err = %s", msg)
	}
}

func TestFetchExtendsPrivateRepoPassesToken(t *testing.T) {
	f := newFakeGitHub(t)
	f.requireToken = "s3cret"
	_, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", f.opts(""))
	if msg := extendsMsg(t, err); !strings.Contains(msg, "no token was passed") {
		t.Errorf("without a token: %s", msg)
	}
	f.requests = nil
	src, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", f.opts("s3cret"))
	if err != nil || string(src.Text) != sharedConfig {
		t.Fatalf("with the token: %+v, %v", src, err)
	}
	for _, r := range f.requests {
		if !strings.HasSuffix(r, " Bearer s3cret") {
			t.Errorf("request without the token: %q", r)
		}
	}
}

func TestFetchExtendsRetriesServerErrors(t *testing.T) {
	f := newFakeGitHub(t)
	f.failFirst = 2
	var slept []time.Duration
	o := f.opts("")
	o.Sleep = func(d time.Duration) { slept = append(slept, d) }
	if _, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", o); err != nil {
		t.Fatal(err)
	}
	if len(slept) != 2 || slept[0] != time.Second || slept[1] != 2*time.Second {
		t.Errorf("slept %v", slept)
	}
	f.failFirst = 3
	if _, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", o); err == nil || !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Errorf("err = %v", err)
	}
}

func TestFetchExtendsCacheHit(t *testing.T) {
	f := newFakeGitHub(t)
	o := f.opts("")
	o.CacheDir = t.TempDir()
	if _, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", o); err != nil {
		t.Fatal(err)
	}
	cached := filepath.Join(o.CacheDir, "cfg", "shared", "v1.0.0", "release-drafter.yml")
	if b, err := os.ReadFile(cached); err != nil || string(b) != sharedConfig {
		t.Fatalf("cache file: %q, %v", b, err)
	}
	f.requests = nil
	src, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", o)
	if err != nil || string(src.Text) != sharedConfig {
		t.Fatalf("cached: %+v, %v", src, err)
	}
	if len(f.requests) != 0 {
		t.Errorf("a cache hit must not call the API: %q", f.requests)
	}
	// A damaged cache entry is fetched again, not trusted.
	if err := os.WriteFile(cached, []byte("template: tampered\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	src, err = FetchExtends(context.Background(), "cfg/shared@v1.0.0", o)
	if err != nil || string(src.Text) != sharedConfig || len(f.requests) != 3 {
		t.Errorf("damaged cache: %q, %v, %d requests", src.Text, err, len(f.requests))
	}
}

// TestFetchExtendsBranchNeverCached checks that the Contents API fallback
// (a branch, here) is never written to the cache: a branch moves, so a
// stale copy must never be served without an API call.
func TestFetchExtendsBranchNeverCached(t *testing.T) {
	f := newFakeGitHub(t)
	f.contents = map[string]string{"main": sharedConfig}
	o := f.opts("")
	o.CacheDir = t.TempDir()
	if _, err := FetchExtends(context.Background(), "cfg/shared@main", o); err != nil {
		t.Fatal(err)
	}
	cached := filepath.Join(o.CacheDir, "cfg", "shared", "main", "release-drafter.yml")
	if _, err := os.ReadFile(cached); err == nil {
		t.Error("a branch read must not be cached")
	}
	f.requests = nil
	if _, err := FetchExtends(context.Background(), "cfg/shared@main", o); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) == 0 {
		t.Error("a branch read must call the API again, never serve a cached copy")
	}
}

// TestCachePathsRejectsEscapingRef is white-box: ParseExtends already
// rejects "." and ".." for owner and repo, so this can no longer be reached
// through FetchExtends. cachePaths still refuses to build a path outside
// dir on its own, as a second guard against whatever produced the ref.
func TestCachePathsRejectsEscapingRef(t *testing.T) {
	dir := t.TempDir()
	if _, _, ok := cachePaths(dir, ExtendsRef{Owner: "..", Repo: "..", Ref: "v1.0.0"}, "release-drafter.yml"); ok {
		t.Error("cachePaths must not return a path outside dir")
	}
	if file, sumFile, ok := cachePaths(dir, ExtendsRef{Owner: "cfg", Repo: "shared", Ref: "v1.0.0"}, "release-drafter.yml"); !ok || file == "" || sumFile == "" {
		t.Errorf("a normal ref must still resolve: %q, %q, %v", file, sumFile, ok)
	}
}

func TestFetchExtendsAssetTooLarge(t *testing.T) {
	f := newFakeGitHub(t)
	big := strings.Repeat("a", maxAssetSize+1)
	f.asset = big
	f.digest = "sha256:" + sha256Hex(big)
	_, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", f.opts(""))
	if msg := extendsMsg(t, err); !strings.Contains(msg, "is larger than") {
		t.Errorf("err = %s", msg)
	}
}

func TestFetchExtendsAssetNameMustBeFileName(t *testing.T) {
	f := newFakeGitHub(t)
	for _, bad := range []string{"sub/release-drafter.yml", "../release-drafter.yml", ".", ".."} {
		o := f.opts("")
		o.Asset = bad
		_, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", o)
		if msg := extendsMsg(t, err); !strings.Contains(msg, "must be a file name") {
			t.Errorf("asset %q: err = %s", bad, msg)
		}
	}
}

// failStorageTransport fails every request to the storage host at the
// transport level, standing in for a reset connection or timeout on the
// signed redirect target.
type failStorageTransport struct {
	storageHost string
}

func (t *failStorageTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == t.storageHost {
		return nil, errors.New("simulated storage transport failure")
	}
	return http.DefaultTransport.RoundTrip(req)
}

// TestFetchExtendsStorageTransportErrorHidesSignedQuery checks that a
// transport failure on the storage redirect never leaks the redirect's
// query string (a stand-in for a signed, short-lived credential) into the
// returned error or the log.
func TestFetchExtendsStorageTransportErrorHidesSignedQuery(t *testing.T) {
	f := newFakeGitHub(t)
	f.storageQuery = "X-Amz-Signature=verysekritvalue"
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	o := f.opts("")
	o.Log = &logger
	o.Transport = &failStorageTransport{storageHost: strings.TrimPrefix(f.storage.URL, "http://")}
	_, err := FetchExtends(context.Background(), "cfg/shared@v1.0.0", o)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "verysekritvalue") {
		t.Errorf("the signed query leaked into the error: %v", err)
	}
	if strings.Contains(buf.String(), "verysekritvalue") {
		t.Errorf("the signed query leaked into the log: %s", buf.String())
	}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
