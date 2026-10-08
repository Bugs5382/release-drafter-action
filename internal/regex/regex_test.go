package regex

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
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseMatchesRegexParser(t *testing.T) {
	// Source and flags as regex-parser 2.3.1 produces them behind v7's
	// literal check; plain strings are escaped with the g flag.
	cases := []struct {
		in      string
		literal bool
		flags   string
	}{
		{`/^feat(\(.+\))?:/`, false, ""},
		{`/BREAKING[ -]CHANGE/`, false, ""},
		{`/a/b/gi`, false, "gi"},
		{`/x/y`, true, "g"},
		{`/x/X`, false, ""},
		{`/foo/Ugi`, false, "gi"},
		{`foo`, true, "g"},
		{`/foo`, true, "g"},
		{`a.b`, true, "g"},
	}
	for _, c := range cases {
		r, err := Parse(c.in, 0)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.in, err)
		}
		if r.Literal != c.literal || r.Flags != c.flags {
			t.Errorf("Parse(%q) literal=%v flags=%q; want literal=%v flags=%q", c.in, r.Literal, r.Flags, c.literal, c.flags)
		}
	}
}

func TestLastSlashSplitsPatternFromFlags(t *testing.T) {
	r, err := Parse(`/a/b/gi`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if r.Source != "a/b" {
		t.Errorf("Source = %q; want a/b", r.Source)
	}
	if ok, _ := r.MatchString("xA/By"); !ok {
		t.Error("i flag should make the match case-insensitive")
	}
}

func TestHubAutolabelerPatterns(t *testing.T) {
	cases := []struct {
		pattern, title string
		want           bool
	}{
		{`/^[a-z]+(\(.+\))?!:/`, "feat(api)!: drop v1", true},
		{`/^[a-z]+(\(.+\))?!:/`, "feat(api): add v2", false},
		{`/BREAKING[ -]CHANGE/`, "fix: BREAKING-CHANGE in parser", true},
		{`/^feat(\(.+\))?:/`, "feat: add cache", true},
		{`/^chore\(deps.*\):/`, "chore(deps): bump x", true},
		{`/^chore(?!\(deps)(\(.+\))?:/`, "chore(deps): bump x", false},
		{`/^chore(?!\(deps)(\(.+\))?:/`, "chore: tidy", true},
		{`/^chore(?!\(deps)(\(.+\))?:/`, "chore(repo): scaffold", true},
		{`/^(ci|test|style)(\(.+\))?:/`, "ci: pin actions", true},
	}
	for _, c := range cases {
		r, err := Parse(c.pattern, 0)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.pattern, err)
		}
		got, err := r.MatchString(c.title)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("%s on %q = %v; want %v", c.pattern, c.title, got, c.want)
		}
	}
}

