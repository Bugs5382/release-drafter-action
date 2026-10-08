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
	"slices"
	"strings"

	"github.com/Bugs5382/release-drafter-action/internal/config"
	"github.com/Bugs5382/release-drafter-action/internal/logging"
	"github.com/Bugs5382/release-drafter-action/internal/model"
)

func uniq(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// matchesValues is a pure hot predicate (called once per condition per pull
// request), so it is deliberately log-free.
func matchesValues(actualValues, expectedValues []string, mode string) bool {
	actual, expected := uniq(actualValues), uniq(expectedValues)
	if len(expected) == 0 {
		return true
	}
	switch mode {
	case "all":
		for _, e := range expected {
			if !slices.Contains(actual, e) {
				return false
			}
		}
		return true
	case "only":
		if len(actual) == 0 {
			return false
		}
		for _, a := range actual {
			if !slices.Contains(expected, a) {
				return false
			}
		}
		return true
	case "exactly":
		if len(actual) != len(expected) {
			return false
		}
		for _, a := range actual {
			if !slices.Contains(expected, a) {
				return false
			}
		}
		return true
	}
	for _, e := range expected {
		if slices.Contains(actual, e) {
			return true
		}
	}
	return false
}

// matchesPaths is a pure hot predicate; see matchesValues.
func matchesPaths(c config.ParsedCondition, pr model.PullRequest) bool {
	if len(c.Paths) == 0 {
		return true
	}
	files := uniq(pr.ChangedFiles)
	if len(files) == 0 {
		return false
	}
	matchesAll := true
	for _, m := range c.Matchers {
		if !m.MatchAny(files) {
			matchesAll = false
			break
		}
	}
	matchesOnly := true
	for _, f := range files {
		hit := false
		for _, m := range c.Matchers {
			if m.Match(f) {
				hit = true
				break
			}
		}
		if !hit {
			matchesOnly = false
			break
		}
	}
	switch c.PathsMode {
	case "all":
		return matchesAll
	case "only":
		return matchesOnly
	case "exactly":
		return matchesAll && matchesOnly
	}
	for _, f := range files {
		for _, m := range c.Matchers {
			if m.Match(f) {
				return true
			}
		}
	}
	return false
}

// matchesConventional is a pure hot predicate; see matchesValues.
func matchesConventional(c config.ParsedCondition, pr model.PullRequest) bool {
	if c.Conventional == nil {
		return true
	}
	ct, ok := ParseConventionalTitle(pr.Title)
	if !ok {
		return false
	}
	conv := c.Conventional
	return (len(conv.Types) == 0 || slices.Contains(conv.Types, ct.Type)) &&
		(len(conv.Scopes) == 0 || (ct.HasScope && slices.Contains(conv.Scopes, ct.Scope))) &&
		(conv.Breaking == nil || *conv.Breaking == ct.Breaking)
}

// MatchesCondition reports whether pr satisfies every predicate of c. Pure
// hot predicate; see matchesValues.
func MatchesCondition(c config.ParsedCondition, pr model.PullRequest) bool {
	return matchesValues(pr.Labels, c.Labels, c.LabelsMode) && matchesPaths(c, pr) && matchesConventional(c, pr)
}

// MatchesCategory reports whether pr belongs to cat. A category without
// conditions matches everything. Pure hot predicate; see matchesValues.
func MatchesCategory(cat config.ParsedCategory, pr model.PullRequest) bool {
	if len(cat.When) == 0 {
		return true
	}
	for _, c := range cat.When {
		if MatchesCondition(c, pr) {
			return true
		}
	}
	return false
}

// FilterPre keeps the pull requests that pass the pre-include categories
// (any of them) and fail every pre-exclude category.
func FilterPre(prs []model.PullRequest, cats []config.ParsedCategory) []model.PullRequest {
	var include, exclude []config.ParsedCategory
	for _, c := range cats {
		switch c.Type {
		case "pre-include":
			include = append(include, c)
		case "pre-exclude":
			exclude = append(exclude, c)
		}
	}
	logging.L().Debug().Int("pull_requests", len(prs)).Int("pre_include_categories", len(include)).
		Int("pre_exclude_categories", len(exclude)).Msg("render: filtering pull requests against pre-include/pre-exclude categories")

	out := []model.PullRequest{}
	dropped := 0
	for _, pr := range prs {
		ok := len(include) == 0
		for _, c := range include {
			if MatchesCategory(c, pr) {
				ok = true
				break
			}
		}
		if !ok {
			dropped++
			logging.L().Trace().Int("pull_request", pr.Number).Msg("render: dropped pull request, no pre-include category matched")
			continue
		}
		for i, c := range exclude {
			if MatchesCategory(c, pr) {
				ok = false
				logging.L().Trace().Int("pull_request", pr.Number).Int("pre_exclude_category", i).Str("pre_exclude_title", c.Title).
					Msg("render: dropped pull request, a pre-exclude category matched")
				break
			}
		}
		if ok {
			out = append(out, pr)
		} else {
			dropped++
		}
	}
	logging.L().Debug().Int("kept", len(out)).Int("dropped", dropped).Msg("render: finished filtering pull requests")
	return out
}

// NeedsChangedFiles reports whether any condition uses paths, in which case
// the changed files of every pull request must be loaded.
func NeedsChangedFiles(cats []config.ParsedCategory) bool {
	needs := false
	for _, c := range cats {
		for _, w := range c.When {
			if len(w.Paths) > 0 {
				needs = true
				break
			}
		}
		if needs {
			break
		}
	}
	logging.L().Debug().Bool("needs_changed_files", needs).Msg("render: checked whether categories need changed files")
	return needs
}

// OfType returns the categories of one type, in config order. Pure hot
// predicate; see matchesValues.
func OfType(cats []config.ParsedCategory, typ string) []config.ParsedCategory {
	out := []config.ParsedCategory{}
	for _, c := range cats {
		if c.Type == typ {
			out = append(out, c)
		}
	}
	return out
}

// FilterBots drops pull requests opened by a bot account, the way
// exclude-bots trims the built-in default's draft before anything else in
// Build runs: sorting, categorizing, the changelog, contributors and the
// version increment all see the filtered list. A pull request with no
// author (a deleted account, rendered as "ghost") is never a bot.
func FilterBots(prs []model.PullRequest) []model.PullRequest {
	out := make([]model.PullRequest, 0, len(prs))
	dropped := 0
	for _, pr := range prs {
		if pr.Author != nil && config.IsBot(pr.Author.Typename, pr.Author.Login) {
			dropped++
			continue
		}
		out = append(out, pr)
	}
	logging.L().Debug().Int("kept", len(out)).Int("dropped", dropped).Msg("render: filtered bot pull requests for exclude-bots")
	return out
}

// NeedsPullRequestFields reports which optional associated-pull-request
// GraphQL fields changeTemplate (config's change-template) actually
// renders, so the caller only asks GitHub for what it will use, the way v7
// sizes its own query from the config.
func NeedsPullRequestFields(changeTemplate string) (body, url, baseRefName, headRefName bool) {
	body = strings.Contains(changeTemplate, "$BODY")
	url = strings.Contains(changeTemplate, "$URL")
	baseRefName = strings.Contains(changeTemplate, "$BASE_REF_NAME")
	headRefName = strings.Contains(changeTemplate, "$HEAD_REF_NAME")
	logging.L().Debug().Bool("body", body).Bool("url", url).Bool("base_ref_name", baseRefName).Bool("head_ref_name", headRefName).
		Msg("render: checked which pull request fields change-template needs")
	return body, url, baseRefName, headRefName
}

// NeedsNewContributors reports whether the rendered body ever uses
// $NEW_CONTRIBUTORS, the only place it can appear (header, template or
// footer, the way Build glues them together before rendering). When it
// does not, the caller can skip the new-contributor lookup entirely.
func NeedsNewContributors(cfg config.Config) bool {
	needs := strings.Contains(cfg.Header+cfg.Template+cfg.Footer, "$NEW_CONTRIBUTORS")
	logging.L().Debug().Bool("needs_new_contributors", needs).Msg("render: checked whether the body needs new contributors")
	return needs
}
