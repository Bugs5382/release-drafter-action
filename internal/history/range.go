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
	"regexp"
	"time"

	"github.com/Bugs5382/release-drafter-action/internal/github"
	"github.com/Bugs5382/release-drafter-action/internal/logging"
	"github.com/Bugs5382/release-drafter-action/internal/model"
)

// RangeClient is what CollectCommits needs from a *github.Client: just the
// two ways to walk commits, so tests can fake either without a server.
type RangeClient interface {
	CommitsInRange(ctx context.Context, owner, repo, baseRef, headRef string, fields github.CommitFieldOptions) ([]model.Commit, error)
	CommitsSince(ctx context.Context, owner, repo, target string, since *string, fields github.CommitFieldOptions) ([]model.Commit, error)
}

// sha40 matches a full, bare commit SHA, the one case ParseSince treats as
// a direct commit expression rather than a tag.
var sha40 = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// gitTimestampLayouts are the date forms the since input accepts, beyond a
// tag or a SHA: a bare date, or a full RFC3339 timestamp.
var gitTimestampLayouts = []string{time.RFC3339, "2006-01-02"}

// ParseSince classifies the since input: a date (returned as a GitTimestamp
// string) or a commit expression (a tag or SHA, returned as-is). Empty
// stays empty, meaning "the whole history, no lower bound".
func ParseSince(since string) (date string, expression string) {
	if since == "" {
		return "", ""
	}
	if sha40.MatchString(since) {
		return "", since
	}
	for _, layout := range gitTimestampLayouts {
		if t, err := time.Parse(layout, since); err == nil {
			return t.UTC().Format(time.RFC3339), ""
		}
	}
	// Anything else is treated as a tag; CollectCommits resolves it the
	// same way it resolves a bare SHA, as a commit expression.
	return "", since
}

// RangeInput is what CollectCommits needs to find the commit range.
type RangeInput struct {
	Owner, Repo string
	// Commitish is the release target, the head side of the range.
	Commitish string
	// LastRelease is nil on a first release.
	LastRelease *model.Release
	// Since is the first-release start point (a tag, SHA or date); ignored
	// when LastRelease is set.
	Since  string
	Fields github.CommitFieldOptions
}

// CollectCommits finds the commits in the release range: between the last
// release's tag and the commitish when there is a last release, or the
// first-release walk (the since input, or the whole history) when there is
// not.
func CollectCommits(ctx context.Context, client RangeClient, in RangeInput) ([]model.Commit, error) {
	if in.LastRelease != nil {
		baseRef := "refs/tags/" + in.LastRelease.TagName
		logging.L().Info().Str("owner", in.Owner).Str("repo", in.Repo).Str("base_ref", baseRef).Str("head_ref", in.Commitish).
			Msg("history: finding commits since the last release")
		return client.CommitsInRange(ctx, in.Owner, in.Repo, baseRef, in.Commitish, in.Fields)
	}

	date, expression := ParseSince(in.Since)
	switch {
	case expression != "":
		logging.L().Info().Str("owner", in.Owner).Str("repo", in.Repo).Str("since", expression).Str("head_ref", in.Commitish).
			Msg("history: first release, comparing against the since commitish")
		return client.CommitsInRange(ctx, in.Owner, in.Repo, expression, in.Commitish, in.Fields)
	case date != "":
		logging.L().Info().Str("owner", in.Owner).Str("repo", in.Repo).Str("since", date).Str("target", in.Commitish).
			Msg("history: first release, walking history since a date")
		return client.CommitsSince(ctx, in.Owner, in.Repo, in.Commitish, &date, in.Fields)
	default:
		logging.L().Info().Str("owner", in.Owner).Str("repo", in.Repo).Str("target", in.Commitish).
			Msg("history: first release, walking the whole history")
		return client.CommitsSince(ctx, in.Owner, in.Repo, in.Commitish, nil, in.Fields)
	}
}