func TestSinglelineAndBackreference(t *testing.T) {
	r, err := Parse(`/a.b/s`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := r.MatchString("a\nb"); !ok {
		t.Error("s flag should let . match a newline")
	}
	br, err := Parse(`/(\w)\1/`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := br.MatchString("book"); !ok {
		t.Error("backreference should match oo")
	}
}

func TestLookbehind(t *testing.T) {
	positive, err := Parse(`/(?<=v)\d+/`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := positive.MatchString("v2"); !ok {
		t.Error("positive lookbehind should match the digits after v")
	}
	if ok, _ := positive.MatchString("x2"); ok {
		t.Error("positive lookbehind should not match without a preceding v")
	}
	negative, err := Parse(`/(?<!v)\d+/`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := negative.MatchString("x2"); !ok {
		t.Error("negative lookbehind should match digits not preceded by v")
	}
	if ok, _ := negative.MatchString("v2"); ok {
		t.Error("negative lookbehind should not match digits preceded by v")
	}
}

func TestPathologicalPatternTimesOut(t *testing.T) {
	r, err := Parse(`/^(a+)+$/`, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = r.MatchString(strings.Repeat("a", 40) + "!")
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v; want ErrTimeout", err)
	}
	if !strings.Contains(err.Error(), `/^(a+)+$/`) {
		t.Errorf("error %q should name the pattern", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("timeout took %s", time.Since(start))
	}
}

// The timeout error goes into an ::error annotation, so it must name the
// pattern and the limit but never carry the text that was being matched
// (a whole release or pull request body).
func TestTimeoutErrorLeavesOutTheInput(t *testing.T) {
	r, err := Parse(`/^(a+)+$/g`, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	input := strings.Repeat("a", 40) + "!SECRETBODY"
	_, matchErr := r.MatchString(input)
	_, replaceErr := r.Replace(input, func([]string) string { return "" })
	for name, err := range map[string]error{"MatchString": matchErr, "Replace": replaceErr} {
		if !errors.Is(err, ErrTimeout) {
			t.Fatalf("%s err = %v; want ErrTimeout", name, err)
		}
		if strings.Contains(err.Error(), "SECRETBODY") || strings.Contains(err.Error(), "aaaa") {
			t.Errorf("%s error carries the input: %q", name, err)
		}
		if want := "pattern \"/^(a+)+$/g\" timed out after 50ms"; !strings.Contains(err.Error(), want) {
			t.Errorf("%s error = %q; want it to contain %q", name, err, want)
		}
	}
}

func TestCompileErrorNamesPattern(t *testing.T) {
	_, err := Parse(`/(unclosed/`, 0)
	if err == nil || !strings.Contains(err.Error(), `/(unclosed/`) {
		t.Fatalf("err = %v; want an error naming the pattern", err)
	}
}

func TestReplaceFirstOrAll(t *testing.T) {
	first, _ := Parse(`/o/`, 0)
	all, _ := Parse(`/o/g`, 0)
	lit, _ := Parse(`o`, 0)
	zero := func([]string) string { return "0" }
	if got, _ := first.Replace("foo boo", zero); got != "f0o boo" {
		t.Errorf("non-global replace = %q", got)
	}
	if got, _ := all.Replace("foo boo", zero); got != "f00 b00" {
		t.Errorf("global replace = %q", got)
	}
	if got, _ := lit.Replace("foo boo", zero); got != "f00 b00" {
		t.Errorf("plain string replace = %q", got)
	}
}

func TestReplaceEmptyMatchesGlobally(t *testing.T) {
	// Pins the existing zero-width-match behaviour (adjacent empty matches
	// between and around real matches) so the manual replace loop keeps
	// producing the same output as regexp2's ReplaceFunc did.
	r, err := Parse(`/x*/g`, 0)
	if err != nil {
		t.Fatal(err)
	}
	i := 0
	got, err := r.Replace("abxxc", func([]string) string {
		i++
		return fmt.Sprintf("[%d]", i)
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := "[1]a[2]b[3][4]c[5]"; got != want {
		t.Errorf("Replace = %q; want %q", got, want)
	}
}

func TestReplaceReportsTimeoutPastFirstMatch(t *testing.T) {
	// zz matches cheaply first; the second alternative is pathological and
	// times out. A global replace must surface that as an error naming the
	// pattern, not silently return an empty string.
	r, err := Parse(`/zz|(a+)+b/g`, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Replace("zz "+strings.Repeat("a", 40)+"!", func([]string) string { return "X" })
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v; want ErrTimeout", err)
	}
	if !strings.Contains(err.Error(), `/zz|(a+)+b/g`) {
		t.Errorf("error %q should name the pattern", err)
	}
}

func TestReplaceGroups(t *testing.T) {
	r, _ := Parse(`/(a)|(b)/g`, 0)
	var seen [][]string
	if _, err := r.Replace("ab", func(g []string) string {
		seen = append(seen, append([]string(nil), g...))
		return ""
	}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0][2] != "" || seen[1][1] != "" || seen[1][2] != "b" {
		t.Errorf("groups = %q", seen)
	}
}

func TestCompileErrorIsTyped(t *testing.T) {
	_, err := Parse(`/[/`, 0)
	var ce *CompileError
	if !errors.As(err, &ce) || ce.Pattern != `/[/` {
		t.Fatalf("err = %#v; want *CompileError", err)
	}
}
