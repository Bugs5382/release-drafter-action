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
	"strconv"
	"strings"

	"github.com/Bugs5382/release-drafter-action/internal/config"
	"github.com/Bugs5382/release-drafter-action/internal/logging"
	"github.com/Bugs5382/release-drafter-action/internal/model"
)

func itoa(n int) string { return strconv.Itoa(n) }

// Bucket is a changelog category with the pull requests assigned to it.
type Bucket struct {
	Category     config.ParsedCategory
	PullRequests []model.PullRequest
}

// Categorize ports v7's categorizePullRequests. It returns the pull
// requests no category took (only when there is no uncategorized category)
// and one bucket per changelog category, in config order. A pull request
// joins every category it matches until an exclusive one stops it.
func Categorize(prs []model.PullRequest, cats []config.ParsedCategory) ([]model.PullRequest, []Bucket) {
	changelog := OfType(cats, "changelog")
	buckets := make([]Bucket, len(changelog))
	uncategorizedIndex := -1
	for i, c := range changelog {
		buckets[i] = Bucket{Category: c, PullRequests: []model.PullRequest{}}
		if len(c.When) == 0 && uncategorizedIndex == -1 {
			uncategorizedIndex = i
		}
	}
	logging.L().Debug().Int("pull_requests", len(prs)).Int("changelog_categories", len(changelog)).
		Int("uncategorized_index", uncategorizedIndex).Msg("render: categorizing pull requests")

	var uncategorized []model.PullRequest
	for _, pr := range FilterPre(prs, cats) {
		matched := false
		for i := range buckets {
			if len(buckets[i].Category.When) == 0 {
				continue
			}
			if MatchesCategory(buckets[i].Category, pr) {
				buckets[i].PullRequests = append(buckets[i].PullRequests, pr)
				matched = true
				logging.L().Trace().Int("pull_request", pr.Number).Str("category", buckets[i].Category.Title).
					Bool("exclusive_stopped", buckets[i].Category.Exclusive).Msg("render: pull request matched a category")
				if buckets[i].Category.Exclusive {
					break
				}
			}
		}
		if !matched {
			if uncategorizedIndex == -1 {
				uncategorized = append(uncategorized, pr)
			} else {
				buckets[uncategorizedIndex].PullRequests = append(buckets[uncategorizedIndex].PullRequests, pr)
			}
			logging.L().Trace().Int("pull_request", pr.Number).Msg("render: pull request fell to uncategorized")
		}
	}
	for _, b := range buckets {
		logging.L().Debug().Str("category", b.Category.Title).Int("count", len(b.PullRequests)).
			Msg("render: categorized bucket")
	}
	logging.L().Debug().Int("uncategorized", len(uncategorized)).Msg("render: finished categorizing pull requests")
	return uncategorized, buckets
}

// ChangeLines renders change-template for each pull request, joined by
// newlines. category fills $CATEGORY.
func ChangeLines(prs []model.PullRequest, category string, commits []model.Commit, p *config.Parsed, serverURL string) string {
	logging.L().Debug().Int("pull_requests", len(prs)).Str("category", category).Msg("render: rendering change lines")
	c := p.Config
	noAuthors := Render(c.ChangeAuthorTemplate, Vars{"$AUTHOR": "ghost", "$AUTHOR_MENTION": "@ghost"})
	lines := make([]string, len(prs))
	for i, pr := range prs {
		author := "ghost"
		authorURL := ""
		if pr.Author != nil {
			author = pr.Author.Login
			if pr.Author.Typename == "Bot" {
				author = fmt.Sprintf("[%s[bot]](%s)", pr.Author.Login, pr.Author.URL)
			}
			authorURL = pr.Author.URL
		}
		tmpl := c.ChangeAuthorTemplate
		vars := Vars{
			"$CATEGORY": category,
			"$TITLE":    EscapeTitle(pr.Title, c.ChangeTitleEscapes),
			"$NUMBER":   itoa(pr.Number),
			"$AUTHORS": AuthorsSentence(AuthorsOptions{
				Commits:           commits,
				PullRequests:      []model.PullRequest{pr},
				NoAuthorsTemplate: noAuthors,
				AuthorTemplate:    &tmpl,
				Separator:         c.ChangeAuthorsSeparator,
				FinalSeparator:    c.ChangeAuthorsFinalSeparator,
				ServerURL:         serverURL,
			}),
			"$AUTHOR":     author,
			"$AUTHOR_URL": authorURL,
		}
		for k, v := range map[string]*string{"$BODY": pr.Body, "$URL": pr.URL, "$BASE_REF_NAME": pr.BaseRefName, "$HEAD_REF_NAME": pr.HeadRefName} {
			if v != nil {
				vars[k] = *v
			}
		}
		lines[i] = Render(c.ChangeTemplate, vars)
	}
	return strings.Join(lines, "\n")
}

