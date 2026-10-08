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

	"github.com/google/go-github/v76/github"
)

// pullRequestFilesPerPage matches v7: only the per-page size changes, the
// REST client still follows every pagination link.
const pullRequestFilesPerPage = 50

// ChangedFiles lists every file changed by a pull request, in the order
// the REST API returns them.
func (c *Client) ChangedFiles(ctx context.Context, owner, repo string, number int) ([]string, error) {
	c.log.Debug().Str("owner", owner).Str("repo", repo).Int("number", number).Msg("github: listing changed files")
	var out []string
	opts := &github.ListOptions{PerPage: pullRequestFilesPerPage}
	for {
		files, resp, err := c.rest.PullRequests.ListFiles(ctx, owner, repo, number, opts)
		if err != nil {
			c.log.Error().Err(err).Str("owner", owner).Str("repo", repo).Int("number", number).
				Msg("github: failed to list changed files")
			return nil, fmt.Errorf("github: listing changed files for %s/%s#%d: %w", owner, repo, number, err)
		}
		for _, f := range files {
			out = append(out, f.GetFilename())
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	c.log.Debug().Str("owner", owner).Str("repo", repo).Int("number", number).Int("files", len(out)).
		Msg("github: listed changed files")
	return out, nil
}
