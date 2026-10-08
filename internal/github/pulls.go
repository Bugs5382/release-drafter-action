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

// RecentMergedPullRequests reads the most recently merged pull requests
// directly from the pull request table, each with its merge commit's OID.
// GitHub's associatedPullRequests index lags for very recently merged pull
// requests, so the caller uses this to recover one whose merge commit
// falls inside the range but was missing from the commit walk.
// baseRefName narrows the search to one branch when the commitish is a
// confirmed branch ref; nil asks for every branch.
func (c *Client) RecentMergedPullRequests(ctx context.Context, owner, repo string, baseRefName *string, limit int, fields CommitFieldOptions) ([]model.PullRequest, error) {
	if limit <= 0 {
		limit = 5
	}
	c.log.Debug().Str("owner", owner).Str("repo", repo).Int("limit", limit).Msg("github: finding recently merged pull requests")
	vars := fields.variables()
	vars["name"], vars["owner"], vars["limit"] = repo, owner, limit
	if baseRefName != nil {
		vars["baseRefName"] = *baseRefName
	}
	var data struct {
		Repository *struct {
			PullRequests struct {
				Nodes []gqlPullRequest `json:"nodes"`
			} `json:"pullRequests"`
		} `json:"repository"`
	}
	if err := c.doGraphQL(ctx, findRecentMergedPullRequestsQuery, vars, &data); err != nil {
		c.log.Error().Err(err).Str("owner", owner).Str("repo", repo).Msg("github: failed to find recently merged pull requests")
		return nil, fmt.Errorf("github: finding recently merged pull requests for %s/%s: %w", owner, repo, err)
	}
	if data.Repository == nil {
		return nil, fmt.Errorf("github: finding recently merged pull requests for %s/%s: repository not found", owner, repo)
	}
	out := make([]model.PullRequest, 0, len(data.Repository.PullRequests.Nodes))
	for _, n := range data.Repository.PullRequests.Nodes {
		out = append(out, n.toModel())
	}
	c.log.Debug().Str("owner", owner).Str("repo", repo).Int("found", len(out)).Msg("github: found recently merged pull requests")
	return out, nil
}
