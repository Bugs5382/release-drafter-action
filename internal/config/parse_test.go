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

func TestParseAppliesV7Defaults(t *testing.T) {
	cfg, err := Parse([]byte("template: hello\n"), "test")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ChangeTemplate != "* $TITLE (#$NUMBER) $AUTHORS" || cfg.CategoryTemplate != "## $TITLE" ||
		cfg.NoChangesTemplate != "* No changes" || cfg.VersionTemplate != "$MAJOR.$MINOR.$PATCH$PRERELEASE" ||
		cfg.PullRequestLimit != 5 || cfg.HistoryLimit != 15 || cfg.SortBy != "merged_at" ||
		cfg.SortDirection != "descending" || cfg.VersionResolver.Default != "patch" || cfg.Template != "hello" {
		t.Errorf("defaults not applied: %+v", cfg)
	}
	if cfg.ChangeAuthorsFinalSeparator != nil || cfg.Latest != nil || cfg.Prerelease != nil {
		t.Error("unset optional values must stay nil")
	}
}

func TestParseHubConfig(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", ".github", "release-drafter.yml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse(data, "hub.yml")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Categories) != 10 || !strings.HasSuffix(cfg.Categories[9].Title, " Dependency Updates") || cfg.Categories[9].CollapseAfter != 5 {
		t.Errorf("categories = %+v", cfg.Categories)
	}
	if got := cfg.Categories[2].Labels; len(got) != 2 || got[0] != "bug" || got[1] != "fix" {
		t.Errorf("bug fix labels = %q", got)
	}
	if cfg.ChangeTitleEscapes != `\<*_&` || cfg.ChangeTemplate != "- $TITLE @$AUTHOR (#$NUMBER)" {
		t.Errorf("templates = %q %q", cfg.ChangeTitleEscapes, cfg.ChangeTemplate)
	}
	if len(cfg.ExcludeLabels) != 1 || cfg.ExcludeLabels[0] != "skip-changelog" {
		t.Errorf("exclude-labels = %q", cfg.ExcludeLabels)
	}
	if !cfg.HasAutolabeler || len(cfg.Autolabeler) != 9 || cfg.Autolabeler[8].Title[1] != `/^chore(?!\(deps)(\(.+\))?:/` {
		t.Errorf("autolabeler = %+v", cfg.Autolabeler)
	}
	if !strings.HasSuffix(cfg.Template, "...v$RESOLVED_VERSION\n") {
		t.Errorf("template must keep its trailing newline: %q", cfg.Template)
	}
}

func TestParseRejectsUnknownKeysWithPath(t *testing.T) {
	_, err := Parse([]byte("template: x\ncategories:\n  - title: A\n    lables: [bug]\ncategroies: []\n"), "input config")
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v; want *ValidationError", err)
	}
	want := []string{
		`categories[0].lables (line 4): unknown key "lables"`,
		`categroies (line 5): unknown key "categroies"`,
	}
	if len(ve.Errors) != len(want) {
		t.Fatalf("errors = %v", ve.Errors)
	}
	for i, w := range want {
		if ve.Errors[i].String() != w {
			t.Errorf("error %d = %q; want %q", i, ve.Errors[i].String(), w)
		}
	}
}

func TestParseTypeErrorsNameTheExpectedValue(t *testing.T) {
	cases := map[string]string{
		"categories:\n  - title: A\n    collapse-after: five\n": `categories[0].collapse-after (line 3): expected an integer >= -1, got str "five"`,
		"sort-by: date\n":                                     `sort-by (line 1): expected one of merged_at, title, got str "date"`,
		"filter-by-commitish: 'yes'\n":                        `filter-by-commitish (line 1): expected true or false, got str "yes"`,
		"pull-request-limit: 0\n":                             `pull-request-limit (line 1): expected an integer >= 1, got int "0"`,
		"name-template: 1.0\n":                                `name-template (line 1): expected a string, got float "1.0"`,
		"categories:\n":                                       `categories (line 1): expected a list, got null`,
		"replacers:\n  - search: x\n":                         `replacers[0].replace (line 2): is required`,
		"autolabeler:\n  - title: [x]\n":                      `autolabeler[0].label (line 2): is required`,
		"_extends: org/.github\n":                             `_extends (line 1): is not supported: use the extends input to load a shared config from a release asset`,
		"categories:\n  - when:\n      conventional: false\n": `categories[0].when.conventional (line 3): expected true or a mapping, got bool "false"`,
	}
	for in, want := range cases {
		_, err := Parse([]byte(in), "test")
		var ve *ValidationError
		if !errors.As(err, &ve) || len(ve.Errors) != 1 || ve.Errors[0].String() != want {
			t.Errorf("Parse(%q) = %v; want %q", in, err, want)
		}
	}
}

