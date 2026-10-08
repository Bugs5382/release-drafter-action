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
)

// ResolveCommitish resolves a git expression (a tag, SHA, branch or
// "<ref>^{commit}") to the commit SHA it points at. It fails when the
// expression does not resolve to a commit at all.
func (c *Client) ResolveCommitish(ctx context.Context, owner, repo, expression string) (string, error) {
	c.log.Debug().Str("owner", owner).Str("repo", repo).Str("expression", expression).Msg("github: resolving a commitish")
	vars := map[string]any{"name": repo, "owner": owner, "expression": expression}
	var data struct {
		Repository *struct {
			Object *struct {
				Typename string `json:"__typename"`
				Oid      string `json:"oid"`
			} `json:"object"`
		} `json:"repository"`
	}
	if err := c.doGraphQL(ctx, resolveCommitishQuery, vars, &data); err != nil {
		c.log.Error().Err(err).Str("owner", owner).Str("repo", repo).Str("expression", expression).
			Msg("github: failed to resolve a commitish")
		return "", fmt.Errorf("github: resolving %q for %s/%s: %w", expression, owner, repo, err)
	}
	if data.Repository == nil || data.Repository.Object == nil || data.Repository.Object.Typename != "Commit" {
		return "", fmt.Errorf("github: %q does not resolve to a commit in %s/%s", expression, owner, repo)
	}
	c.log.Debug().Str("owner", owner).Str("repo", repo).Str("expression", expression).Str("oid", data.Repository.Object.Oid).
		Msg("github: resolved a commitish")
	return data.Repository.Object.Oid, nil
}
