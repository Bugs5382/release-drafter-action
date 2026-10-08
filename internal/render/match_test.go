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
	"github.com/Bugs5382/release-drafter-action/internal/pathmatch"
)

func cond(labels []string, labelsMode string, paths []string, pathsMode string, conv *config.ParsedConventional) config.ParsedCondition {
	c := config.ParsedCondition{Labels: labels, LabelsMode: labelsMode, Paths: paths, PathsMode: pathsMode, Conventional: conv}
	for _, p := range uniq(paths) {
		c.Matchers = append(c.Matchers, pathmatch.New(p))
	}
	return c
}

func conv(types, scopes []string, breaking *bool) *config.ParsedConventional {
	return &config.ParsedConventional{Types: types, Scopes: scopes, Breaking: breaking}
}

// Each row is one condition; each column one pull request. The expected
// booleans come from v7.7.0's matchesCategoryCondition.
func TestMatchesConditionMatchesV7(t *testing.T) {
	yes, no := true, false
	prs := []model.PullRequest{
		{Title: "feat(api)!: x", Labels: []string{"a", "b"}, ChangedFiles: []string{"src/a.go", "README.md"}},
		{Title: "fix: y", Labels: []string{"a"}, ChangedFiles: []string{"src/b.go"}},
		{Title: "docs: z", Labels: []string{}, ChangedFiles: []string{}},
		{Title: "not conventional", Labels: []string{"b", "c"}},
	}
	none := []string{}
	conds := []config.ParsedCondition{
		cond([]string{"a"}, "any", none, "any", nil),
		cond([]string{"a", "b"}, "all", none, "any", nil),
		cond([]string{"a", "b"}, "only", none, "any", nil),
		cond([]string{"a", "b"}, "exactly", none, "any", nil),
		cond(none, "any", []string{"src/**"}, "any", nil),
		cond(none, "any", []string{"src/**"}, "only", nil),
		cond(none, "any", []string{"src/**", "README.md"}, "all", nil),
		cond(none, "any", []string{"src/**", "README.md"}, "exactly", nil),
		cond(none, "any", none, "any", conv(none, none, nil)),
		cond(none, "any", none, "any", conv([]string{"feat"}, none, nil)),
		cond(none, "any", none, "any", conv(none, []string{"api"}, nil)),
		cond(none, "any", none, "any", conv(none, none, &yes)),
		cond(none, "any", none, "any", conv([]string{"fix"}, none, &no)),
		cond([]string{"a"}, "any", none, "any", conv([]string{"fix"}, none, nil)),
	}
	want := [][]bool{
		{true, true, false, false},
		{true, false, false, false},
		{true, true, false, false},
		{true, false, false, false},
		{true, true, false, false},
		{false, true, false, false},
		{true, false, false, false},
		{true, false, false, false},
		{true, true, true, false},
		{true, false, false, false},
		{true, false, false, false},
		{true, false, false, false},
		{false, true, false, false},
		{false, true, false, false},
	}
	for i, c := range conds {
		for j, pr := range prs {
			if got := MatchesCondition(c, pr); got != want[i][j] {
				t.Errorf("condition %d, pull request %d: got %v, want %v", i, j, got, want[i][j])
			}
		}
	}
}

func TestFilterPreAndCategoryWithoutConditions(t *testing.T) {
	prs := []model.PullRequest{{Number: 1, Labels: []string{"keep"}}, {Number: 2, Labels: []string{"keep", "skip"}}, {Number: 3}}
	cats := []config.ParsedCategory{
		{Type: "pre-include", When: []config.ParsedCondition{cond([]string{"keep"}, "any", []string{}, "any", nil)}},
		{Type: "pre-exclude", When: []config.ParsedCondition{cond([]string{"skip"}, "any", []string{}, "any", nil)}},
	}
	got := FilterPre(prs, cats)
	if len(got) != 1 || got[0].Number != 1 {
		t.Errorf("FilterPre = %+v", got)
	}
	if !MatchesCategory(config.ParsedCategory{Type: "changelog"}, prs[2]) {
		t.Error("a category without conditions matches everything")
	}
	if NeedsChangedFiles(cats) {
		t.Error("label-only categories do not need changed files")
	}
}

func TestFilterBots(t *testing.T) {
	prs := []model.PullRequest{
		{Number: 1, Author: &model.Actor{Typename: "User", Login: "alice"}},
		{Number: 2, Author: &model.Actor{Typename: "Bot", Login: "some-app"}},
		{Number: 3, Author: &model.Actor{Typename: "User", Login: "dependabot[bot]"}},
		{Number: 4},
	}
	got := FilterBots(prs)
	if len(got) != 2 || got[0].Number != 1 || got[1].Number != 4 {
		t.Errorf("FilterBots = %+v", got)
	}
}

func TestNeedsPullRequestFields(t *testing.T) {
	cases := []struct {
		template                            string
		body, url, baseRefName, headRefName bool
	}{
		{"* $TITLE (#$NUMBER) $AUTHORS", false, false, false, false},
		{"* $TITLE\n$BODY", true, false, false, false},
		{"* [$TITLE]($URL)", false, true, false, false},
		{"* $TITLE ($BASE_REF_NAME -> $HEAD_REF_NAME)", false, false, true, true},
		{"$BODY $URL $BASE_REF_NAME $HEAD_REF_NAME", true, true, true, true},
	}
	for _, c := range cases {
		body, url, baseRefName, headRefName := NeedsPullRequestFields(c.template)
		if body != c.body || url != c.url || baseRefName != c.baseRefName || headRefName != c.headRefName {
			t.Errorf("NeedsPullRequestFields(%q) = (%v, %v, %v, %v), want (%v, %v, %v, %v)",
				c.template, body, url, baseRefName, headRefName, c.body, c.url, c.baseRefName, c.headRefName)
		}
	}
}

func TestNeedsNewContributors(t *testing.T) {
	if NeedsNewContributors(config.Config{Template: "$CHANGES"}) {
		t.Error("a template without $NEW_CONTRIBUTORS does not need them")
	}
	if !NeedsNewContributors(config.Config{Template: "$CHANGES\n$NEW_CONTRIBUTORS"}) {
		t.Error("a template with $NEW_CONTRIBUTORS needs them")
	}
	if !NeedsNewContributors(config.Config{Header: "$NEW_CONTRIBUTORS"}) {
		t.Error("a header with $NEW_CONTRIBUTORS needs them")
	}
	if !NeedsNewContributors(config.Config{Footer: "$NEW_CONTRIBUTORS"}) {
		t.Error("a footer with $NEW_CONTRIBUTORS needs them")
	}
}