func TestParseAcceptsV7ValueShapes(t *testing.T) {
	in := "prerelease: 'yes'\nlatest: false\nversion-template: 2024-01-01\npull-request-limit: 10.0\n" +
		"categories:\n  - title: A\n    when: {labels: [a], conventional: true}\n  - title: B\n    when:\n      - paths: [src/**]\n        paths-mode: all\n"
	cfg, err := Parse([]byte(in), "test")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Prerelease == nil || !*cfg.Prerelease || cfg.Latest == nil || *cfg.Latest {
		t.Errorf("booleans = %v %v", cfg.Prerelease, cfg.Latest)
	}
	if cfg.VersionTemplate != "2024-01-01" || cfg.PullRequestLimit != 10 {
		t.Errorf("scalars = %q %d", cfg.VersionTemplate, cfg.PullRequestLimit)
	}
	if !cfg.Categories[0].When[0].Conventional.Any || cfg.Categories[1].When[0].PathsMode != "all" {
		t.Errorf("when = %+v", cfg.Categories)
	}
}

func TestParseEmptyAndBrokenYAML(t *testing.T) {
	for _, in := range []string{"", "   \n", "a: [unclosed\n", "- a list\n"} {
		if _, err := Parse([]byte(in), "test"); err == nil {
			t.Errorf("Parse(%q) should fail", in)
		}
	}
}

func TestParseBool(t *testing.T) {
	for in, want := range map[string]bool{"true": true, "YES": true, "on": true, "1": true, "y": true, "enabled": true, "false": false, "No": false, "off": false, "0": false, "n": false, "disabled": false} {
		got, err := ParseBool(in)
		if err != nil || got != want {
			t.Errorf("ParseBool(%q) = %v, %v", in, got, err)
		}
	}
	if _, err := ParseBool("maybe"); err == nil {
		t.Error("ParseBool(maybe) should fail")
	}
}

