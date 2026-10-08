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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// DefaultExtendsAsset is the release asset extends downloads when the
// extends-asset input is empty.
const DefaultExtendsAsset = "release-drafter.yml"

// maxAssetSize caps the config asset download. A release-drafter config is
// a few kilobytes.
const maxAssetSize = 1 << 20

// ExtendsRef is a parsed extends input: owner/repo@ref, where ref is a
// branch, tag or commit SHA.
type ExtendsRef struct {
	Owner string
	Repo  string
	Ref   string
}

func (r ExtendsRef) String() string { return r.Owner + "/" + r.Repo + "@" + r.Ref }

// ExtendsError is any failure to load the extends config. The step fails
// with it; Msg says what to fix.
type ExtendsError struct {
	Ref string
	Msg string
}

func (e *ExtendsError) Error() string { return fmt.Sprintf("extends %s: %s", e.Ref, e.Msg) }

var (
	// ownerPart follows GitHub's own login rule: alphanumeric, starting
	// with an alphanumeric, up to 39 characters. This also rejects "." and
	// "..", which would otherwise let extends escape the cache directory.
	ownerPart = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
	// repoPart allows the wider set GitHub allows in a repository name;
	// "." and ".." are rejected separately below for the same reason.
	repoPart = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

// ParseExtends checks the extends input. It must be owner/repo@ref, where
// ref is a branch, tag or commit SHA. A branch or an unpinned tag follows
// every change to that ref; pin a tag or a full SHA for stability. FetchExtends
// resolves what kind of ref it is: a tag with a matching release asset keeps
// the digest-verified download, everything else reads the file straight from
// the repository at that ref.
func ParseExtends(s string) (ExtendsRef, error) {
	fail := func(msg string) (ExtendsRef, error) { return ExtendsRef{}, &ExtendsError{Ref: s, Msg: msg} }
	repo, ref, ok := strings.Cut(s, "@")
	owner, name, ok2 := strings.Cut(repo, "/")
	switch {
	case !ok || !ok2 || strings.Contains(ref, "@") || strings.Contains(name, "/"):
		return fail("expected owner/repo@ref, for example my-org/release-drafter-config@main")
	case !ownerPart.MatchString(owner) || !repoPart.MatchString(name) || name == "." || name == "..":
		return fail("the owner and repository may only hold letters, digits, '.', '_' and '-'")
	case ref == "" || strings.ContainsAny(ref, " \t\r\n") || strings.Contains(ref, ".."):
		return fail("the ref is empty or not a valid branch, tag or SHA")
	case strings.HasPrefix(ref, "refs/"):
		return fail("give the bare branch, tag or SHA, not a fully qualified ref like refs/heads/main")
	}
	return ExtendsRef{Owner: owner, Repo: name, Ref: ref}, nil
}

// ExtendsOptions configures FetchExtends.
type ExtendsOptions struct {
	// APIURL is the REST API root; "" means https://api.github.com.
	APIURL string
	// Token authenticates every API call. A private config repository needs
	// one that can read it.
	Token string
	// Asset is the release asset name; "" means DefaultExtendsAsset.
	Asset string
	// CacheDir holds downloaded assets per tag; "" turns the cache off.
	CacheDir string
	// Transport and Sleep are for tests.
	Transport http.RoundTripper
	Sleep     func(time.Duration)
	Log       *zerolog.Logger
}

type fetcher struct {
	o      ExtendsOptions
	ref    ExtendsRef
	client *http.Client
	log    *zerolog.Logger
}

type releaseAsset struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}

