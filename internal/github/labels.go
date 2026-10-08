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

// AddLabels adds labels to a pull request (issues and pull requests share
// the same labels endpoint). It only ever adds: the REST call never removes
// a label already on the pull request.
func (c *Client) AddLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	c.log.Info().Str("owner", owner).Str("repo", repo).Int("number", number).Strs("labels", labels).
		Msg("github: adding labels")
	if len(labels) == 0 {
		c.log.Debug().Int("number", number).Msg("github: no labels to add, skipping the call")
		return nil
	}
	_, _, err := c.rest.Issues.AddLabelsToIssue(ctx, owner, repo, number, labels)
	if err != nil {
		c.log.Error().Err(err).Str("owner", owner).Str("repo", repo).Int("number", number).Msg("github: failed to add labels")
		return fmt.Errorf("github: adding labels to %s/%s#%d: %w", owner, repo, number, err)
	}
	c.log.Info().Str("owner", owner).Str("repo", repo).Int("number", number).Msg("github: added labels")
	return nil
}