func TestNormalizeConfigName(t *testing.T) {
	cases := map[string]string{
		"":                            ".github/release-drafter.yml",
		"release-drafter.yml":         ".github/release-drafter.yml",
		".github/release-drafter.yml": ".github/release-drafter.yml",
		"file:drafter.yaml":           ".github/drafter.yaml",
		"/configs/rd.yml":             "configs/rd.yml",
		"../configs/rd.json":          "configs/rd.json",
	}
	for in, want := range cases {
		got, err := NormalizeConfigName(in)
		if err != nil || got != want {
			t.Errorf("NormalizeConfigName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"org/.github:release-drafter.yml", "github:x.yml", "rd.yml@main", "rd.toml", "../../x.yml"} {
		if _, err := NormalizeConfigName(bad); err == nil {
			t.Errorf("NormalizeConfigName(%q) should fail", bad)
		}
	}
}

func TestReadLocal(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".github"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".github", "release-drafter.yml"), []byte("template: file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	src, err := ReadLocal("release-drafter.yml", ws)
	if err != nil || string(src.Text) != "template: file\n" || src.Origin != ".github/release-drafter.yml" || src.Kind != KindFile {
		t.Errorf("file: %+v %v", src, err)
	}
	_, err = ReadLocal("missing.yml", ws)
	if !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "missing.yml") || !strings.Contains(err.Error(), "check out") {
		t.Errorf("missing: %v", err)
	}
	if _, err := ReadLocal("org/.github:release-drafter.yml", ws); err == nil || !strings.Contains(err.Error(), "extends input") {
		t.Errorf("remote: %v", err)
	}
}

// Review focus: an org variable edited on Windows, or pasted with a blank
// first line, must parse exactly like the file.
func TestParseCRLFAndPaddedText(t *testing.T) {
	unix := "template: |\n  a\n  b\ncategories:\n  - title: A\n    labels: [x]\n"
	crlf := "\r\n" + strings.ReplaceAll(unix, "\n", "\r\n")
	a, err := Parse([]byte(unix), "unix")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse([]byte(crlf), "crlf")
	if err != nil {
		t.Fatal(err)
	}
	if a.Template != "a\nb\n" || b.Template != a.Template || b.Categories[0].Title != "A" {
		t.Errorf("templates %q and %q", a.Template, b.Template)
	}
}

// Review focus: v7 accepts a .json config file.
func TestReadLocalAndParseJSON(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".github"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".github", "drafter.json"), []byte(`{"template":"$CHANGES","categories":[{"title":"A","labels":["a"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	src, err := ReadLocal("drafter.json", ws)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse(src.Text, src.Origin)
	if err != nil || cfg.Template != "$CHANGES" || cfg.Categories[0].Labels[0] != "a" {
		t.Errorf("cfg = %+v, %v", cfg, err)
	}
}

// v7's yaml parse rejects a key repeated in one mapping, at any level.
func TestParseRejectsDuplicateKeys(t *testing.T) {
	cases := map[string]string{
		"categories: []\ntemplate: x\ncategories: []\n":           `categories (line 3): duplicate key "categories" (first at line 1)`,
		"categories:\n  - title: A\n    title: B\n":               `categories[0].title (line 3): duplicate key "title" (first at line 2)`,
		"version-resolver:\n  default: minor\n  default: major\n": `version-resolver.default (line 3): duplicate key "default" (first at line 2)`,
	}
	for in, want := range cases {
		_, err := Parse([]byte(in), "test")
		var ve *ValidationError
		if !errors.As(err, &ve) || len(ve.Errors) != 1 || ve.Errors[0].String() != want {
			t.Errorf("Parse(%q) = %v; want %q", in, err, want)
		}
	}
}

// v7's yaml parse rejects text that holds more than one document.
func TestParseRejectsMoreThanOneDocument(t *testing.T) {
	for _, in := range []string{"template: a\n---\nbogus: 1\n", "template: a\n---\ntemplate: b\n"} {
		_, err := Parse([]byte(in), "test")
		var ve *ValidationError
		if !errors.As(err, &ve) || len(ve.Errors) != 1 || ve.Errors[0].Path != "(root)" ||
			!strings.Contains(ve.Errors[0].Msg, "the config must be a single YAML document") {
			t.Errorf("Parse(%q) = %v; want a single-document error at (root)", in, err)
		}
	}
	if _, err := Parse([]byte("---\ntemplate: a\n"), "test"); err != nil {
		t.Errorf("a leading document marker is still one document: %v", err)
	}
}

// zod's .int() stops at Number.MAX_SAFE_INTEGER.
func TestParseRejectsIntegersPastMaxSafeInteger(t *testing.T) {
	cases := map[string]string{
		"history-limit: 1e30\n":             `history-limit (line 1): expected an integer >= 1, got float "1e30"`,
		"history-limit: 9007199254740992\n": `history-limit (line 1): expected an integer >= 1, got int "9007199254740992"`,
	}
	for in, want := range cases {
		_, err := Parse([]byte(in), "test")
		var ve *ValidationError
		if !errors.As(err, &ve) || len(ve.Errors) != 1 || ve.Errors[0].String() != want {
			t.Errorf("Parse(%q) = %v; want %q", in, err, want)
		}
	}
	cfg, err := Parse([]byte("history-limit: 9007199254740991\n"), "test")
	if err != nil || cfg.HistoryLimit != 9007199254740991 {
		t.Errorf("the largest safe integer must pass: %+v, %v", cfg, err)
	}
}

func TestValidationErrorWithoutOrigin(t *testing.T) {
	e := &ValidationError{Errors: []FieldError{{Path: "filter-by-range", Msg: "bad"}}}
	if got := e.Error(); got != "invalid config:\nfilter-by-range: bad" {
		t.Errorf("Error() = %q", got)
	}
	e.Origin = "input config"
	if got := e.Error(); got != "invalid config in input config:\nfilter-by-range: bad" {
		t.Errorf("Error() = %q", got)
	}
}
