package action

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

	"github.com/Bugs5382/release-drafter-action/internal/config"
	"github.com/Bugs5382/release-drafter-action/internal/github"
	"github.com/Bugs5382/release-drafter-action/internal/history"
	"github.com/Bugs5382/release-drafter-action/internal/logging"
	"github.com/Bugs5382/release-drafter-action/internal/render"
)

// runDraft loads the config, finds the commit range and its pull requests,
// renders the body and upserts the draft (or published) release, the way
// v7's index.js drives its own run for push, workflow_dispatch and release
// events.
func runDraft(ctx context.Context, client *github.Client, o Options, in parsedInputs, owner, repo string, ann logging.Annotator) int {
	log := logging.L()

	loaded, err := config.Load(ctx, config.LoadOptions{
		Inline:      in.Config,
		Extends:     in.Extends,
		ConfigName:  in.ConfigName,
		Workspace:   o.Env.Workspace,
		ExcludeBots: in.ExcludeBots,
		Fetch: config.ExtendsOptions{
			APIURL:   o.Env.APIURL,
			Token:    o.Env.Token,
			Asset:    in.ExtendsAsset,
			CacheDir: o.ExtendsCacheDir,
			Log:      log,
		},
		Log: log,
	})
	if err != nil {
		return fail(ann, err)
	}
	for _, w := range loaded.Warnings {
		ann.Warning(w)
	}

	parsed, err := config.Merge(loaded.Config, in.Inputs, o.Env.Ref)
	if err != nil {
		return fail(ann, err)
	}
	for _, w := range parsed.Warnings {
		ann.Warning(w)
	}
	for _, n := range parsed.Notes {
		log.Info().Msg(n)
	}

	releases, err := client.ListReleases(ctx, owner, repo)
	if err != nil {
		return fail(ann, err)
	}
	last, draftRelease := history.FindPreviousReleases(releases, history.ReleaseFilter{
		Commitish:          parsed.Commitish,
		FilterByCommitish:  parsed.Config.FilterByCommitish,
		TagPrefix:          parsed.Config.TagPrefix,
		Prerelease:         parsed.Prerelease,
		IncludePreReleases: parsed.IncludePreReleases,
		Range:              parsed.Range,
	})

	withBody, withURL, withBaseRef, withHeadRef := render.NeedsPullRequestFields(parsed.Config.ChangeTemplate)
	fields := github.CommitFieldOptions{
		WithPullRequestBody: withBody,
		WithPullRequestURL:  withURL,
		WithBaseRefName:     withBaseRef,
		WithHeadRefName:     withHeadRef,
		PullRequestLimit:    parsed.Config.PullRequestLimit,
		HistoryLimit:        parsed.Config.HistoryLimit,
	}

	commits, err := history.CollectCommits(ctx, client, history.RangeInput{
		Owner: owner, Repo: repo, Commitish: parsed.Commitish, LastRelease: last, Since: in.Since, Fields: fields,
	})
	if err != nil {
		return fail(ann, err)
	}

	prs, newContributors, err := history.CollectPullRequests(ctx, client, history.CollectInput{
		Owner: owner, Repo: repo, Commits: commits, Commitish: parsed.Commitish, Fields: fields,
		NeedsChangedFiles:    render.NeedsChangedFiles(parsed.Categories),
		NeedsNewContributors: render.NeedsNewContributors(parsed.Config),
	})
	if err != nil {
		return fail(ann, err)
	}

	// On a first release there is no last release to anchor $PREVIOUS_TAG,
	// so it falls back to the oldest commit in range: CommitsSince returns
	// history newest first, so that commit is the last element.
	previousTag := ""
	if last == nil && len(commits) > 0 {
		previousTag = commits[len(commits)-1].OID
	}

	payload, err := render.Build(render.BuildInput{
		Parsed: parsed, Inputs: in.Inputs, LastRelease: last, PreviousTag: previousTag,
		Commits: commits, PullRequests: prs, NewContributors: newContributors,
		Owner: owner, Repo: repo, ServerURL: o.Env.ServerURL, FirstVersion: in.FirstVersion,
	})
	if err != nil {
		return fail(ann, err)
	}

	result, err := history.Upsert(ctx, client, history.UpsertInput{
		Owner: owner, Repo: repo, DraftRelease: draftRelease, Payload: payload,
		TargetCommitish: parsed.Commitish, Empty: len(prs) == 0, DryRun: in.Inputs.DryRun,
	})
	if err != nil {
		return fail(ann, err)
	}

	if err := writeOutputs(o.Env.OutputPath, draftOutputs(payload, result)); err != nil {
		return fail(ann, err)
	}
	if in.Inputs.DryRun {
		_, _ = fmt.Fprintln(o.Stdout, payload.Body)
	}
	log.Info().Str("owner", owner).Str("repo", repo).Str("tag", payload.Tag).Str("resolved_version", payload.ResolvedVersion).
		Bool("dry_run", in.Inputs.DryRun).Bool("skipped_empty", len(prs) == 0).Msg("action: finished the draft run")
	return 0
}
