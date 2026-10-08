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

type fakePullRequestsClient struct {
	recentBaseRefName *string
	recentLimit       int
	recent            []model.PullRequest
	recentErr         error

	changedFilesByNumber map[int][]string
	changedFilesErr      error

	candidates         []github.Candidate
	newContributors    map[string]bool
	newContributorsErr error
}

func (f *fakePullRequestsClient) RecentMergedPullRequests(_ context.Context, _, _ string, baseRefName *string, limit int, _ github.CommitFieldOptions) ([]model.PullRequest, error) {
	f.recentBaseRefName = baseRefName
	f.recentLimit = limit
	return f.recent, f.recentErr
}

func (f *fakePullRequestsClient) ChangedFiles(_ context.Context, _, _ string, number int) ([]string, error) {
	if f.changedFilesErr != nil {
		return nil, f.changedFilesErr
	}
	return f.changedFilesByNumber[number], nil
}

func (f *fakePullRequestsClient) NewContributorLogins(_ context.Context, _, _ string, candidates []github.Candidate) (map[string]bool, error) {
	f.candidates = candidates
	if f.newContributorsErr != nil {
		return nil, f.newContributorsErr
	}
	return f.newContributors, nil
}

func pr(number int, baseRepo string, merged bool, author *model.Actor, mergedAt *string) model.PullRequest {
	return model.PullRequest{Number: number, BaseRepository: baseRepo, Merged: merged, Author: author, MergedAt: mergedAt}
}

func ptr(s string) *string { return &s }

func TestCollectPullRequestsDedupesAcrossCommitsAndFiltersToTheRepo(t *testing.T) {
	commits := []model.Commit{
		{OID: "a", PullRequests: []model.PullRequest{pr(1, "acme/widget", true, nil, nil)}},
		{OID: "b", PullRequests: []model.PullRequest{
			pr(1, "acme/widget", true, nil, nil),
			pr(2, "acme/widget", true, nil, nil),
			pr(3, "other/fork", true, nil, nil),
		}},
	}
	f := &fakePullRequestsClient{}
	prs, contributors, err := CollectPullRequests(context.Background(), f, CollectInput{
		Owner: "acme", Repo: "widget", Commits: commits, Commitish: "refs/heads/main",
	})
	if err != nil {
		t.Fatalf("CollectPullRequests: %v", err)
	}
	if len(prs) != 2 || prs[0].Number != 1 || prs[1].Number != 2 {
		t.Fatalf("prs = %+v, want #1 and #2 (deduped, #3 excluded as a different repo)", prs)
	}
	if contributors == nil || len(contributors) != 0 {
		t.Fatalf("contributors = %v, want empty but non-nil", contributors)
	}
}

func TestCollectPullRequestsExcludesUnmergedPullRequests(t *testing.T) {
	commits := []model.Commit{{OID: "a", PullRequests: []model.PullRequest{pr(1, "acme/widget", false, nil, nil)}}}
	f := &fakePullRequestsClient{}
	prs, _, err := CollectPullRequests(context.Background(), f, CollectInput{Owner: "acme", Repo: "widget", Commits: commits, Commitish: "main"})
	if err != nil {
		t.Fatalf("CollectPullRequests: %v", err)
	}
	if len(prs) != 0 {
		t.Fatalf("prs = %+v, want none (not merged)", prs)
	}
}

func TestCollectPullRequestsRecoversFromRecentMergedWhenMissingFromAssociation(t *testing.T) {
	commits := []model.Commit{{OID: "mergecommitoid"}}
	f := &fakePullRequestsClient{recent: []model.PullRequest{
		{Number: 9, BaseRepository: "acme/widget", Merged: true, MergeCommitOID: "mergecommitoid"},
		{Number: 10, BaseRepository: "acme/widget", Merged: true, MergeCommitOID: "not-in-range"},
	}}
	prs, _, err := CollectPullRequests(context.Background(), f, CollectInput{
		Owner: "acme", Repo: "widget", Commits: commits, Commitish: "refs/heads/main",
	})
	if err != nil {
		t.Fatalf("CollectPullRequests: %v", err)
	}
	if len(prs) != 1 || prs[0].Number != 9 {
		t.Fatalf("prs = %+v, want only #9 (its merge commit is in range)", prs)
	}
	if f.recentBaseRefName == nil || *f.recentBaseRefName != "main" {
		t.Fatalf("recentBaseRefName = %v, want main (branch ref)", f.recentBaseRefName)
	}
}

