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
	"sort"
	"strings"

	"github.com/Bugs5382/release-drafter-action/internal/config"
	"github.com/Bugs5382/release-drafter-action/internal/logging"
	"github.com/Bugs5382/release-drafter-action/internal/model"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

const botSuffix = "[bot]"

type contributor struct {
	login  string
	name   string
	botURL string
	isName bool
}

func (c contributor) sortName() string {
	if c.isName {
		return c.name
	}
	return c.login
}

func normalizeLogin(login string, isBot bool) string {
	if isBot && !strings.HasSuffix(login, botSuffix) {
		return login + botSuffix
	}
	return login
}

func mention(c contributor, serverURL string) string {
	if c.isName {
		return c.name
	}
	if strings.HasSuffix(c.login, botSuffix) {
		url := c.botURL
		if url == "" {
			url = strings.TrimSuffix(serverURL, "/") + "/apps/" + strings.TrimSuffix(c.login, botSuffix)
		}
		return "[@" + c.login + "](" + url + ")"
	}
	return "@" + c.login
}

// AuthorsOptions configures AuthorsSentence.
type AuthorsOptions struct {
	Commits             []model.Commit
	PullRequests        []model.PullRequest
	ExcludeContributors []string
	NoAuthorsTemplate   string
	// AuthorTemplate, when set, renders each author and joins them with
	// Separator and FinalSeparator; otherwise mentions are joined as
	// "a, b and c".
	AuthorTemplate *string
	Separator      string
	FinalSeparator *string
	ServerURL      string
}

// newLocaleCollator returns a fresh collator for one call's sort. A
// collate.Collator's CompareString mutates internal iterator buffers, so it
// is not safe to share across concurrent callers (confirmed with
// -race): AuthorsSentence and NewContributors each build their own
// instance rather than reusing a package-level one.
func newLocaleCollator() *collate.Collator {
	return collate.New(language.Und)
}

// AuthorsSentence ports v7's generateAuthorsSentence: commit authors (with
// co-authors) of commits that belong to the pull requests, then the pull
// request authors, sorted pull request authors first, humans before bots,
// then by name the way JavaScript's localeCompare orders them.
func AuthorsSentence(o AuthorsOptions) string {
	included := map[string]bool{}
	mergeOIDs := map[string]bool{}
	for _, pr := range o.PullRequests {
		included[pr.Key()] = true
		if pr.MergeCommitOID != "" {
			mergeOIDs[pr.MergeCommitOID] = true
		}
	}
	var order []string
	byKey := map[string]contributor{}
	put := func(key string, c contributor) {
		if _, ok := byKey[key]; !ok {
			order = append(order, key)
		}
		byKey[key] = c
	}
	for _, commit := range o.Commits {
		belongs := mergeOIDs[commit.OID]
		for _, pr := range commit.PullRequests {
			if included[pr.Key()] {
				belongs = true
				break
			}
		}
		if !belongs {
			continue
		}
		authors := commit.Authors
		if authors == nil && commit.Author != nil {
			authors = []model.CommitAuthor{*commit.Author}
		}
		for _, a := range authors {
			switch {
			case a.Login != "":
				login := normalizeLogin(a.Login, false)
				put("login:"+login, contributor{login: login})
			case a.Name != "":
				put("name:"+a.Name, contributor{name: a.Name, isName: true})
			}
		}
	}
	prAuthors := map[string]bool{}
	for _, pr := range o.PullRequests {
		if pr.Author == nil {
			continue
		}
		isBot := pr.Author.Typename == "Bot"
		login := normalizeLogin(pr.Author.Login, isBot)
		prAuthors[login] = true
		c := contributor{login: login}
		if isBot {
			c.botURL = pr.Author.URL
		}
		put("login:"+login, c)
	}
	var list []contributor
	for _, k := range order {
		c := byKey[k]
		if !c.isName && slices.ContainsFunc(o.ExcludeContributors, func(ex string) bool {
			return ex == c.login || ex+botSuffix == c.login
		}) {
			continue
		}
		list = append(list, c)
	}
	isBot := func(c contributor) bool {
		return !c.isName && (c.botURL != "" || strings.HasSuffix(c.login, botSuffix))
	}
	collator := newLocaleCollator()
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		ap, bp := !a.isName && prAuthors[a.login], !b.isName && prAuthors[b.login]
		if ap != bp {
			return ap
		}
		if isBot(a) != isBot(b) {
			return !isBot(a)
		}
		return collator.CompareString(a.sortName(), b.sortName()) < 0
	})
	usedNoAuthorsTemplate := len(list) == 0
	logging.L().Trace().Int("contributors_found", len(order)).Int("excluded", len(order)-len(list)).
		Int("final_count", len(list)).Bool("used_no_authors_template", usedNoAuthorsTemplate).
		Msg("render: built the authors sentence")
	if usedNoAuthorsTemplate {
		return o.NoAuthorsTemplate
	}
	if o.AuthorTemplate != nil {
		authors := make([]string, len(list))
		for i, c := range list {
			authors[i] = Render(*o.AuthorTemplate, Vars{"$AUTHOR": c.sortName(), "$AUTHOR_MENTION": mention(c, o.ServerURL)})
		}
		if o.FinalSeparator != nil && len(authors) > 1 {
			return strings.Join(authors[:len(authors)-1], o.Separator) + *o.FinalSeparator + authors[len(authors)-1]
		}
		return strings.Join(authors, o.Separator)
	}
	mentions := make([]string, len(list))
	for i, c := range list {
		mentions[i] = mention(c, o.ServerURL)
	}
	if len(mentions) > 1 {
		return strings.Join(mentions[:len(mentions)-1], ", ") + " and " + mentions[len(mentions)-1]
	}
	return mentions[0]
}

