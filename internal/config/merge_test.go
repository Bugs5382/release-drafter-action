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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustParse(t *testing.T, text string) *Config {
	t.Helper()
	cfg, err := Parse([]byte(text), "test")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func ptr[T any](v T) *T { return &v }

func TestMergeHubConfigMigratesDeprecatedKeys(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", ".github", "release-drafter.yml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse(data, "hub.yml")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Merge(cfg, Inputs{}, "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, c := range p.Categories {
		types = append(types, c.Type)
	}
	want := "changelog changelog changelog changelog changelog changelog changelog changelog changelog changelog pre-exclude version-resolver version-resolver version-resolver"
	if strings.Join(types, " ") != want {
		t.Errorf("types = %s", strings.Join(types, " "))
	}
	bugs := p.Categories[2].When
	if len(bugs) != 1 || strings.Join(bugs[0].Labels, ",") != "bug,fix" || bugs[0].LabelsMode != "any" {
		t.Errorf("bug fixes when = %+v", bugs)
	}
	if got := p.Categories[10].When[0].Labels; len(got) != 1 || got[0] != "skip-changelog" {
		t.Errorf("pre-exclude labels = %q", got)
	}
	if p.Categories[11].SemverIncrement != "major" || p.Categories[13].SemverIncrement != "patch" || len(p.Categories[13].When[0].Labels) != 10 {
		t.Errorf("version-resolver categories = %+v", p.Categories[11:])
	}
	if len(p.Warnings) != 14 {
		t.Errorf("got %d warnings, want 14:\n%s", len(p.Warnings), strings.Join(p.Warnings, "\n"))
	}
	want0 := `Use of deprecated 'categories[*].label' or 'categories[*].labels' field detected on category "` + cfg.Categories[0].Title +
		`". Please migrate. This field will be removed in a future release. To migrate, move the labels into the category's 'when' condition. Migration documentation: https://github.com/release-drafter/release-drafter/pull/1558`
	if p.Warnings[0] != want0 {
		t.Errorf("first warning = %q", p.Warnings[0])
	}
	if p.Commitish != "refs/heads/main" || !p.Latest || p.Prerelease || len(p.Replacers) != 1 || !p.Replacers[0].Search.Global {
		t.Errorf("parsed = %+v", p)
	}
}

func TestMergeInputsOverrideConfig(t *testing.T) {
	cfg := mustParse(t, "template: x\ncommitish: develop\nheader: from-config\nprerelease: true\n")
	p, err := Merge(cfg, Inputs{Commitish: "main", Header: "from-input", Prerelease: ptr(false)}, "refs/heads/ignored")
	if err != nil {
		t.Fatal(err)
	}
	if p.Commitish != "main" || p.Config.Header != "from-input" || p.Prerelease {
		t.Errorf("parsed = %+v", p)
	}
	want := []string{
		`Input's commitish "main" overrides config's commitish "develop"`,
		`Input's header "from-input" overrides config's header "from-config"`,
		`Input's prerelease "false" overrides config's prerelease "true"`,
	}
	if strings.Join(p.Notes, "\n") != strings.Join(want, "\n") {
		t.Errorf("notes = %q", p.Notes)
	}
}

func TestMergeReleaseModeRules(t *testing.T) {
	p, err := Merge(mustParse(t, "latest: true\nprerelease: true\n"), Inputs{}, "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	if p.Latest || !p.Prerelease || len(p.Warnings) != 1 {
		t.Errorf("latest+prerelease: %+v", p)
	}
	p, _ = Merge(mustParse(t, "prerelease-identifier: beta\n"), Inputs{}, "refs/heads/main")
	if !p.Prerelease || p.Warnings[0] != "You specified a 'prerelease-identifier' (beta), but 'prerelease' is set to false. Switching to true." {
		t.Errorf("identifier switches prerelease on: %+v", p)
	}
	p, _ = Merge(mustParse(t, "prerelease-identifier: beta\n"), Inputs{Prerelease: ptr(false)}, "refs/heads/main")
	if p.Prerelease {
		t.Error("an explicit prerelease: false input must win over a config identifier")
	}
	p, _ = Merge(mustParse(t, "template: x\n"), Inputs{Prerelease: ptr(false), PrereleaseIdentifier: "rc"}, "refs/heads/main")
	if !p.Prerelease {
		t.Error("an identifier input switches prerelease on even when the prerelease input is false")
	}
}

func TestMergeValidation(t *testing.T) {
	cases := []struct{ in, path, msg string }{
		{"categories:\n  - title: A\n    labels: [b]\n  - labels: [a]\n", "categories[1].title", "every 'type: \"changelog\"' category must define a non-empty 'title'"},
		{"categories:\n  - title: A\n  - title: B\n", "categories[1]", "more than one 'type: \"changelog\"' category has no 'when' condition"},
		{"filter-by-range: not a range\n", "filter-by-range", "'filter-by-range' value \"not a range\" could not be parsed as a valid semver range"},
		{"categories:\n  - title: A\n  - type: pre-exclude\n    exclusive: true\n", "categories[1].exclusive", "\"exclusive\" can only be set on categories of type \"changelog\" or \"version-resolver\"; it cannot be used on category of type \"pre-exclude\""},
		{"exclude-labels: [a]\ncategories:\n  - type: pre-exclude\n    labels: [b]\n", "exclude-labels", "a 'pre-exclude' category already exists"},
		{"exclude-paths: [a]\ncategories:\n  - type: pre-exclude\n    labels: [b]\n", "exclude-paths", "a 'pre-exclude' category already exists"},
		{"include-labels: [a]\ncategories:\n  - type: pre-include\n    labels: [b]\n", "include-labels", "a 'pre-include' category already exists"},
		{"version-resolver:\n  default: minor\ncategories:\n  - type: version-resolver\n", "version-resolver.default", "a 'version-resolver' category with no 'when' condition already exists"},
	}
	for _, c := range cases {
		_, err := Merge(mustParse(t, c.in), Inputs{}, "refs/heads/main")
		var ve *ValidationError
		if !errors.As(err, &ve) || len(ve.Errors) != 1 || ve.Errors[0].Path != c.path || !strings.HasPrefix(ve.Errors[0].Msg, c.msg) {
			t.Errorf("Merge(%q) = %v; want *ValidationError at %s with %q", c.in, err, c.path, c.msg)
			continue
		}
		if !strings.HasPrefix(err.Error(), "invalid config:\n"+c.path+": "+c.msg) {
			t.Errorf("Merge(%q).Error() = %q", c.in, err)
		}
	}
	// A replacer that does not compile stays a regex error naming its field.
	_, err := Merge(mustParse(t, "replacers:\n  - search: '/(x/'\n    replace: y\n"), Inputs{}, "refs/heads/main")
	if err == nil || !strings.HasPrefix(err.Error(), "replacers[0].search: pattern \"/(x/\" does not compile") {
		t.Errorf("bad replacer: %v", err)
	}
	_, err = Merge(mustParse(t, "template: x\n"), Inputs{}, "")
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Errors[0].Path != "commitish" || !strings.HasPrefix(ve.Errors[0].Msg, "'commitish' is required") {
		t.Errorf("missing commitish: %v", err)
	}
}

func TestNormalizeCategoriesErrorsNameTheCategory(t *testing.T) {
	cfg := mustParse(t, "categories:\n  - title: A\n  - title: B\n    labels: [x]\n  - type: pre-include\n    exclusive: true\n")
	_, _, err := NormalizeCategories(cfg)
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Errors[0].Path != "categories[2].exclusive" {
		t.Errorf("err = %v; want *ValidationError at categories[2].exclusive", err)
	}
	cfg.Categories[2].Exclusive = false
	cfg.Categories[2].Type = "bogus"
	_, _, err = NormalizeCategories(cfg)
	if !errors.As(err, &ve) || ve.Errors[0].Path != "categories[2].type" || ve.Errors[0].Msg != "unsupported category type: bogus" {
		t.Errorf("err = %v; want *ValidationError at categories[2].type", err)
	}
}

func TestNormalizeDropsEmptyConditions(t *testing.T) {
	cats, _, err := NormalizeCategories(mustParse(t, "categories:\n  - title: A\n    when:\n      - labels-mode: all\n      - conventional: {type: feat}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cats[0].When) != 1 || cats[0].When[0].Conventional == nil || cats[0].When[0].Conventional.Types[0] != "feat" {
		t.Errorf("when = %+v", cats[0].When)
	}
}

func TestCompileAutolabeler(t *testing.T) {
	cfg := mustParse(t, "autolabeler:\n  - label: docs\n    files: ['*.md']\n    title: ['/^docs/']\n")
	rules, err := CompileAutolabeler(cfg)
	if err != nil || len(rules) != 1 || rules[0].Files == nil || len(rules[0].Title) != 1 {
		t.Fatalf("rules = %+v, %v", rules, err)
	}
	_, err = CompileAutolabeler(mustParse(t, "autolabeler:\n  - label: x\n    branch: ['/(/']\n"))
	if err == nil || !strings.HasPrefix(err.Error(), "autolabeler[0].branch[0]: pattern") {
		t.Errorf("bad branch regex: %v", err)
	}
	if _, err := CompileAutolabeler(mustParse(t, "template: x\n")); err == nil {
		t.Error("a config without autolabeler rules must fail in autolabel mode")
	}
}
