package pathmatch

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
	"strings"
	"testing"
	"time"

	"github.com/dlclark/regexp2"

	"github.com/Bugs5382/release-drafter-action/internal/regex"
)

// Expected matches were produced by node-ignore 7.0.6, the version
// release-drafter v7.7.0 locks.
var paths = []string{
	"src/a.go", "src/x/y.go", "src/x/x.go", "README.md", "readme.md", "docs/x.md",
	"docs/a/b/x.md", "docs", "root.txt", "sub/root.txt", "x/foo", "foo", "a/b",
	"a/x/y/b", "lib/a.go", "lib/x/a.go", "b.txt", "d.txt", "MAKEFILE",
	".github/workflows/ci.yaml", ".github/release-drafter.yml", "pkg/a.test.ts",
	"cmd/action/main.go", "cmd", "foo1.txt", "foo12.txt", "main.go", "go.mod",
	"abc/x", "abc/x/y", "dir/sub/file", "x/dir/sub/file",
}

func TestMatchAgainstNodeIgnore(t *testing.T) {
	cases := []struct {
		pattern string
		want    []string
	}{
		{"src/**", []string{"src/a.go", "src/x/y.go", "src/x/x.go"}},
		{"*.md", []string{"README.md", "readme.md", "docs/x.md", "docs/a/b/x.md"}},
		{"docs/", []string{"docs/x.md", "docs/a/b/x.md"}},
		{"/root.txt", []string{"root.txt"}},
		{"**/foo", []string{"x/foo", "foo"}},
		{"a/**/b", []string{"a/b", "a/x/y/b"}},
		{"lib/*.go", []string{"lib/a.go"}},
		{"[a-c].txt", []string{"b.txt"}},
		{"[!a-c].txt", []string{"d.txt"}},
		{"Makefile", []string{"MAKEFILE"}},
		{"cmd/", []string{"cmd/action/main.go"}},
		{"foo?.txt", []string{"foo1.txt"}},
		{"docs/**/*.md", []string{"docs/x.md", "docs/a/b/x.md"}},
		{"abc/*", []string{"abc/x", "abc/x/y"}},
		{".github/", []string{".github/workflows/ci.yaml", ".github/release-drafter.yml"}},
	}
	for _, c := range cases {
		want := map[string]bool{}
		for _, p := range c.want {
			want[p] = true
		}
		m := New(c.pattern)
		for _, p := range paths {
			if got := m.Match(p); got != want[p] {
				t.Errorf("pattern %q, path %q: got %v, want %v", c.pattern, p, got, want[p])
			}
		}
	}
}

func TestNegationCannotReincludeButFiltersFiles(t *testing.T) {
	m := New("*.md", "!README.md")
	if m.Match("README.md") {
		t.Error("README.md should be re-included by !README.md")
	}
	if !m.Match("docs/x.md") {
		t.Error("docs/x.md should match *.md")
	}
}

func TestSkipsCommentsAndBlankPatterns(t *testing.T) {
	m := New("", "   ", "# comment", "trailing\\")
	if m.Match("comment") || m.Match("trailing") {
		t.Error("comments, blank and invalid patterns must not match")
	}
}

func TestMatchAny(t *testing.T) {
	m := New("docs/")
	if !m.MatchAny([]string{"main.go", "docs/readme.md"}) {
		t.Error("MatchAny should find docs/readme.md")
	}
	if m.MatchAny(nil) {
		t.Error("MatchAny(nil) must be false")
	}
}

// TestRuleRegexHasMatchTimeout proves every compiled rule carries a bounded
// MatchTimeout, per the global constraint that every regexp2 match has a
// timeout. Without it, a rule's regex keeps regexp2's own default (no
// timeout at all), which lets a pathological include-paths/exclude-paths/
// when.paths/files pattern run unbounded.
func TestRuleRegexHasMatchTimeout(t *testing.T) {
	m := New("*.md")
	if len(m.rules) != 1 {
		t.Fatalf("expected 1 compiled rule, got %d", len(m.rules))
	}
	if got := m.rules[0].re.MatchTimeout; got != regex.DefaultTimeout {
		t.Errorf("rule.re.MatchTimeout = %s, want %s", got, regex.DefaultTimeout)
	}
}

// TestMatchTreatsTimeoutAsNoMatch proves a match that times out is handled,
// not left to panic or hang: it comes back as "no match" within the
// MatchTimeout window. ^(a+)+$ against 40 a's followed by a non-matching
// character is the same pathological pattern internal/regex uses to prove
// a timeout: catastrophic backtracking reliably exceeds a short
// MatchTimeout.
func TestMatchTreatsTimeoutAsNoMatch(t *testing.T) {
	re := regexp2.MustCompile(`^(a+)+$`, regexp2.ECMAScript)
	re.MatchTimeout = 50 * time.Millisecond
	m := &Matcher{cache: map[string]result{}, rules: []rule{{re: re, pattern: "^(a+)+$"}}}
	start := time.Now()
	if m.Match(strings.Repeat("a", 40) + "!") {
		t.Error("a match that times out must be treated as no match")
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("timeout took %s; should have failed fast", time.Since(start))
	}
}
