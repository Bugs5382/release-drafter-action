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
	"fmt"
	"strings"

	"github.com/Bugs5382/release-drafter-action/internal/github"
	"github.com/Bugs5382/release-drafter-action/internal/logging"
	"github.com/Bugs5382/release-drafter-action/internal/model"
)

// defaultRecentLookback mirrors v7's RECENT_PR_LOOKBACK.
const defaultRecentLookback = 5

// PullRequestsClient is what CollectPullRequests needs from a
// *github.Client.
type PullRequestsClient interface {
	RecentMergedPullRequests(ctx context.Context, owner, repo string, baseRefName *string, limit int, fields github.CommitFieldOptions) ([]model.PullRequest, error)
	ChangedFiles(ctx context.Context, owner, repo string, number int) ([]string, error)
	NewContributorLogins(ctx context.Context, owner, repo string, candidates []github.Candidate) (map[string]bool, error)
}

// CollectInput is what CollectPullRequests needs to turn a commit range
// into the pull requests merged in it.
type CollectInput struct {
	Owner, Repo string
	// Commits is the range CollectCommits found.
	Commits []model.Commit
	// Commitish is the release target; it decides whether the recovery
	// query can be scoped to one branch.
	Commitish            string
	Fields               github.CommitFieldOptions
	NeedsChangedFiles    bool
	NeedsNewContributors bool
	// RecentLookback overrides defaultRecentLookback; 0 means the default.
	RecentLookback int
}

// CollectPullRequests ports v7's findPullRequests: the pull requests
// associated with the commits in range, plus any very recently merged pull
// request GitHub's associatedPullRequests index has not caught up with
// yet, filtered to merged pull requests that target owner/repo.
// newContributors is never nil, even when it is empty.
func CollectPullRequests(ctx context.Context, client PullRequestsClient, in CollectInput) ([]model.PullRequest, map[string]bool, error) {
	logging.L().Debug().Str("owner", in.Owner).Str("repo", in.Repo).Int("commits", len(in.Commits)).
		Msg("history: collecting pull requests from the commit range")

	order, byKey := dedupeAssociated(in.Commits)

	recovered, err := recoverRecentlyMerged(ctx, client, in, byKey)
	if err != nil {
		return nil, nil, err
	}

	nameWithOwner := in.Owner + "/" + in.Repo
	prs := make([]model.PullRequest, 0, len(order)+len(recovered))
	for _, key := range order {
		if pr := byKey[key]; pr.BaseRepository == nameWithOwner && pr.Merged {
			prs = append(prs, pr)
		}
	}
	for _, pr := range recovered {
		if pr.BaseRepository == nameWithOwner && pr.Merged {
			prs = append(prs, pr)
		}
	}

	if in.NeedsChangedFiles {
		for i := range prs {
			files, err := client.ChangedFiles(ctx, in.Owner, in.Repo, prs[i].Number)
			if err != nil {
				return nil, nil, fmt.Errorf("history: loading changed files for %s#%d: %w", nameWithOwner, prs[i].Number, err)
			}
			prs[i].ChangedFiles = files
		}
	}

	newContributors := map[string]bool{}
	if in.NeedsNewContributors {
		newContributors, err = client.NewContributorLogins(ctx, in.Owner, in.Repo, newContributorCandidates(prs))
		if err != nil {
			return nil, nil, fmt.Errorf("history: checking for new contributors in %s: %w", nameWithOwner, err)
		}
	}

	logging.L().Info().Str("owner", in.Owner).Str("repo", in.Repo).Int("pull_requests", len(prs)).
		Int("new_contributors", len(newContributors)).Msg("history: collected pull requests")
	return prs, newContributors, nil
}

// dedupeAssociated collects every pull request associated with the commits,
// deduplicated by Key(), in first-seen order.
func dedupeAssociated(commits []model.Commit) (order []string, byKey map[string]model.PullRequest) {
	byKey = map[string]model.PullRequest{}
	for _, commit := range commits {
		for _, pr := range commit.PullRequests {
			key := pr.Key()
			if _, ok := byKey[key]; !ok {
				order = append(order, key)
			}
			byKey[key] = pr
		}
	}
	return order, byKey
}

func recoverRecentlyMerged(ctx context.Context, client PullRequestsClient, in CollectInput, foundByKey map[string]model.PullRequest) ([]model.PullRequest, error) {
	oids := map[string]bool{}
	for _, c := range in.Commits {
		if c.OID != "" {
			oids[c.OID] = true
		}
	}
	isBranchRef := strings.HasPrefix(in.Commitish, "refs/heads/")
	isUnsupportedRef := strings.HasPrefix(in.Commitish, "refs/tags/") || strings.HasPrefix(in.Commitish, "refs/pull/")
	if len(oids) == 0 || isUnsupportedRef {
		return nil, nil
	}

	var baseRefName *string
	if isBranchRef {
		name := strings.TrimPrefix(in.Commitish, "refs/heads/")
		baseRefName = &name
	}
	limit := in.RecentLookback
	if limit <= 0 {
		limit = defaultRecentLookback
	}
	candidates, err := client.RecentMergedPullRequests(ctx, in.Owner, in.Repo, baseRefName, limit, in.Fields)
	if err != nil {
		return nil, fmt.Errorf("history: finding recently merged pull requests for %s/%s: %w", in.Owner, in.Repo, err)
	}

	var recovered []model.PullRequest
	for _, pr := range candidates {
		if pr.MergeCommitOID == "" || !oids[pr.MergeCommitOID] {
			continue
		}
		if _, found := foundByKey[pr.Key()]; found {
			continue
		}
		recovered = append(recovered, pr)
	}
	if len(recovered) > 0 {
		numbers := make([]int, len(recovered))
		for i, pr := range recovered {
			numbers[i] = pr.Number
		}
		logging.L().Info().Ints("recovered_pull_requests", numbers).
			Msg("history: recovered pull requests missing from the associated-pull-requests index")
	}
	return recovered, nil
}

// newContributorCandidates finds, for every human (non-bot) pull request
// author, the earliest mergedAt among the pull requests in prs, ported from
// v7's findNewContributorLogins.
func newContributorCandidates(prs []model.PullRequest) []github.Candidate {
	earliest := map[string]string{}
	var order []string
	for _, pr := range prs {
		if pr.Author == nil || pr.Author.Typename != "User" || pr.MergedAt == nil {
			continue
		}
		login := pr.Author.Login
		prev, ok := earliest[login]
		if !ok {
			order = append(order, login)
		}
		if !ok || *pr.MergedAt < prev {
			earliest[login] = *pr.MergedAt
		}
	}
	out := make([]github.Candidate, 0, len(order))
	for _, login := range order {
		out = append(out, github.Candidate{Login: login, MergedAt: earliest[login]})
	}
	return out
}
