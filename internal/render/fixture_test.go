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
	"fmt"

	"github.com/Bugs5382/release-drafter-action/internal/model"
)

// fixture is a synthetic release range that exercises every rendering
// rule: escapes, bot and ghost authors, co-authors, labels that hit several
// categories, a skip-changelog item, a tie on merged_at and enough
// dependency updates to collapse.
type fixture struct {
	commits []model.Commit
	prs     []model.PullRequest
	files   map[int][]string
}

func newFixture() fixture {
	str := func(s string) *string { return &s }
	type spec struct {
		n        int
		title    string
		labels   []string
		author   string
		typename string
		merged   string
		files    []string
	}
	specs := []spec{
		{1, "feat(api): add `x_y` endpoint <b> & *stars*", []string{"enhancement"}, "alice", "User", "2026-01-03T10:00:00Z", []string{"src/api.go"}},
		{2, "fix: crash @mention #12", []string{"bug"}, "Bob", "User", "2026-01-02T10:00:00Z", []string{"src/crash.go", "README.md"}},
		{3, "chore(deps): bump lib", []string{"dependencies"}, "dependabot", "Bot", "2026-01-04T10:00:00Z", []string{"go.mod"}},
		{4, "docs: readme", []string{"documentation", "enhancement"}, "alice", "User", "2026-01-01T10:00:00Z", []string{"README.md"}},
		{5, "ci: pin", []string{"skip-changelog"}, "alice", "User", "2026-01-05T10:00:00Z", []string{".github/workflows/ci.yaml"}},
		{6, "refactor!: drop old", []string{"breaking", "refactor"}, "carol", "User", "2026-01-02T10:00:00Z", []string{"src/old.go"}},
		{7, "Uncategorized thing", []string{}, "", "", "2026-01-06T10:00:00Z", []string{"misc.txt"}},
	}
	for i := 8; i < 14; i++ {
		specs = append(specs, spec{i, fmt.Sprintf("chore(deps): bump dep%d", i), []string{"dependencies"}, "renovate", "Bot", fmt.Sprintf("2026-01-0%dT12:00:00Z", i-6), []string{"go.sum"}})
	}
	f := fixture{files: map[int][]string{}}
	for _, s := range specs {
		pr := model.PullRequest{
			Number: s.n, Title: s.title, Labels: s.labels, Merged: true,
			URL: str(fmt.Sprintf("https://github.com/o/r/pull/%d", s.n)), Body: str(fmt.Sprintf("body %d", s.n)),
			BaseRepository: "o/r", MergedAt: str(s.merged), BaseRefName: str("main"), HeadRefName: str(fmt.Sprintf("branch-%d", s.n)),
		}
		authors := []model.CommitAuthor{{Name: "Some Bot"}}
		if s.author != "" {
			url := "https://github.com/" + s.author
			if s.typename == "Bot" {
				url = "https://github.com/apps/" + s.author
			}
			pr.Author = &model.Actor{Typename: s.typename, Login: s.author, URL: url}
			if s.typename == "User" {
				authors = []model.CommitAuthor{{Name: s.author, Login: s.author}}
			}
		}
		switch s.n {
		case 1:
			authors = append(authors, model.CommitAuthor{Name: "Dave NoUser"})
		case 6:
			authors = append(authors, model.CommitAuthor{Name: "eve", Login: "eve"})
		}
		f.prs = append(f.prs, pr)
		f.files[s.n] = s.files
		f.commits = append(f.commits, model.Commit{
			OID: fmt.Sprintf("%040x", s.n), CommittedDate: s.merged, Message: s.title,
			Author: &authors[0], Authors: authors, PullRequests: []model.PullRequest{pr},
		})
	}
	return f
}