func TestCollectPullRequestsSkipsRecoveryForTagAndPullRefs(t *testing.T) {
	for _, commitish := range []string{"refs/tags/v1.0.0", "refs/pull/5/merge"} {
		commits := []model.Commit{{OID: "a"}}
		f := &fakePullRequestsClient{recent: []model.PullRequest{{Number: 1, BaseRepository: "acme/widget", Merged: true, MergeCommitOID: "a"}}}
		prs, _, err := CollectPullRequests(context.Background(), f, CollectInput{Owner: "acme", Repo: "widget", Commits: commits, Commitish: commitish})
		if err != nil {
			t.Fatalf("CollectPullRequests(%s): %v", commitish, err)
		}
		if len(prs) != 0 {
			t.Fatalf("CollectPullRequests(%s): prs = %+v, want none (recovery skipped)", commitish, prs)
		}
		if f.recentBaseRefName != nil || f.recentLimit != 0 {
			t.Fatalf("CollectPullRequests(%s): recovery query was still made", commitish)
		}
	}
}

func TestCollectPullRequestsDoesNotRecoverAnAlreadyFoundPullRequest(t *testing.T) {
	commits := []model.Commit{{OID: "mergecommitoid", PullRequests: []model.PullRequest{pr(9, "acme/widget", true, nil, nil)}}}
	f := &fakePullRequestsClient{recent: []model.PullRequest{{Number: 9, BaseRepository: "acme/widget", Merged: true, MergeCommitOID: "mergecommitoid"}}}
	prs, _, err := CollectPullRequests(context.Background(), f, CollectInput{Owner: "acme", Repo: "widget", Commits: commits, Commitish: "main"})
	if err != nil {
		t.Fatalf("CollectPullRequests: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("prs = %+v, want exactly one (not duplicated by recovery)", prs)
	}
}

func TestCollectPullRequestsLoadsChangedFilesWhenNeeded(t *testing.T) {
	commits := []model.Commit{{OID: "a", PullRequests: []model.PullRequest{pr(1, "acme/widget", true, nil, nil)}}}
	f := &fakePullRequestsClient{changedFilesByNumber: map[int][]string{1: {"a.go", "b.go"}}}
	prs, _, err := CollectPullRequests(context.Background(), f, CollectInput{
		Owner: "acme", Repo: "widget", Commits: commits, Commitish: "main", NeedsChangedFiles: true,
	})
	if err != nil {
		t.Fatalf("CollectPullRequests: %v", err)
	}
	if len(prs) != 1 || len(prs[0].ChangedFiles) != 2 {
		t.Fatalf("prs = %+v", prs)
	}
}

func TestCollectPullRequestsFindsNewContributorsFromHumanAuthorsOnly(t *testing.T) {
	alice := &model.Actor{Typename: "User", Login: "alice"}
	bot := &model.Actor{Typename: "Bot", Login: "renovate"}
	commits := []model.Commit{{OID: "a", PullRequests: []model.PullRequest{
		pr(1, "acme/widget", true, alice, ptr("2026-02-01T00:00:00Z")),
		pr(2, "acme/widget", true, bot, ptr("2026-02-02T00:00:00Z")),
	}}}
	f := &fakePullRequestsClient{newContributors: map[string]bool{"alice": true}}
	_, contributors, err := CollectPullRequests(context.Background(), f, CollectInput{
		Owner: "acme", Repo: "widget", Commits: commits, Commitish: "main", NeedsNewContributors: true,
	})
	if err != nil {
		t.Fatalf("CollectPullRequests: %v", err)
	}
	if !contributors["alice"] {
		t.Fatalf("contributors = %v, want alice", contributors)
	}
	if len(f.candidates) != 1 || f.candidates[0].Login != "alice" {
		t.Fatalf("candidates = %+v, want only the human author", f.candidates)
	}
}
