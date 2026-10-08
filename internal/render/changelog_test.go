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
	"testing"

	"github.com/Bugs5382/release-drafter-action/internal/config"
	"github.com/Bugs5382/release-drafter-action/internal/model"
)

func TestTrimJSMatchesStringTrim(t *testing.T) {
	in := string(byteOrderMark) + " \t\nx y" + string(lineSeparator) + string(rune(0x3000))
	if got := trimJS(in); got != "x y" {
		t.Errorf("trimJS = %q", got)
	}
	nel := string(rune(0x0085)) + "x"
	if got := trimJS(nel); got != nel {
		t.Errorf("U+0085 is not JavaScript whitespace, got %q", got)
	}
}

func mustParsed(t *testing.T, yamlText string) *config.Parsed {
	t.Helper()
	cfg, err := config.Parse([]byte(yamlText), "test")
	if err != nil {
		t.Fatal(err)
	}
	p, err := config.Merge(cfg, config.Inputs{Commitish: "main"}, "")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func labeled(n int, title string, labels ...string) model.PullRequest {
	return model.PullRequest{Number: n, Title: title, Labels: labels, BaseRepository: "o/r", Merged: true,
		Author: &model.Actor{Typename: "User", Login: "a", URL: "https://github.com/a"}}
}

// Expected layouts come from v7.7.0's generateChangeLog.
func TestChangelogLayoutMatchesV7(t *testing.T) {
	cats := "change-template: '* $TITLE'\ncategories:\n  - title: A\n    labels: [a]\n    collapse-after: 1\n  - title: Empty\n    labels: [zzz]\n  - title: B\n    labels: [b]\n"
	prs := []model.PullRequest{labeled(1, "u"), labeled(2, "a1", "a"), labeled(3, "a2", "a"), labeled(4, "b1", "b")}
	cases := []struct {
		name, config string
		prs          []model.PullRequest
		want         string
	}{
		{"layout", cats + "category-template: '### $TITLE'\n", prs,
			"* u\n\n### A\n\n<details>\n<summary>2 changes</summary>\n\n* a1\n* a2\n</details>\n\n### B\n\n* b1"},
		{"no category title", cats + "category-template: ''\n", prs,
			"* u\n\n<details>\n<summary>2 changes</summary>\n\n* a1\n* a2\n</details>\n\n* b1"},
		{"no changes", cats + "no-changes-template: Nothing yet\n", nil, "Nothing yet"},
		{"one collapsed change", "change-template: '* $TITLE'\ncategories:\n  - title: A\n    labels: [a]\n    collapse-after: 0\n", prs[1:2],
			"## A\n\n<details>\n<summary>1 change</summary>\n\n* a1\n</details>"},
	}
	for _, c := range cases {
		if got := Changelog(nil, c.prs, mustParsed(t, c.config), "https://github.com"); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// Expected sentences come from v7.7.0's generateAuthorsSentence.
func TestAuthorsSentenceMatchesV7(t *testing.T) {
	bob := labeled(1, "t")
	bob.Author = &model.Actor{Typename: "User", Login: "bob", URL: "https://github.com/bob"}
	bot := labeled(2, "t")
	bot.Author = &model.Actor{Typename: "Bot", Login: "dependabot", URL: "https://github.com/dependabot"}
	other := labeled(9, "t")
	commits := []model.Commit{
		{OID: "c1", Authors: []model.CommitAuthor{{Name: "zed", Login: "zed"}, {Name: "Anne Human"}}, PullRequests: []model.PullRequest{bob}},
		{OID: "c2", Authors: []model.CommitAuthor{{Name: "x", Login: "renovate[bot]"}, {Name: "alice", Login: "alice"}}, PullRequests: []model.PullRequest{bot}},
		{OID: "c3", Authors: []model.CommitAuthor{{Name: "eve", Login: "eve"}}, PullRequests: []model.PullRequest{other}},
	}
	prs := []model.PullRequest{bob, bot}
	server := "https://github.com"
	tmpl, final := "$AUTHOR", " + "
	cases := map[string]struct {
		opts AuthorsOptions
		want string
	}{
		"default": {AuthorsOptions{Commits: commits, PullRequests: prs, ServerURL: server},
			"@bob, [@dependabot[bot]](https://github.com/dependabot), @alice, Anne Human, @zed and [@renovate[bot]](https://github.com/apps/renovate)"},
		"excluded": {AuthorsOptions{Commits: commits, PullRequests: prs, ExcludeContributors: []string{"zed", "dependabot"}, ServerURL: server},
			"@bob, @alice, Anne Human and [@renovate[bot]](https://github.com/apps/renovate)"},
		"templated": {AuthorsOptions{Commits: commits, PullRequests: prs, AuthorTemplate: &tmpl, Separator: "; ", FinalSeparator: &final, ServerURL: server},
			"bob; dependabot[bot]; alice; Anne Human; zed + renovate[bot]"},
		"nobody": {AuthorsOptions{NoAuthorsTemplate: "nobody"}, "nobody"},
	}
	for name, c := range cases {
		if got := AuthorsSentence(c.opts); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", name, got, c.want)
		}
	}
}
