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
	"strings"

	"github.com/Bugs5382/release-drafter-action/internal/autolabel"
	"github.com/Bugs5382/release-drafter-action/internal/config"
	"github.com/Bugs5382/release-drafter-action/internal/github"
	"github.com/Bugs5382/release-drafter-action/internal/logging"
)

// runAutolabel loads the config, matches the pull request from the event
// payload against the autolabeler rules and adds whatever labels matched,
// the way v7's autolabeler runs on pull_request and pull_request_target.
func runAutolabel(ctx context.Context, client *github.Client, o Options, in parsedInputs, owner, repo string, ann logging.Annotator) int {
	log := logging.L()

	event, err := loadPullRequestEvent(o.Env.EventPath)
	if err != nil {
		return fail(ann, err)
	}

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

	rules, err := config.CompileAutolabeler(loaded.Config)
	if err != nil {
		return fail(ann, err)
	}

	pr := autolabel.PullRequest{Title: event.PullRequest.Title, BranchRef: event.PullRequest.Head.Ref}
	if event.PullRequest.Body != nil {
		pr.Body = *event.PullRequest.Body
	}
	if needsChangedFilesForLabels(rules) {
		files, err := client.ChangedFiles(ctx, owner, repo, event.Number)
		if err != nil {
			return fail(ann, err)
		}
		pr.ChangedFiles = files
	}

	labels, err := autolabel.Labels(rules, pr)
	if err != nil {
		return fail(ann, err)
	}

	if len(labels) > 0 && !in.Inputs.DryRun {
		if err := client.AddLabels(ctx, owner, repo, event.Number, labels); err != nil {
			return fail(ann, err)
		}
	}
	if in.Inputs.DryRun {
		_, _ = fmt.Fprintln(o.Stdout, strings.Join(labels, ","))
	}
	log.Info().Str("owner", owner).Str("repo", repo).Int("number", event.Number).Strs("labels", labels).
		Bool("dry_run", in.Inputs.DryRun).Msg("action: finished the autolabel run")
	return 0
}

func needsChangedFilesForLabels(rules []config.CompiledAutolabel) bool {
	for _, r := range rules {
		if r.Files != nil {
			return true
		}
	}
	return false
}
