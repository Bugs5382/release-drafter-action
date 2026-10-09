package config

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
	"testing"

	"github.com/Bugs5382/release-drafter-action/internal/regex"
)

// labelsFor runs the compiled autolabeler rules over one pull request the
// way the autolabeler does: a rule matches when any of its patterns does.
func labelsFor(t *testing.T, rules []CompiledAutolabel, branch, title, body string) []string {
	t.Helper()
	var out []string
	for _, r := range rules {
		matched := false
		for _, group := range []struct {
			text string
			res  []*regex.Regex
		}{{branch, r.Branch}, {title, r.Title}, {body, r.Body}} {
			for _, re := range group.res {
				ok, err := re.MatchString(group.text)
				if err != nil {
					t.Fatal(err)
				}
				matched = matched || ok
			}
		}
		if matched && !slices.Contains(out, r.Label) {
			out = append(out, r.Label)
		}
	}
	return out
}

func TestBuiltinDefaultParsesAndMerges(t *testing.T) {
	cfg, err := BuiltinDefault(false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ExcludeBots || len(cfg.ExcludeContributors) != 0 {
		t.Errorf("exclude-bots off must leave bots in: %v %q", cfg.ExcludeBots, cfg.ExcludeContributors)
	}
	if pre := cfg.Categories[0]; pre.Type != "pre-exclude" || pre.When[0].Labels[0] != "skip-changelog" ||
		strings.Join(pre.When[1].Conventional.Types, ",") != "ci,test,style" {
		t.Errorf("pre-exclude = %+v", pre)
	}
	var titles, increments []string
	for _, c := range cfg.Categories[1:] {
		titles = append(titles, c.Title[strings.Index(c.Title, " ")+1:])
		increments = append(increments, c.SemverIncrement)
	}
	if got := strings.Join(titles, ","); got != "Breaking Changes,Features,Bug Fixes,Performance,Refactoring,Security,Documentation,Dependency Updates" {
		t.Errorf("categories = %s", got)
	}
	if got := strings.Join(increments, ","); got != "major,minor,patch,patch,patch,patch,patch,patch" {
		t.Errorf("semver-increment = %s", got)
	}
	if !cfg.Categories[1].Exclusive || cfg.Categories[8].CollapseAfter != 5 {
		t.Errorf("breaking exclusive %v, dependency collapse-after %d", cfg.Categories[1].Exclusive, cfg.Categories[8].CollapseAfter)
	}
	if deps := cfg.Categories[8].When[1].Conventional; strings.Join(deps.Scopes, ",") != "deps,deps-dev" {
		t.Errorf("dependency conventional = %+v", deps)
	}
	if cfg.ChangeTemplate != "- $TITLE @$AUTHOR (#$NUMBER)" || cfg.CategoryTemplate != "### $TITLE" || len(cfg.ExcludeLabels) != 0 {
		t.Errorf("change-template %q, category-template %q, exclude-labels %q", cfg.ChangeTemplate, cfg.CategoryTemplate, cfg.ExcludeLabels)
	}
	// The "Contributors" section lists only first-time contributors, the
	// same as upstream release-drafter's own default and GitHub's native
	// release notes: a roster of every contributor to the release (every
	// author with a merged pull request in range, whether or not it is
	// their first) is a different thing release-drafter calls $CONTRIBUTORS,
	// and is not what the built-in default renders.
	if !strings.Contains(cfg.Template, "$NEW_CONTRIBUTORS") {
		t.Errorf("template must render $NEW_CONTRIBUTORS: %q", cfg.Template)
	}
	if strings.Contains(cfg.Template, "$CONTRIBUTORS") {
		t.Errorf("template must not render the full-roster $CONTRIBUTORS: %q", cfg.Template)
	}
	p, err := Merge(cfg, Inputs{}, "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Warnings) != 0 {
		t.Errorf("the default must not use deprecated keys: %q", p.Warnings)
	}
	if len(p.Replacers) != 1 {
		t.Fatalf("replacers = %+v", p.Replacers)
	}
	for in, want := range map[string]string{
		"- chore(deps): bump x @[dependabot[bot]](https://github.com/apps/dependabot) (#3)": "- chore(deps): bump x [@dependabot](https://github.com/apps/dependabot) (#3)",
		"@alice, [@renovate[bot]](https://github.com/apps/renovate) and @bob":               "@alice, [@renovate](https://github.com/apps/renovate) and @bob",
		"- feat: add cache @alice (#1)":                                                     "- feat: add cache @alice (#1)",
	} {
		got, err := p.Replacers[0].Search.Replace(in, func(g []string) string { return "[@" + g[1] + "](" })
		if err != nil || got != want {
			t.Errorf("replace(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestBuiltinDefaultAutolabeler(t *testing.T) {
	cfg, err := BuiltinDefault(false)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := CompileAutolabeler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ branch, title, body, want string }{
		{"feat/x", "feat(api): add cache", "", "enhancement"},
		{"x", "feat!: drop v1 endpoints", "", "breaking"},
		{"x", "refactor: split the client", "Notes\n\nBREAKING CHANGE: the client moved", "breaking,refactor"},
		{"x", "fix: nil map", "", "fix"},
		{"renovate/x-1.x", "fix(deps): update module x to v1.4.1", "", "dependencies"},
		{"x", "perf: cache compiled patterns", "", "performance"},
		{"x", "docs: usage", "", "documentation"},
		{"x", "security: bump the TLS floor", "", "security"},
		{"dependabot/go_modules/x-1.2.3", "Bump x from 1.2.2 to 1.2.3", "", "dependencies"},
		{"renovate/x-2.x", "chore(deps): update module x to v2", "", "dependencies"},
		{"x", "build(deps-dev): bump y", "", "dependencies"},
		{"x", "chore: tidy", "", "skip-changelog"},
		{"x", "build: new Dockerfile stage", "", "skip-changelog"},
		{"x", "ci(lint): bump golangci-lint", "", "skip-changelog"},
		{"x", "Update README", "", ""},
	}
	for _, c := range cases {
		if got := strings.Join(labelsFor(t, rules, c.branch, c.title, c.body), ","); got != c.want {
			t.Errorf("%q on %q: labels %q; want %q", c.title, c.branch, got, c.want)
		}
	}
}

func TestBuiltinDefaultExcludeBots(t *testing.T) {
	cfg, err := BuiltinDefault(true)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ExcludeBots || strings.Join(cfg.ExcludeContributors, ",") != "dependabot,renovate,github-actions" {
		t.Errorf("exclude-bots on: %v %q", cfg.ExcludeBots, cfg.ExcludeContributors)
	}
	again, err := BuiltinDefault(false)
	if err != nil || again.ExcludeBots || len(again.ExcludeContributors) != 0 {
		t.Errorf("BuiltinDefault must not share state between calls: %+v %v", again, err)
	}
	for _, c := range []struct {
		typename, login string
		want            bool
	}{{"Bot", "some-app", true}, {"User", "dependabot[bot]", true}, {"User", "renovate", true}, {"User", "alice", false}, {"Organization", "acme", false}} {
		if got := IsBot(c.typename, c.login); got != c.want {
			t.Errorf("IsBot(%q, %q) = %v", c.typename, c.login, got)
		}
	}
}

func TestDefaultTextIsACopy(t *testing.T) {
	a := DefaultText()
	a[0] = 'X'
	if DefaultText()[0] != '#' {
		t.Error("DefaultText must return a copy")
	}
}
