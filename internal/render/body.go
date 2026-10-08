package render

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
	"github.com/Bugs5382/release-drafter-action/internal/config"
	"github.com/Bugs5382/release-drafter-action/internal/logging"
	"github.com/Bugs5382/release-drafter-action/internal/model"
)

// BuildInput is everything Build needs. None of it touches the network.
type BuildInput struct {
	Parsed *config.Parsed
	Inputs config.Inputs
	// LastRelease is nil on a first release.
	LastRelease *model.Release
	// PreviousTag fills $PREVIOUS_TAG only on a first release (LastRelease
	// nil): the SHA of the first commit. When LastRelease is set, $PREVIOUS_TAG
	// comes from LastRelease.TagName instead, the way v7 fills it from
	// lastRelease.tag_name.
	PreviousTag     string
	Commits         []model.Commit
	PullRequests    []model.PullRequest
	NewContributors map[string]bool
	Owner           string
	Repo            string
	ServerURL       string
	// FirstVersion is the $RESOLVED_VERSION baseline on a first release
	// (LastRelease nil) with no version/tag/name override, instead of
	// incrementing from 0.0.0. Empty keeps the 0.0.0 fallback.
	FirstVersion string
}

// Payload is the rendered release.
type Payload struct {
	Name              string
	Tag               string
	Body              string
	Prerelease        bool
	MakeLatest        bool
	Draft             bool
	ResolvedVersion   string
	MajorVersion      string
	MinorVersion      string
	PatchVersion      string
	PrereleaseVersion string
}

// Build ports v7's buildReleasePayload without the network step (the
// target commitish is resolved by the draft package). The body is header,
// template and footer glued together with nothing in between; v7's
// "no previous release" banner is never added.
func Build(in BuildInput) (Payload, error) {
	p := in.Parsed
	c := p.Config
	logging.L().Debug().Int("pull_requests", len(in.PullRequests)).Int("commits", len(in.Commits)).
		Bool("first_release", in.LastRelease == nil).Int("replacers", len(p.Replacers)).
		Msg("render: building the release payload")
	prs := in.PullRequests
	if c.ExcludeBots {
		prs = FilterBots(prs)
	}
	sorted := SortPullRequests(prs, c.SortBy, c.SortDirection)
	previousTag := in.PreviousTag
	if in.LastRelease != nil {
		previousTag = in.LastRelease.TagName
	}
	body := c.Header + c.Template + c.Footer
	body = Render(body, Vars{
		"$PREVIOUS_TAG":     previousTag,
		"$CHANGES":          Changelog(in.Commits, sorted, p, in.ServerURL),
		"$CONTRIBUTORS":     Contributors(in.Commits, sorted, p, in.ServerURL),
		"$NEW_CONTRIBUTORS": NewContributors(sorted, in.NewContributors, p),
		"$OWNER":            in.Owner,
		"$REPOSITORY":       in.Repo,
	})
	body, err := ApplyReplacers(body, p.Replacers)
	if err != nil {
		logging.L().Error().Err(err).Msg("render: failed to apply the replacers")
		return Payload{}, err
	}
	increment := ResolveIncrement(prs, p)
	vars, err := GetVersionInfo(in.LastRelease, c, VersionInput{Version: in.Inputs.Version, Tag: in.Inputs.Tag, Name: in.Inputs.Name, FirstVersion: in.FirstVersion}, increment)
	if err != nil {
		logging.L().Error().Err(err).Msg("render: failed to resolve the version info")
		return Payload{}, err
	}
	body = Render(body, vars)
	out := Payload{
		Name:              RenderNameOrTag(in.Inputs.Name, c.NameTemplate, vars),
		Tag:               RenderNameOrTag(in.Inputs.Tag, c.TagTemplate, vars),
		Body:              body,
		Prerelease:        p.Prerelease,
		MakeLatest:        p.Latest,
		Draft:             !in.Inputs.Publish,
		ResolvedVersion:   vars["$RESOLVED_VERSION"],
		MajorVersion:      vars["$RESOLVED_VERSION_MAJOR"],
		MinorVersion:      vars["$RESOLVED_VERSION_MINOR"],
		PatchVersion:      vars["$RESOLVED_VERSION_PATCH"],
		PrereleaseVersion: vars["$RESOLVED_VERSION_PRERELEASE"],
	}
	logging.L().Debug().Str("name", out.Name).Str("tag", out.Tag).Str("resolved_version", out.ResolvedVersion).
		Int("body_length", len(out.Body)).Bool("prerelease", out.Prerelease).Bool("draft", out.Draft).
		Msg("render: built the release payload")
	return out, nil
}