// FetchExtends loads the config another repository publishes at the given
// ref. When the ref is a tag that has a published release carrying the
// named asset, it downloads that asset and checks its sha256 against the
// digest GitHub reports; a pinned tag never goes stale, so a cached copy is
// used without any API call. Otherwise (a branch, a SHA, or a tag with no
// matching release asset) it reads .github/<asset> from that ref through
// the Contents API; that read is never cached, since a branch or an
// unpinned tag can change between calls.
func FetchExtends(ctx context.Context, extends string, o ExtendsOptions) (Source, error) {
	ref, err := ParseExtends(extends)
	if err != nil {
		return Source{}, err
	}
	if o.Asset == "" {
		o.Asset = DefaultExtendsAsset
	}
	if o.APIURL == "" {
		o.APIURL = "https://api.github.com"
	}
	if o.Sleep == nil {
		o.Sleep = time.Sleep
	}
	log := o.Log
	if log == nil {
		nop := zerolog.Nop()
		log = &nop
	}
	origin := fmt.Sprintf("extends %s (asset %s)", ref, o.Asset)
	if strings.ContainsAny(o.Asset, `/\`) || o.Asset == "." || o.Asset == ".." {
		return Source{}, &ExtendsError{Ref: ref.String(), Msg: fmt.Sprintf("extends-asset %q must be a file name", o.Asset)}
	}
	if data, ok := readCache(o.CacheDir, ref, o.Asset, log); ok {
		log.Info().Str("extends", ref.String()).Str("asset", o.Asset).Msg("extends config loaded from the cache")
		return Source{Text: data, Origin: origin, Kind: KindExtends}, nil
	}
	f := &fetcher{o: o, ref: ref, log: log, client: &http.Client{
		Transport: o.Transport,
		Timeout:   30 * time.Second,
		// The asset download redirects to a storage host. The token is
		// only for the API, so it never follows the redirect.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if req.URL.Host != via[0].URL.Host {
				req.Header.Del("Authorization")
			}
			return nil
		},
	}}
	data, cacheable, err := f.fetch(ctx)
	if err != nil {
		return Source{}, err
	}
	if cacheable {
		writeCache(o.CacheDir, ref, o.Asset, data, log)
	}
	return Source{Text: data, Origin: origin, Kind: KindExtends}, nil
}

func (f *fetcher) fail(format string, args ...any) error {
	return &ExtendsError{Ref: f.ref.String(), Msg: fmt.Sprintf(format, args...)}
}

func (f *fetcher) privateHint() string {
	if f.o.Token == "" {
		return " (no token was passed; a private repository needs one)"
	}
	return " (for a private repository, check that the token can read it)"
}

// sanitizeTransportErr strips the query string from a *url.Error's URL
// before it is logged or reported. The asset download redirects to a
// signed storage URL; the query string is a short-lived credential and
// must never reach the Actions log or the step's error annotation, even
// when the request to that URL fails at the transport level.
func sanitizeTransportErr(err error) error {
	var uerr *url.Error
	if !errors.As(err, &uerr) {
		return err
	}
	sanitized := *uerr
	if parsed, perr := url.Parse(uerr.URL); perr == nil && parsed.RawQuery != "" {
		parsed.RawQuery = ""
		sanitized.URL = parsed.String()
	}
	return &sanitized
}

func escapeRef(ref string) string {
	parts := strings.Split(ref, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// fetch resolves the extends ref and returns its config text. cacheable is
// true only for the digest-verified release-asset path: a pinned tag never
// goes stale, so that result may be cached; a branch, SHA, or an unpinned
// tag's live file never is.
func (f *fetcher) fetch(ctx context.Context) (data []byte, cacheable bool, err error) {
	repo := "/repos/" + f.ref.Owner + "/" + f.ref.Repo
	ref := escapeRef(f.ref.Ref)

	var gitRef struct {
		Ref string `json:"ref"`
	}
	status, err := f.getJSON(ctx, repo+"/git/ref/tags/"+ref, &gitRef)
	if err != nil {
		return nil, false, err
	}
	isTag := status == http.StatusOK && gitRef.Ref == "refs/tags/"+f.ref.Ref
	if isTag {
		data, found, err := f.fetchReleaseAsset(ctx, repo, ref)
		if err != nil {
			return nil, false, err
		}
		if found {
			return data, true, nil
		}
		// The tag exists but has no release, or no matching asset: fall
		// through to reading the file at that tag through the Contents API,
		// the same as a branch or a SHA.
	}
	data, err = f.fetchContents(ctx, repo)
	if err != nil {
		return nil, false, err
	}
	return data, false, nil
}

// fetchReleaseAsset looks for a published release at tag carrying the named
// asset. found is false only when the tag has no release, or the release
// has no asset with that name, so the caller can fall back to the Contents
// API; any other failure (a bad digest, a download error) is a hard error.
func (f *fetcher) fetchReleaseAsset(ctx context.Context, repo, tag string) (data []byte, found bool, err error) {
	var rel struct {
		TagName string         `json:"tag_name"`
		Assets  []releaseAsset `json:"assets"`
	}
	status, err := f.getJSON(ctx, repo+"/releases/tags/"+tag, &rel)
	if err != nil {
		return nil, false, err
	}
	if status == http.StatusNotFound {
		f.log.Debug().Str("extends", f.ref.String()).Msg("tag has no published release; reading the file at the tag instead")
		return nil, false, nil
	}
	var asset *releaseAsset
	for i := range rel.Assets {
		if rel.Assets[i].Name == f.o.Asset {
			asset = &rel.Assets[i]
		}
	}
	if asset == nil {
		f.log.Debug().Str("extends", f.ref.String()).Str("asset", f.o.Asset).Msg("release has no matching asset; reading the file at the tag instead")
		return nil, false, nil
	}
	want, ok := strings.CutPrefix(asset.Digest, "sha256:")
	if !ok || len(want) != sha256.Size*2 {
		return nil, false, f.fail("GitHub reports no sha256 digest for asset %s (got %q), so it cannot be verified; upload the asset again", asset.Name, asset.Digest)
	}
	f.log.Debug().Str("extends", f.ref.String()).Str("asset", asset.Name).Int64("asset_id", asset.ID).Int64("size", asset.Size).Str("digest", asset.Digest).Msg("extends asset found")

	resp, err := f.do(ctx, fmt.Sprintf("%s/releases/assets/%d", repo, asset.ID), "application/octet-stream")
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, false, f.fail("downloading asset %s returned %d%s", asset.Name, resp.StatusCode, f.privateHint())
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, maxAssetSize+1))
	if err != nil {
		return nil, false, f.fail("downloading asset %s: %v", asset.Name, err)
	}
	if len(data) > maxAssetSize {
		return nil, false, f.fail("asset %s is larger than %d bytes; a config asset should be a few kilobytes", asset.Name, maxAssetSize)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, want) {
		return nil, false, f.fail("asset %s failed verification: GitHub reports sha256:%s but the download is sha256:%s", asset.Name, want, got)
	}
	f.log.Info().Str("extends", f.ref.String()).Str("asset", asset.Name).Int("bytes", len(data)).Msg("extends config downloaded and verified")
	return data, true, nil
}

// fetchContents reads .github/<asset> from the repository at the extends
// ref (a branch, a SHA, or a tag with no matching release asset) through
// the Contents API. It has no digest to verify against: the caller must
// trust the ref the same way actions/checkout would.
func (f *fetcher) fetchContents(ctx context.Context, repo string) ([]byte, error) {
	path := fmt.Sprintf("%s/contents/.github/%s?ref=%s", repo, url.PathEscape(f.o.Asset), url.QueryEscape(f.ref.Ref))
	resp, err := f.do(ctx, path, "application/vnd.github.raw+json")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxAssetSize+1))
		if err != nil {
			return nil, f.fail("reading .github/%s at %s: %v", f.o.Asset, f.ref.Ref, err)
		}
		if len(data) > maxAssetSize {
			return nil, f.fail(".github/%s at %s is larger than %d bytes; a config file should be a few kilobytes", f.o.Asset, f.ref.Ref, maxAssetSize)
		}
		f.log.Info().Str("extends", f.ref.String()).Str("path", ".github/"+f.o.Asset).Int("bytes", len(data)).Msg("extends config read from the repository at the ref")
		return data, nil
	case http.StatusNotFound:
		return nil, f.fail(".github/%s does not exist at ref %q in %s/%s%s; upload a release asset named %s, or add the file", f.o.Asset, f.ref.Ref, f.ref.Owner, f.ref.Repo, f.privateHint(), f.o.Asset)
	case http.StatusUnprocessableEntity:
		return nil, f.fail("ref %q does not exist in %s/%s%s", f.ref.Ref, f.ref.Owner, f.ref.Repo, f.privateHint())
	default:
		var body struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body)
		return nil, f.fail("GET %s returned %d: %s%s", path, resp.StatusCode, body.Message, f.privateHint())
	}
}

// getJSON decodes a 200 response into dst and returns the status. A 404 is
// returned as a status, not an error, so the caller can say what is missing.
func (f *fetcher) getJSON(ctx context.Context, path string, dst any) (int, error) {
	resp, err := f.do(ctx, path, "application/vnd.github+json")
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(dst); err != nil {
			return 0, f.fail("GET %s: decoding the response: %v", path, err)
		}
		return http.StatusOK, nil
	case http.StatusNotFound:
		return http.StatusNotFound, nil
	}
	var body struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body)
	return 0, f.fail("GET %s returned %d: %s%s", path, resp.StatusCode, body.Message, f.privateHint())
}

// do sends one GET, retrying transport errors, 429 and 5xx up to three
// times with backoff.
func (f *fetcher) do(ctx context.Context, path, accept string) (*http.Response, error) {
	u := strings.TrimSuffix(f.o.APIURL, "/") + path
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, f.fail("building the request for %s: %v", path, err)
		}
		req.Header.Set("Accept", accept)
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("User-Agent", "release-drafter-action")
		if f.o.Token != "" {
			req.Header.Set("Authorization", "Bearer "+f.o.Token)
		}
		start := time.Now()
		resp, err := f.client.Do(req)
		switch {
		case err != nil:
			lastErr = sanitizeTransportErr(err)
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			_ = resp.Body.Close()
		default:
			f.log.Debug().Str("method", "GET").Str("path", path).Int("status", resp.StatusCode).Dur("duration", time.Since(start)).Msg("extends request")
			return resp, nil
		}
		f.log.Warn().Str("path", path).Int("attempt", attempt).Err(lastErr).Dur("duration", time.Since(start)).Msg("extends request failed, retrying")
		if attempt < 3 {
			f.o.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	return nil, f.fail("GET %s failed after 3 attempts: %v", path, lastErr)
}

// cachePaths builds the cache file and its checksum sidecar under dir. ok
// is false when either path would land outside dir: ParseExtends already
// rejects owner/repo values that could cause that, and the asset name is
// checked by its caller, but this is a second guard so a cache read or
// write never trusts a path it did not build safely.
func cachePaths(dir string, ref ExtendsRef, asset string) (file, sumFile string, ok bool) {
	root := filepath.Clean(dir)
	base := filepath.Join(root, ref.Owner, ref.Repo, url.PathEscape(ref.Ref))
	file = filepath.Join(base, asset)
	sumFile = filepath.Join(base, asset+".sha256")
	return file, sumFile, pathUnder(root, file) && pathUnder(root, sumFile)
}

// pathUnder reports whether path is root itself or lies inside it.
func pathUnder(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

// readCache returns the cached asset when its recorded sha256 still
// matches. A damaged entry is ignored and fetched again.
func readCache(dir string, ref ExtendsRef, asset string, log *zerolog.Logger) ([]byte, bool) {
	if dir == "" {
		return nil, false
	}
	file, sumFile, ok := cachePaths(dir, ref, asset)
	if !ok {
		log.Warn().Str("extends", ref.String()).Msg("extends cache path would land outside the cache directory; skipping the cache")
		return nil, false
	}
	data, err := os.ReadFile(filepath.Clean(file))
	if err != nil {
		log.Debug().Str("extends", ref.String()).Msg("extends cache miss")
		return nil, false
	}
	want, err := os.ReadFile(filepath.Clean(sumFile))
	sum := sha256.Sum256(data)
	if err != nil || strings.TrimSpace(string(want)) != hex.EncodeToString(sum[:]) {
		log.Warn().Str("extends", ref.String()).Msg("extends cache entry does not match its checksum; fetching again")
		return nil, false
	}
	return data, true
}

// writeCache stores a verified asset. Failing to cache is logged, not fatal.
func writeCache(dir string, ref ExtendsRef, asset string, data []byte, log *zerolog.Logger) {
	if dir == "" {
		return
	}
	file, sumFile, ok := cachePaths(dir, ref, asset)
	if !ok {
		log.Warn().Str("extends", ref.String()).Msg("extends cache path would land outside the cache directory; not caching")
		return
	}
	sum := sha256.Sum256(data)
	err := os.MkdirAll(filepath.Dir(file), 0o750)
	if err == nil {
		err = os.WriteFile(file, data, 0o600)
	}
	if err == nil {
		err = os.WriteFile(sumFile, []byte(hex.EncodeToString(sum[:])+"\n"), 0o600)
	}
	if err != nil {
		log.Warn().Err(err).Str("extends", ref.String()).Msg("could not cache the extends config")
		return
	}
	log.Debug().Str("extends", ref.String()).Str("path", file).Msg("extends config cached")
}