// Changelog renders $CHANGES exactly as v7's generateChangeLog: the
// uncategorized list first, then each non-empty category under its
// category-template title, collapsed into <details> past collapse-after,
// separated by blank lines and trimmed.
func Changelog(commits []model.Commit, prs []model.PullRequest, p *config.Parsed, serverURL string) string {
	logging.L().Debug().Int("pull_requests", len(prs)).Msg("render: rendering the changelog")
	uncategorized, buckets := Categorize(prs, p.Categories)
	total := len(uncategorized)
	for _, b := range buckets {
		total += len(b.PullRequests)
	}
	if total == 0 {
		out := p.Config.NoChangesTemplate
		logging.L().Debug().Int("output_length", len(out)).Bool("used_no_changes_template", true).
			Msg("render: finished rendering the changelog")
		return out
	}
	var out strings.Builder
	if len(uncategorized) > 0 {
		out.WriteString(ChangeLines(uncategorized, "", commits, p, serverURL))
		out.WriteString("\n\n")
	}
	for i, b := range buckets {
		n := len(b.PullRequests)
		if n == 0 {
			logging.L().Trace().Str("category", b.Category.Title).Int("count", 0).
				Int("collapse_after", b.Category.CollapseAfter).Msg("render: skipped empty category")
			continue
		}
		if title := Render(p.Config.CategoryTemplate, Vars{"$TITLE": b.Category.Title}); title != "" {
			out.WriteString(title)
			out.WriteString("\n\n")
		}
		lines := ChangeLines(b.PullRequests, b.Category.Title, commits, p, serverURL)
		if b.Category.CollapseAfter != -1 && n > b.Category.CollapseAfter {
			plural := ""
			if n > 1 {
				plural = "s"
			}
			fmt.Fprintf(&out, "<details>\n<summary>%d change%s</summary>\n\n%s\n</details>", n, plural, lines)
			logging.L().Trace().Str("category", b.Category.Title).Int("count", n).
				Int("collapse_after", b.Category.CollapseAfter).Msg("render: collapsed category")
		} else {
			out.WriteString(lines)
			logging.L().Trace().Str("category", b.Category.Title).Int("count", n).
				Int("collapse_after", b.Category.CollapseAfter).Msg("render: rendered category")
		}
		if i+1 != len(buckets) {
			out.WriteString("\n\n")
		}
	}
	result := trimJS(out.String())
	logging.L().Debug().Int("output_length", len(result)).Bool("used_no_changes_template", false).
		Msg("render: finished rendering the changelog")
	return result
}

// jsSpace reports whether String.prototype.trim removes r: ECMAScript's
// WhiteSpace and LineTerminator characters, which include the byte order
// mark and exclude U+0085.
func jsSpace(r rune) bool {
	switch r {
	case byteOrderMark, lineSeparator, paragraphSeparator,
		' ', '\t', '\n', '\v', '\f', '\r',
		0x00A0, 0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006,
		0x2007, 0x2008, 0x2009, 0x200A, 0x202F, 0x205F, 0x3000:
		return true
	}
	return false
}

// trimJS is String.prototype.trim.
func trimJS(s string) string {
	return strings.TrimFunc(s, jsSpace)
}
