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
	"fmt"

	"github.com/Bugs5382/release-drafter-action/internal/model"
)

// CommitFieldOptions picks which optional pull request fields the
// associated-pull-request queries ask for, and the two pagination limits
// v7 exposes as config: pull-request-limit (per commit) and history-limit
// (commits per page).
type CommitFieldOptions struct {
	WithPullRequestBody bool
	WithPullRequestURL  bool
	WithBaseRefName     bool
	WithHeadRefName     bool
	// PullRequestLimit caps associatedPullRequests per commit.
	PullRequestLimit int
	// HistoryLimit caps commits fetched per GraphQL page.
	HistoryLimit int
}

func (o CommitFieldOptions) withDefaults() CommitFieldOptions {
	out := o
	if out.PullRequestLimit <= 0 {
		out.PullRequestLimit = 1
	}
	if out.HistoryLimit <= 0 {
		out.HistoryLimit = 100
	}
	return out
}

func (o CommitFieldOptions) variables() map[string]any {
	return map[string]any{
		"withPullRequestBody": o.WithPullRequestBody,
		"withPullRequestURL":  o.WithPullRequestURL,
		"withBaseRefName":     o.WithBaseRefName,
		"withHeadRefName":     o.WithHeadRefName,
		"pullRequestLimit":    o.PullRequestLimit,
		"historyLimit":        o.HistoryLimit,
	}
}

// CommitsInRange walks every commit GitHub reports between baseRef and
// headRef (a GitHub "compare", the same direction git log baseRef..headRef
// walks), each with its associated pull requests. baseRef is normally
// "refs/tags/<lastRelease tag>".
func (c *Client) CommitsInRange(ctx context.Context, owner, repo, baseRef, headRef string, fields CommitFieldOptions) ([]model.Commit, error) {
	fields = fields.withDefaults()
	c.log.Debug().Str("owner", owner).Str("repo", repo).Str("base_ref", baseRef).Str("head_ref", headRef).
		Msg("github: comparing commits")
	var out []model.Commit
	cursor := ""
	for page := 1; ; page++ {
		vars := fields.variables()
		vars["name"], vars["owner"], vars["baseRef"], vars["headRef"] = repo, owner, baseRef, headRef
		if cursor != "" {
			vars["cursor"] = cursor
		}
		var data struct {
			Repository *struct {
				Ref *struct {
					Compare *struct {
						Commits struct {
							PageInfo gqlPageInfo     `json:"pageInfo"`
							Nodes    []gqlCommitNode `json:"nodes"`
						} `json:"commits"`
					} `json:"compare"`
				} `json:"ref"`
			} `json:"repository"`
		}
		if err := c.doGraphQL(ctx, findCommitsInComparisonQuery, vars, &data); err != nil {
			c.log.Error().Err(err).Str("owner", owner).Str("repo", repo).Str("base_ref", baseRef).
				Msg("github: failed to compare commits")
			return nil, fmt.Errorf("github: comparing %s...%s for %s/%s: %w", baseRef, headRef, owner, repo, err)
		}
		if data.Repository == nil || data.Repository.Ref == nil || data.Repository.Ref.Compare == nil {
			return nil, fmt.Errorf("github: comparing %s...%s for %s/%s: ref or comparison not found", baseRef, headRef, owner, repo)
		}
		commits := data.Repository.Ref.Compare.Commits
		for _, n := range commits.Nodes {
			out = append(out, n.toModel())
		}
		c.log.Trace().Int("page", page).Int("commits", len(commits.Nodes)).Bool("has_next_page", commits.PageInfo.HasNextPage).
			Msg("github: fetched a page of compared commits")
		if !commits.PageInfo.HasNextPage {
			break
		}
		cursor = commits.PageInfo.EndCursor
	}
	c.log.Info().Str("owner", owner).Str("repo", repo).Int("commits", len(out)).Msg("github: compared commits")
	return out, nil
}

// CommitsSince walks the history reachable from target, trimmed to commits
// committed at or after since when it is non-nil. This is the first-release
// walk: with no earlier published release there is no tag to compare
// against, so the whole history (or everything after since) stands in for
// the commit range.
func (c *Client) CommitsSince(ctx context.Context, owner, repo, target string, since *string, fields CommitFieldOptions) ([]model.Commit, error) {
	fields = fields.withDefaults()
	c.log.Debug().Str("owner", owner).Str("repo", repo).Str("target", target).Msg("github: walking commit history")
	var out []model.Commit
	cursor := ""
	for page := 1; ; page++ {
		vars := fields.variables()
		vars["name"], vars["owner"], vars["target"] = repo, owner, target
		if since != nil {
			vars["since"] = *since
		}
		if cursor != "" {
			vars["cursor"] = cursor
		}
		var data struct {
			Repository *struct {
				Object *struct {
					History *struct {
						PageInfo gqlPageInfo     `json:"pageInfo"`
						Nodes    []gqlCommitNode `json:"nodes"`
					} `json:"history"`
				} `json:"object"`
			} `json:"repository"`
		}
		if err := c.doGraphQL(ctx, findCommitsSinceQuery, vars, &data); err != nil {
			c.log.Error().Err(err).Str("owner", owner).Str("repo", repo).Str("target", target).
				Msg("github: failed to walk commit history")
			return nil, fmt.Errorf("github: walking history of %s for %s/%s: %w", target, owner, repo, err)
		}
		if data.Repository == nil || data.Repository.Object == nil || data.Repository.Object.History == nil {
			return nil, fmt.Errorf("github: walking history of %s for %s/%s: target is not a commit", target, owner, repo)
		}
		history := data.Repository.Object.History
		for _, n := range history.Nodes {
			out = append(out, n.toModel())
		}
		c.log.Trace().Int("page", page).Int("commits", len(history.Nodes)).Bool("has_next_page", history.PageInfo.HasNextPage).
			Msg("github: fetched a page of commit history")
		if !history.PageInfo.HasNextPage {
			break
		}
		cursor = history.PageInfo.EndCursor
	}
	c.log.Info().Str("owner", owner).Str("repo", repo).Int("commits", len(out)).Msg("github: walked commit history")
	return out, nil
}