// Contributors renders $CONTRIBUTORS over the pull requests that pass the
// pre-include and pre-exclude categories.
func Contributors(commits []model.Commit, prs []model.PullRequest, p *config.Parsed, serverURL string) string {
	filtered := FilterPre(prs, p.Categories)
	result := AuthorsSentence(AuthorsOptions{
		Commits:             commits,
		PullRequests:        filtered,
		ExcludeContributors: p.Config.ExcludeContributors,
		NoAuthorsTemplate:   p.Config.NoContributorsTemplate,
		ServerURL:           serverURL,
	})
	logging.L().Debug().Int("pull_requests", len(filtered)).
		Bool("used_no_contributors_template", result == p.Config.NoContributorsTemplate).
		Msg("render: rendered the contributors sentence")
	return result
}

// NewContributors renders $NEW_CONTRIBUTORS. newLogins holds the authors
// with no merged pull request before this range.
func NewContributors(prs []model.PullRequest, newLogins map[string]bool, p *config.Parsed) string {
	includedKeys := map[string]bool{}
	for _, pr := range FilterPre(prs, p.Categories) {
		includedKeys[pr.Key()] = true
	}
	first := map[string]model.PullRequest{}
	var logins []string
	for _, pr := range prs {
		if pr.Author == nil || !newLogins[pr.Author.Login] || slices.Contains(p.Config.ExcludeContributors, pr.Author.Login) {
			continue
		}
		prev, ok := first[pr.Author.Login]
		if !ok {
			logins = append(logins, pr.Author.Login)
		}
		if !ok || deref(pr.MergedAt) < deref(prev.MergedAt) {
			first[pr.Author.Login] = pr
		}
	}
	type entry struct {
		login string
		pr    model.PullRequest
	}
	var entries []entry
	for _, l := range logins {
		if includedKeys[first[l].Key()] {
			entries = append(entries, entry{l, first[l]})
		}
	}
	collator := newLocaleCollator()
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := deref(entries[i].pr.MergedAt), deref(entries[j].pr.MergedAt)
		if c := collator.CompareString(a, b); c != 0 {
			return c < 0
		}
		return entries[i].pr.Number < entries[j].pr.Number
	})
	usedNoNewContributorTemplate := len(entries) == 0
	logging.L().Debug().Int("new_contributor_entries", len(entries)).
		Bool("used_no_new_contributor_template", usedNoNewContributorTemplate).
		Msg("render: rendered the new contributors list")
	if usedNoNewContributorTemplate {
		return p.Config.NoNewContributorTemplate
	}
	lines := make([]string, len(entries))
	for i, e := range entries {
		vars := Vars{"$AUTHOR": e.login, "$AUTHOR_MENTION": "@" + e.login, "$NUMBER": itoa(e.pr.Number)}
		if e.pr.Author != nil {
			vars["$AUTHOR_URL"] = e.pr.Author.URL
		}
		if e.pr.URL != nil {
			vars["$URL"] = *e.pr.URL
		}
		lines[i] = Render(p.Config.NewContributorTemplate, vars)
	}
	return strings.Join(lines, "\n")
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
