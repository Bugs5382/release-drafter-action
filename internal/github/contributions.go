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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Candidate is a pull request author considered for new-contributor status:
// the earliest mergedAt timestamp release-drafter found for that login.
type Candidate struct {
	Login    string
	MergedAt string
}

// NewContributorLogins reports which candidates had never had a merged
// pull request in owner/repo before their earliest one in this draft,
// ported from v7's findPreviousContributions: one GitHub code search per
// candidate, aliased into a single GraphQL request, counting merged pull
// requests by that author merged strictly before the candidate's own.
func (c *Client) NewContributorLogins(ctx context.Context, owner, repo string, candidates []Candidate) (map[string]bool, error) {
	out := map[string]bool{}
	if len(candidates) == 0 {
		return out, nil
	}
	c.log.Debug().Str("owner", owner).Str("repo", repo).Int("candidates", len(candidates)).
		Msg("github: checking for new contributors")

	declarations := make([]string, len(candidates))
	fields := make([]string, len(candidates))
	vars := map[string]any{}
	for i, cand := range candidates {
		name := "query" + strconv.Itoa(i)
		declarations[i] = "$" + name + ": String!"
		fields[i] = fmt.Sprintf("author%d: search(query: $%s, type: ISSUE, first: 1) { issueCount }", i, name)
		vars[name] = fmt.Sprintf("repo:%s/%s is:pr is:merged author:%s merged:<%s", owner, repo, cand.Login, cand.MergedAt)
	}
	query := fmt.Sprintf("query findPreviousContributions(%s) {\n%s\n}", strings.Join(declarations, ", "), strings.Join(fields, "\n"))

	var data map[string]json.RawMessage
	if err := c.doGraphQL(ctx, query, vars, &data); err != nil {
		c.log.Error().Err(err).Str("owner", owner).Str("repo", repo).Msg("github: failed to check for new contributors")
		return nil, fmt.Errorf("github: checking new contributors for %s/%s: %w", owner, repo, err)
	}
	for i, cand := range candidates {
		var result struct {
			IssueCount int `json:"issueCount"`
		}
		raw, ok := data["author"+strconv.Itoa(i)]
		if !ok {
			continue
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("github: decoding the new-contributor search result for %s: %w", cand.Login, err)
		}
		if result.IssueCount == 0 {
			out[cand.Login] = true
		}
	}
	c.log.Debug().Str("owner", owner).Str("repo", repo).Int("new_contributors", len(out)).Msg("github: checked for new contributors")
	return out, nil
}
