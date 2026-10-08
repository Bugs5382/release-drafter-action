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

	"github.com/Bugs5382/release-drafter-action/internal/github"
	"github.com/Bugs5382/release-drafter-action/internal/logging"
	"github.com/Bugs5382/release-drafter-action/internal/model"
	"github.com/Bugs5382/release-drafter-action/internal/render"
)

// UpsertClient is what Upsert needs from a *github.Client.
type UpsertClient interface {
	CreateRelease(ctx context.Context, owner, repo string, in github.ReleaseInput) (model.Release, error)
	UpdateRelease(ctx context.Context, owner, repo string, id int64, in github.ReleaseInput) (model.Release, error)
}

// UpsertInput is what Upsert needs to create or update a draft (or
// published) release.
type UpsertInput struct {
	Owner, Repo string
	// DraftRelease is the existing release to update; nil creates a new
	// one, the way FindPreviousReleases' draft return value feeds this.
	DraftRelease *model.Release
	Payload      render.Payload
	// TargetCommitish overrides the release's target on an update; "" keeps
	// the existing value. CreateRelease always sends it.
	TargetCommitish string
	// Empty is true when there is nothing to draft (no pull requests in
	// range): Upsert makes no API call at all, rather than creating or
	// updating a release that says "No changes". This is this action's own
	// addition; v7 has no equivalent.
	Empty bool
	// DryRun makes no API call either, logging what would happen instead.
	DryRun bool
}

func makeLatest(prerelease, latest bool) string {
	if prerelease {
		return "false"
	}
	if latest {
		return "true"
	}
	return "false"
}

// Upsert creates a new release or updates an existing draft, matching v7's
// upsertRelease/createRelease/updateRelease. On an update, an empty name or
// tag in the payload leaves the release's existing name or tag in place
// rather than clearing it.
func Upsert(ctx context.Context, client UpsertClient, in UpsertInput) (*model.Release, error) {
	nameWithOwner := in.Owner + "/" + in.Repo
	if in.Empty {
		logging.L().Info().Str("repo", nameWithOwner).Msg("history: no pull requests in range, skipping the draft")
		return nil, nil
	}
	if in.DryRun {
		logging.L().Info().Str("repo", nameWithOwner).Str("name", in.Payload.Name).Str("tag", in.Payload.Tag).
			Bool("would_create", in.DraftRelease == nil).Msg("history: dry run, not writing the release")
		return nil, nil
	}

	latest := makeLatest(in.Payload.Prerelease, in.Payload.MakeLatest)
	if in.DraftRelease == nil {
		out, err := client.CreateRelease(ctx, in.Owner, in.Repo, github.ReleaseInput{
			Name:            &in.Payload.Name,
			TagName:         &in.Payload.Tag,
			TargetCommitish: in.TargetCommitish,
			Body:            in.Payload.Body,
			Draft:           in.Payload.Draft,
			Prerelease:      in.Payload.Prerelease,
			MakeLatest:      latest,
		})
		if err != nil {
			return nil, fmt.Errorf("history: creating the release for %s: %w", nameWithOwner, err)
		}
		return &out, nil
	}

	var name, tag *string
	if in.Payload.Name != "" {
		name = &in.Payload.Name
	}
	if in.Payload.Tag != "" {
		tag = &in.Payload.Tag
	}
	out, err := client.UpdateRelease(ctx, in.Owner, in.Repo, in.DraftRelease.ID, github.ReleaseInput{
		Name:            name,
		TagName:         tag,
		TargetCommitish: in.TargetCommitish,
		Body:            in.Payload.Body,
		Draft:           in.Payload.Draft,
		Prerelease:      in.Payload.Prerelease,
		MakeLatest:      latest,
	})
	if err != nil {
		return nil, fmt.Errorf("history: updating the release for %s: %w", nameWithOwner, err)
	}
	return &out, nil
}
