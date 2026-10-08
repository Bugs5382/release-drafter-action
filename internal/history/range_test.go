package history

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
	"testing"

	"github.com/Bugs5382/release-drafter-action/internal/github"
	"github.com/Bugs5382/release-drafter-action/internal/model"
)

// fakeRangeClient fakes RangeClient. It records every call so a test can
// assert which path CollectCommits took without a network fake.
type fakeRangeClient struct {
	inRangeCalls []struct{ baseRef, headRef string }
	sinceCalls   []struct {
		target string
		since  *string
	}
	commits []model.Commit
	err     error
}

func (f *fakeRangeClient) CommitsInRange(_ context.Context, _, _, baseRef, headRef string, _ github.CommitFieldOptions) ([]model.Commit, error) {
	f.inRangeCalls = append(f.inRangeCalls, struct{ baseRef, headRef string }{baseRef, headRef})
	return f.commits, f.err
}

func (f *fakeRangeClient) CommitsSince(_ context.Context, _, _, target string, since *string, _ github.CommitFieldOptions) ([]model.Commit, error) {
	f.sinceCalls = append(f.sinceCalls, struct {
		target string
		since  *string
	}{target, since})
	return f.commits, f.err
}

func TestCollectCommitsComparesAgainstTheLastReleaseTag(t *testing.T) {
	f := &fakeRangeClient{commits: []model.Commit{{OID: "a"}}}
	commits, err := CollectCommits(context.Background(), f, RangeInput{
		Owner: "acme", Repo: "widget", Commitish: "main",
		LastRelease: &model.Release{TagName: "v1.0.0"},
	})
	if err != nil {
		t.Fatalf("CollectCommits: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("commits = %v", commits)
	}
	if len(f.inRangeCalls) != 1 || f.inRangeCalls[0].baseRef != "refs/tags/v1.0.0" || f.inRangeCalls[0].headRef != "main" {
		t.Fatalf("inRangeCalls = %+v", f.inRangeCalls)
	}
	if len(f.sinceCalls) != 0 {
		t.Fatalf("sinceCalls = %+v, want none", f.sinceCalls)
	}
}

func TestCollectCommitsWalksWholeHistoryOnFirstReleaseWithNoSince(t *testing.T) {
	f := &fakeRangeClient{}
	_, err := CollectCommits(context.Background(), f, RangeInput{Owner: "acme", Repo: "widget", Commitish: "main"})
	if err != nil {
		t.Fatalf("CollectCommits: %v", err)
	}
	if len(f.sinceCalls) != 1 || f.sinceCalls[0].target != "main" || f.sinceCalls[0].since != nil {
		t.Fatalf("sinceCalls = %+v", f.sinceCalls)
	}
}

func TestCollectCommitsFirstReleaseWithADateSince(t *testing.T) {
	f := &fakeRangeClient{}
	_, err := CollectCommits(context.Background(), f, RangeInput{Owner: "acme", Repo: "widget", Commitish: "main", Since: "2026-01-01"})
	if err != nil {
		t.Fatalf("CollectCommits: %v", err)
	}
	if len(f.sinceCalls) != 1 || f.sinceCalls[0].since == nil || *f.sinceCalls[0].since != "2026-01-01T00:00:00Z" {
		t.Fatalf("sinceCalls = %+v", f.sinceCalls)
	}
}

func TestCollectCommitsFirstReleaseWithATagOrSHASince(t *testing.T) {
	cases := []string{"v0.9.0", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}
	for _, since := range cases {
		f := &fakeRangeClient{}
		_, err := CollectCommits(context.Background(), f, RangeInput{Owner: "acme", Repo: "widget", Commitish: "main", Since: since})
		if err != nil {
			t.Fatalf("CollectCommits(%s): %v", since, err)
		}
		if len(f.inRangeCalls) != 1 || f.inRangeCalls[0].baseRef != since || f.inRangeCalls[0].headRef != "main" {
			t.Fatalf("CollectCommits(%s): inRangeCalls = %+v", since, f.inRangeCalls)
		}
	}
}

func TestParseSinceClassifiesDatesTagsAndSHAs(t *testing.T) {
	if date, expr := ParseSince(""); date != "" || expr != "" {
		t.Errorf("ParseSince(\"\") = %q, %q", date, expr)
	}
	if date, expr := ParseSince("v1.2.3"); date != "" || expr != "v1.2.3" {
		t.Errorf("ParseSince(tag) = %q, %q", date, expr)
	}
	sha := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	if date, expr := ParseSince(sha); date != "" || expr != sha {
		t.Errorf("ParseSince(sha) = %q, %q", date, expr)
	}
	if date, expr := ParseSince("2026-01-01"); date != "2026-01-01T00:00:00Z" || expr != "" {
		t.Errorf("ParseSince(date) = %q, %q", date, expr)
	}
	if date, expr := ParseSince("2026-01-01T12:30:00Z"); date != "2026-01-01T12:30:00Z" || expr != "" {
		t.Errorf("ParseSince(datetime) = %q, %q", date, expr)
	}
}
