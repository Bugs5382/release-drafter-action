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

	"github.com/Bugs5382/release-drafter-action/internal/model"
)

// releaseCountLimit mirrors v7: GitHub's REST API returns a 500 past 1000
// releases listed, so paging stops there.
const releaseCountLimit = 1000

func toModelRelease(r *github.RepositoryRelease) model.Release {
	out := model.Release{ID: r.GetID()}
	if r.TagName != nil {
		out.TagName = *r.TagName
	}
	if r.Name != nil {
		out.Name = *r.Name
	}
	out.Draft = r.GetDraft()
	out.Prerelease = r.GetPrerelease()
	if r.TargetCommitish != nil {
		out.TargetCommitish = *r.TargetCommitish
	}
	if r.CreatedAt != nil {
		out.CreatedAt = r.CreatedAt.Format("2006-01-02T15:04:05Z")
	}
	if r.HTMLURL != nil {
		out.HTMLURL = *r.HTMLURL
	}
	if r.UploadURL != nil {
		out.UploadURL = *r.UploadURL
	}
	return out
}

// ListReleases lists every release of owner/repo, newest first the way the
// REST API orders them, up to releaseCountLimit.
func (c *Client) ListReleases(ctx context.Context, owner, repo string) ([]model.Release, error) {
	c.log.Debug().Str("owner", owner).Str("repo", repo).Msg("github: listing releases")
	var out []model.Release
	opts := &github.ListOptions{PerPage: 100}
	for {
		releases, resp, err := c.rest.Repositories.ListReleases(ctx, owner, repo, opts)
		if err != nil {
			c.log.Error().Err(err).Str("owner", owner).Str("repo", repo).Msg("github: failed to list releases")
			return nil, fmt.Errorf("github: listing releases for %s/%s: %w", owner, repo, err)
		}
		for _, r := range releases {
			out = append(out, toModelRelease(r))
		}
		if len(out) >= releaseCountLimit || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	c.log.Info().Str("owner", owner).Str("repo", repo).Int("releases", len(out)).Msg("github: listed releases")
	return out, nil
}

// ReleaseInput is the subset of a release's fields that CreateRelease and
// UpdateRelease accept. Name and TagName are pointers so "leave unset" (nil,
// fall back to what already exists on an update) is distinct from "set to
// empty".
type ReleaseInput struct {
	Name            *string
	TagName         *string
	TargetCommitish string
	Body            string
	Draft           bool
	Prerelease      bool
	// MakeLatest is "true", "false" or "legacy", following the REST API.
	MakeLatest string
}

func (in ReleaseInput) toGithub() *github.RepositoryRelease {
	r := &github.RepositoryRelease{
		Body:       &in.Body,
		Draft:      &in.Draft,
		Prerelease: &in.Prerelease,
	}
	if in.Name != nil {
		r.Name = in.Name
	}
	if in.TagName != nil {
		r.TagName = in.TagName
	}
	if in.TargetCommitish != "" {
		r.TargetCommitish = &in.TargetCommitish
	}
	if in.MakeLatest != "" {
		r.MakeLatest = &in.MakeLatest
	}
	return r
}

// CreateRelease creates a new release.
func (c *Client) CreateRelease(ctx context.Context, owner, repo string, in ReleaseInput) (model.Release, error) {
	c.log.Info().Str("owner", owner).Str("repo", repo).Bool("draft", in.Draft).Msg("github: creating release")
	r, _, err := c.rest.Repositories.CreateRelease(ctx, owner, repo, in.toGithub())
	if err != nil {
		c.log.Error().Err(err).Str("owner", owner).Str("repo", repo).Msg("github: failed to create release")
		return model.Release{}, fmt.Errorf("github: creating release for %s/%s: %w", owner, repo, err)
	}
	out := toModelRelease(r)
	c.log.Info().Str("owner", owner).Str("repo", repo).Int64("id", out.ID).Str("tag_name", out.TagName).Msg("github: created release")
	return out, nil
}

// UpdateRelease updates the release identified by id.
func (c *Client) UpdateRelease(ctx context.Context, owner, repo string, id int64, in ReleaseInput) (model.Release, error) {
	c.log.Info().Str("owner", owner).Str("repo", repo).Int64("id", id).Bool("draft", in.Draft).Msg("github: updating release")
	r, _, err := c.rest.Repositories.EditRelease(ctx, owner, repo, id, in.toGithub())
	if err != nil {
		c.log.Error().Err(err).Str("owner", owner).Str("repo", repo).Int64("id", id).Msg("github: failed to update release")
		return model.Release{}, fmt.Errorf("github: updating release %d for %s/%s: %w", id, owner, repo, err)
	}
	out := toModelRelease(r)
	c.log.Info().Str("owner", owner).Str("repo", repo).Int64("id", out.ID).Str("tag_name", out.TagName).Msg("github: updated release")
	return out, nil
}
