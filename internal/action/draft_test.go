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
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunDraftMatchesV7HubFixtureDryRun drives the whole draft mode (config
// load, the GitHub client, history and render) against a fake server
// serving the "hub" fixture, and checks the rendered body is still
// byte-identical to release-drafter v7.7.0's own output for it. Dry run so
// a would-be create or update call is a test failure, not a success.
func TestRunDraftMatchesV7HubFixtureDryRun(t *testing.T) {
	cfgData, err := os.ReadFile(filepath.Join("testdata", "hub-plain.yml"))
	if err != nil {
		t.Fatal(err)
	}

	f := newFakeGitHub(t)
	f.releases = releasesJSON(t, 1, "v1.2.3", "v1.2.3", "main", false)
	f.graphQL = func(query string, _ map[string]any) []byte {
		switch {
		case strings.Contains(query, "findCommitsInComparison"):
			return hubCompareResponse(t, "o", "r")
		case strings.Contains(query, "findRecentMergedPullRequests"):
			return emptyRecentMergedResponse(t)
		}
		t.Fatalf("unexpected graphql query: %s", query)
		return nil
	}
	f.createRelease = func(body []byte) []byte {
		t.Fatalf("dry run must not create a release; body: %s", body)
		return nil
	}
	f.updateRelease = func(id string, body []byte) []byte {
		t.Fatalf("dry run must not update a release %s; body: %s", id, body)
		return nil
	}

	outPath := filepath.Join(t.TempDir(), "output")
	var stdout bytes.Buffer
	code := Run(context.Background(), Options{
		Flags: Flags{Config: string(cfgData), DryRun: "true"},
		Env: Env{
			EventName: "push", Token: "test-token", Repository: "o/r", Ref: "refs/heads/main",
			ServerURL: "https://github.com", APIURL: f.srv.URL, GraphQLURL: f.srv.URL + "/graphql",
			OutputPath: outPath,
		},
		Stdout: &stdout,
	})
	if code != 0 {
		t.Fatalf("Run = %d, want 0; stdout: %s", code, stdout.String())
	}
	// hub-plain.yml still carries the deprecated categories[*].labels and
	// version-resolver shorthands (the canonical Bugs5382 config), so the
	// dry-run output is preceded by their migration warnings; only the
	// rendered body itself has to match v7 byte for byte.
	if got, want := stdout.String(), hubFixtureWantBody+"\n"; !strings.HasSuffix(got, want) {
		t.Errorf("dry-run body:\n--- got ---\n%s\n--- want suffix ---\n%s", got, want)
	}

	outData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(outData)
	for _, want := range []string{"tag_name<<", "v2.0.0", "resolved_version<<", "name<<"} {
		if !strings.Contains(out, want) {
			t.Errorf("outputs missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\nid<<") && !strings.Contains(out, "id<<ghadelimiter") {
		t.Errorf("outputs: id entry malformed:\n%s", out)
	}
}

// TestRunDraftCreatesReleaseAndWritesOutputs exercises the write path: no
// previous release (a first release), so the commit range comes from
// CommitsSince and $PREVIOUS_TAG falls back to the oldest commit in range;
// Upsert has nothing to update, so it creates a release, and the real
// create call's response feeds id, html_url and upload_url back into
// $GITHUB_OUTPUT.
func TestRunDraftCreatesReleaseAndWritesOutputs(t *testing.T) {
	const cfg = "template: \"$CHANGES\\nprev=$PREVIOUS_TAG\"\n" +
		"change-template: '- $TITLE (#$NUMBER)'\n" +
		"name-template: 'v$RESOLVED_VERSION'\n" +
		"tag-template: 'v$RESOLVED_VERSION'\n"

	oldOID := fmt.Sprintf("%040x", 101)
	newOID := fmt.Sprintf("%040x", 102)

	f := newFakeGitHub(t)
	f.releases = []byte("[]")
	f.graphQL = func(query string, _ map[string]any) []byte {
		switch {
		case strings.Contains(query, "findCommitsSince"):
			return graphQLEnvelope(t, map[string]any{
				"repository": map[string]any{
					"object": map[string]any{
						"history": map[string]any{
							"pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""},
							"nodes": []any{
								firstReleaseCommitNode(newOID, 102, "fix: second thing", "2026-02-02T00:00:00Z"),
								firstReleaseCommitNode(oldOID, 101, "feat: first thing", "2026-02-01T00:00:00Z"),
							},
						},
					},
				},
			})
		case strings.Contains(query, "findRecentMergedPullRequests"):
			return emptyRecentMergedResponse(t)
		}
		t.Fatalf("unexpected graphql query: %s", query)
		return nil
	}
	var created []byte
	f.createRelease = func(body []byte) []byte {
		created = body
		return mustJSON(t, map[string]any{
			"id": 55, "tag_name": "v0.0.1", "name": "v0.0.1", "draft": true, "prerelease": false,
			"target_commitish": "refs/heads/main", "created_at": "2026-02-02T00:00:00Z",
			"html_url":   "https://github.com/o/r/releases/tag/v0.0.1",
			"upload_url": "https://uploads.github.com/repos/o/r/releases/55/assets",
		})
	}

	outPath := filepath.Join(t.TempDir(), "output")
	var stdout bytes.Buffer
	code := Run(context.Background(), Options{
		Flags: Flags{Config: cfg},
		Env: Env{
			EventName: "push", Token: "test-token", Repository: "o/r", Ref: "refs/heads/main",
			ServerURL: "https://github.com", APIURL: f.srv.URL, GraphQLURL: f.srv.URL + "/graphql",
			OutputPath: outPath,
		},
		Stdout: &stdout,
	})
	if code != 0 {
		t.Fatalf("Run = %d, want 0; stdout: %s", code, stdout.String())
	}
	if created == nil {
		t.Fatal("CreateRelease was never called")
	}
	if !strings.Contains(string(created), "v0.0.1") {
		t.Errorf("create body missing the resolved tag: %s", created)
	}
	if !strings.Contains(string(created), "prev="+oldOID) {
		t.Errorf("create body missing the oldest commit as the first-release previous tag: %s", created)
	}

	outData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(outData)
	for _, want := range []string{"id<<", "55", "html_url<<", "upload_url<<"} {
		if !strings.Contains(out, want) {
			t.Errorf("outputs missing %q:\n%s", want, out)
		}
	}
}

// comparisonCommitsResponse builds a findCommitsInComparison response (the
// query CollectCommits issues when a last release is found) out of the same
// per-commit node shape firstReleaseCommitNodeWithLabels builds for
// findCommitsSince.
func comparisonCommitsResponse(t *testing.T, nodes []map[string]any) []byte {
	return graphQLEnvelope(t, map[string]any{
		"repository": map[string]any{
			"ref": map[string]any{
				"compare": map[string]any{
					"commits": map[string]any{
						"pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""},
						"nodes":    nodes,
					},
				},
			},
		},
	})
}

// TestRunDraftFirstReleaseWithPrereleaseDraftsRCOne covers issue #4: a
// brand-new repo's very first draft, with prerelease on and a
// prerelease-identifier, must come out as "<first-version>-<identifier>.1"
// rather than dropping the prerelease (the first-release path used to force
// "no_increment" unconditionally). This is the state Sneakers-PAM's first
// release starts from: first-version 0.1.0, prerelease true, identifier rc.
func TestRunDraftFirstReleaseWithPrereleaseDraftsRCOne(t *testing.T) {
	const cfg = "template: \"$CHANGES\"\n" +
		"change-template: '- $TITLE (#$NUMBER)'\n" +
		"name-template: 'v$RESOLVED_VERSION'\n" +
		"tag-template: 'v$RESOLVED_VERSION'\n"

	featOID := fmt.Sprintf("%040x", 301)
	fixOID := fmt.Sprintf("%040x", 302)

	f := newFakeGitHub(t)
	f.releases = []byte("[]")
	f.graphQL = func(query string, _ map[string]any) []byte {
		switch {
		case strings.Contains(query, "findCommitsSince"):
			return graphQLEnvelope(t, map[string]any{
				"repository": map[string]any{
					"object": map[string]any{
						"history": map[string]any{
							"pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""},
							"nodes": []any{
								firstReleaseCommitNode(fixOID, 302, "fix: handle empty input", "2026-03-02T00:00:00Z"),
								firstReleaseCommitNode(featOID, 301, "feat: add the first endpoint", "2026-03-01T00:00:00Z"),
							},
						},
					},
				},
			})
		case strings.Contains(query, "findRecentMergedPullRequests"):
			return emptyRecentMergedResponse(t)
		}
		t.Fatalf("unexpected graphql query: %s", query)
		return nil
	}
	var created []byte
	f.createRelease = func(body []byte) []byte {
		created = body
		return mustJSON(t, map[string]any{
			"id": 1, "tag_name": "v0.1.0-rc.1", "name": "v0.1.0-rc.1", "draft": true, "prerelease": true,
			"target_commitish": "refs/heads/main", "created_at": "2026-03-02T00:00:00Z",
			"html_url":   "https://github.com/o/r/releases/tag/v0.1.0-rc.1",
			"upload_url": "https://uploads.github.com/repos/o/r/releases/1/assets",
		})
	}
	f.updateRelease = func(id string, body []byte) []byte {
		t.Fatalf("a first release must create, not update; id %s, body: %s", id, body)
		return nil
	}

	outPath := filepath.Join(t.TempDir(), "output")
	var stdout bytes.Buffer
	code := Run(context.Background(), Options{
		Flags: Flags{Config: cfg, FirstVersion: "0.1.0", Prerelease: "true", PrereleaseIdentifier: "rc"},
		Env: Env{
			EventName: "push", Token: "test-token", Repository: "o/r", Ref: "refs/heads/main",
			ServerURL: "https://github.com", APIURL: f.srv.URL, GraphQLURL: f.srv.URL + "/graphql",
			OutputPath: outPath,
		},
		Stdout: &stdout,
	})
	if code != 0 {
		t.Fatalf("Run = %d, want 0; stdout: %s", code, stdout.String())
	}
	if created == nil {
		t.Fatal("CreateRelease was never called")
	}
	body := string(created)
	for _, want := range []string{`"tag_name":"v0.1.0-rc.1"`, `"name":"v0.1.0-rc.1"`, `"prerelease":true`, `"draft":true`} {
		if !strings.Contains(body, want) {
			t.Errorf("create body missing %q: %s", want, body)
		}
	}
	for _, want := range []string{"feat: add the first endpoint (#301)", "fix: handle empty input (#302)"} {
		if !strings.Contains(body, want) {
			t.Errorf("create body missing the merged pull request %q: %s", want, body)
		}
	}
	for _, banner := range []string{"could not find a previous published release", "No changes"} {
		if strings.Contains(body, banner) {
			t.Errorf("create body carries the no-previous-release banner %q: %s", banner, body)
		}
	}
}

// TestRunDraftNextRCBumpsFromPublished covers rc.2: a published v0.1.0-rc.1
// release exists, so FindPreviousReleases finds it as the last release
// (prerelease: true keeps a prerelease in that search), and the next draft
// keeps the same major.minor.patch, bumps the prerelease count to rc.2 and
// renders only the pull request merged since rc.1.
func TestRunDraftNextRCBumpsFromPublished(t *testing.T) {
	const cfg = "template: \"$CHANGES\"\n" +
		"change-template: '- $TITLE (#$NUMBER)'\n" +
		"name-template: 'v$RESOLVED_VERSION'\n" +
		"tag-template: 'v$RESOLVED_VERSION'\n"

	oid := fmt.Sprintf("%040x", 303)

	f := newFakeGitHub(t)
	f.releases = mustJSON(t, []map[string]any{{
		"id": 1, "tag_name": "v0.1.0-rc.1", "name": "v0.1.0-rc.1", "draft": false, "prerelease": true,
		"target_commitish": "refs/heads/main", "created_at": "2026-03-02T00:00:00Z",
		"html_url":   "https://github.com/o/r/releases/tag/v0.1.0-rc.1",
		"upload_url": "https://uploads.github.com/repos/o/r/releases/1/assets",
	}})
	var sawBaseRef string
	f.graphQL = func(query string, vars map[string]any) []byte {
		switch {
		case strings.Contains(query, "findCommitsInComparison"):
			if baseRef, ok := vars["baseRef"].(string); ok {
				sawBaseRef = baseRef
			}
			return comparisonCommitsResponse(t, []map[string]any{
				firstReleaseCommitNodeWithLabels(oid, 303, "fix: tighten validation", "2026-03-03T00:00:00Z", nil),
			})
		case strings.Contains(query, "findRecentMergedPullRequests"):
			return emptyRecentMergedResponse(t)
		}
		t.Fatalf("unexpected graphql query: %s", query)
		return nil
	}
	var created []byte
	f.createRelease = func(body []byte) []byte {
		created = body
		return mustJSON(t, map[string]any{
			"id": 2, "tag_name": "v0.1.0-rc.2", "name": "v0.1.0-rc.2", "draft": true, "prerelease": true,
			"target_commitish": "refs/heads/main", "created_at": "2026-03-03T00:00:00Z",
			"html_url":   "https://github.com/o/r/releases/tag/v0.1.0-rc.2",
			"upload_url": "https://uploads.github.com/repos/o/r/releases/2/assets",
		})
	}

	outPath := filepath.Join(t.TempDir(), "output")
	var stdout bytes.Buffer
	code := Run(context.Background(), Options{
		Flags: Flags{Config: cfg, Prerelease: "true", PrereleaseIdentifier: "rc"},
		Env: Env{
			EventName: "push", Token: "test-token", Repository: "o/r", Ref: "refs/heads/main",
			ServerURL: "https://github.com", APIURL: f.srv.URL, GraphQLURL: f.srv.URL + "/graphql",
			OutputPath: outPath,
		},
		Stdout: &stdout,
	})
	if code != 0 {
		t.Fatalf("Run = %d, want 0; stdout: %s", code, stdout.String())
	}
	if sawBaseRef != "refs/tags/v0.1.0-rc.1" {
		t.Errorf("findCommitsInComparison baseRef = %q, want %q", sawBaseRef, "refs/tags/v0.1.0-rc.1")
	}
	if created == nil {
		t.Fatal("CreateRelease was never called")
	}
	body := string(created)
	if !strings.Contains(body, `"tag_name":"v0.1.0-rc.2"`) {
		t.Errorf("create body did not bump to rc.2: %s", body)
	}
	if !strings.Contains(body, "fix: tighten validation (#303)") {
		t.Errorf("create body missing the pull request merged since rc.1: %s", body)
	}
}

// TestRunDraftPromotingDropsThePrerelease covers stabilizing: prerelease off
// with include-pre-releases on still finds the published v0.1.0-rc.1 as the
// last release (so the resolver has something to anchor to), and the
// promoted draft is v0.1.0 exactly -- the prerelease dropped, not a fresh
// major/minor/patch bump on top of it -- matching node-semver's own
// patch/minor/major-on-a-prerelease behavior.
func TestRunDraftPromotingDropsThePrerelease(t *testing.T) {
	const cfg = "template: \"$CHANGES\"\n" +
		"change-template: '- $TITLE (#$NUMBER)'\n" +
		"name-template: 'v$RESOLVED_VERSION'\n" +
		"tag-template: 'v$RESOLVED_VERSION'\n" +
		"version-resolver:\n  minor:\n    labels: [enhancement]\n  default: patch\n"

	oid := fmt.Sprintf("%040x", 304)

	f := newFakeGitHub(t)
	f.releases = mustJSON(t, []map[string]any{{
		"id": 1, "tag_name": "v0.1.0-rc.1", "name": "v0.1.0-rc.1", "draft": false, "prerelease": true,
		"target_commitish": "refs/heads/main", "created_at": "2026-03-02T00:00:00Z",
		"html_url":   "https://github.com/o/r/releases/tag/v0.1.0-rc.1",
		"upload_url": "https://uploads.github.com/repos/o/r/releases/1/assets",
	}})
	f.graphQL = func(query string, _ map[string]any) []byte {
		switch {
		case strings.Contains(query, "findCommitsInComparison"):
			return comparisonCommitsResponse(t, []map[string]any{
				firstReleaseCommitNodeWithLabels(oid, 304, "feat: add the second endpoint", "2026-03-04T00:00:00Z", []string{"enhancement"}),
			})
		case strings.Contains(query, "findRecentMergedPullRequests"):
			return emptyRecentMergedResponse(t)
		}
		t.Fatalf("unexpected graphql query: %s", query)
		return nil
	}
	var created []byte
	f.createRelease = func(body []byte) []byte {
		created = body
		return mustJSON(t, map[string]any{
			"id": 3, "tag_name": "v0.1.0", "name": "v0.1.0", "draft": true, "prerelease": false,
			"target_commitish": "refs/heads/main", "created_at": "2026-03-04T00:00:00Z",
			"html_url":   "https://github.com/o/r/releases/tag/v0.1.0",
			"upload_url": "https://uploads.github.com/repos/o/r/releases/3/assets",
		})
	}

	outPath := filepath.Join(t.TempDir(), "output")
	var stdout bytes.Buffer
	code := Run(context.Background(), Options{
		Flags: Flags{Config: cfg, IncludePreReleases: "true"},
		Env: Env{
			EventName: "push", Token: "test-token", Repository: "o/r", Ref: "refs/heads/main",
			ServerURL: "https://github.com", APIURL: f.srv.URL, GraphQLURL: f.srv.URL + "/graphql",
			OutputPath: outPath,
		},
		Stdout: &stdout,
	})
	if code != 0 {
		t.Fatalf("Run = %d, want 0; stdout: %s", code, stdout.String())
	}
	if created == nil {
		t.Fatal("CreateRelease was never called")
	}
	body := string(created)
	if !strings.Contains(body, `"tag_name":"v0.1.0"`) {
		t.Errorf("create body did not promote to the plain v0.1.0: %s", body)
	}
	if strings.Contains(body, `"tag_name":"v0.1.1"`) || strings.Contains(body, `"tag_name":"v0.2.0"`) {
		t.Errorf("create body bumped past the published rc instead of just dropping the prerelease: %s", body)
	}
	if !strings.Contains(body, `"prerelease":false`) {
		t.Errorf("create body still marked prerelease: %s", body)
	}
	if !strings.Contains(body, "feat: add the second endpoint (#304)") {
		t.Errorf("create body missing the pull request merged since the rc: %s", body)
	}
}

func firstReleaseCommitNode(oid string, number int, title, merged string) map[string]any {
	return firstReleaseCommitNodeWithLabels(oid, number, title, merged, nil)
}

func firstReleaseCommitNodeWithLabels(oid string, number int, title, merged string, labels []string) map[string]any {
	var labelNodes []any
	for _, l := range labels {
		labelNodes = append(labelNodes, map[string]any{"__typename": "Label", "name": l})
	}
	return map[string]any{
		"oid": oid, "committedDate": merged, "message": title,
		"author":  map[string]any{"name": "Dana", "user": map[string]any{"login": "dana"}},
		"authors": map[string]any{"nodes": []any{}},
		"associatedPullRequests": map[string]any{"nodes": []any{map[string]any{
			"__typename":     "PullRequest",
			"title":          title,
			"number":         number,
			"author":         map[string]any{"__typename": "User", "login": "dana", "url": "https://github.com/dana"},
			"baseRepository": map[string]any{"__typename": "Repository", "nameWithOwner": "o/r"},
			"mergedAt":       merged,
			"labels":         map[string]any{"__typename": "LabelConnection", "nodes": labelNodes},
			"merged":         true,
		}}},
	}
}

// TestRunDraftFirstReleaseRewritesExistingDraftToFirstVersion reproduces
// issue #5's bug report: an existing draft release left over at v0.1.0 (the
// way v7's 0.0.0-plus-increment fallback resolves a first release with a
// minor-labeled pull request in range), with the first-version input set
// the way job-release-drafter.yaml's production default does. The update
// must rewrite the draft to the pinned first-version tag, not recompute
// v0.1.0 again.
func TestRunDraftFirstReleaseRewritesExistingDraftToFirstVersion(t *testing.T) {
	const cfg = "template: \"$CHANGES\"\n" +
		"change-template: '- $TITLE (#$NUMBER)'\n" +
		"name-template: 'v$RESOLVED_VERSION'\n" +
		"tag-template: 'v$RESOLVED_VERSION'\n" +
		"version-resolver:\n  minor:\n    labels: [enhancement]\n  default: patch\n"

	oid := fmt.Sprintf("%040x", 201)

	f := newFakeGitHub(t)
	f.releases = releasesJSON(t, 99, "v0.1.0", "v0.1.0", "refs/heads/main", true)
	f.graphQL = func(query string, _ map[string]any) []byte {
		switch {
		case strings.Contains(query, "findCommitsSince"):
			return graphQLEnvelope(t, map[string]any{
				"repository": map[string]any{
					"object": map[string]any{
						"history": map[string]any{
							"pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""},
							"nodes": []any{
								firstReleaseCommitNodeWithLabels(oid, 201, "feat: first thing", "2026-02-01T00:00:00Z", []string{"enhancement"}),
							},
						},
					},
				},
			})
		case strings.Contains(query, "findRecentMergedPullRequests"):
			return emptyRecentMergedResponse(t)
		}
		t.Fatalf("unexpected graphql query: %s", query)
		return nil
	}
	var updatedID string
	var updated []byte
	f.updateRelease = func(id string, body []byte) []byte {
		updatedID = id
		updated = body
		return mustJSON(t, map[string]any{
			"id": 99, "tag_name": "v1.0.0", "name": "v1.0.0", "draft": true, "prerelease": false,
			"target_commitish": "refs/heads/main", "created_at": "2026-01-01T00:00:00Z",
			"html_url":   "https://github.com/o/r/releases/tag/v1.0.0",
			"upload_url": "https://uploads.github.com/repos/o/r/releases/99/assets",
		})
	}
	f.createRelease = func(body []byte) []byte {
		t.Fatalf("an existing draft must be updated, not recreated; body: %s", body)
		return nil
	}

	outPath := filepath.Join(t.TempDir(), "output")
	var stdout bytes.Buffer
	code := Run(context.Background(), Options{
		Flags: Flags{Config: cfg, FirstVersion: "1.0.0"},
		Env: Env{
			EventName: "push", Token: "test-token", Repository: "o/r", Ref: "refs/heads/main",
			ServerURL: "https://github.com", APIURL: f.srv.URL, GraphQLURL: f.srv.URL + "/graphql",
			OutputPath: outPath,
		},
		Stdout: &stdout,
	})
	if code != 0 {
		t.Fatalf("Run = %d, want 0; stdout: %s", code, stdout.String())
	}
	if updatedID != "99" {
		t.Fatalf("UpdateRelease was not called with the existing draft's id; got %q", updatedID)
	}
	if !strings.Contains(string(updated), `"tag_name":"v1.0.0"`) {
		t.Errorf("update body did not pin the tag to v1.0.0: %s", updated)
	}
	if !strings.Contains(string(updated), `"name":"v1.0.0"`) {
		t.Errorf("update body did not pin the name to v1.0.0: %s", updated)
	}
	if strings.Contains(string(updated), "v0.1.0") {
		t.Errorf("update body still carries the stale v0.1.0 tag: %s", updated)
	}
}
