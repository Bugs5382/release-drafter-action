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

	"github.com/dlclark/regexp2"

	"github.com/Bugs5382/release-drafter-action/internal/logging"
	"github.com/Bugs5382/release-drafter-action/internal/regex"
)

type replacer struct {
	re     *regexp2.Regexp
	global bool
	fn     func(m regexp2.Match, input, pattern string) (string, error)
}

// mustRe compiles p and gives it the same bounded MatchTimeout every
// pattern-derived regex in this package gets, so a pathological glob cannot
// hang a match indefinitely.
func mustRe(p string) *regexp2.Regexp {
	re := regexp2.MustCompile(p, regexp2.ECMAScript)
	re.MatchTimeout = regex.DefaultTimeout
	return re
}

func group(m regexp2.Match, i int) string {
	g := m.GroupByNumber(i)
	if g == nil || len(g.Captures) == 0 {
		return ""
	}
	return g.String()
}

var (
	reBlankLine          = mustRe(`^\s+$`)
	reInvalidTrailing    = mustRe(`(?:[^\\]|^)\\$`)
	reLeadingEscapedBang = mustRe(`^\\!`)
	reLeadingEscapedHash = mustRe(`^\\#`)
	reRange              = mustRe(`([0-z])-([0-z])`)
	reSlashNotLast       = mustRe(`\/(?!$)`)
	reTrailingWildcard   = mustRe(`(^|\\\/)?\\\*$`)
	reEscapedStar        = mustRe(`\\\*`)
)

func sanitizeRange(r string) (string, error) {
	return reRange.ReplaceFunc(r, func(m regexp2.Match) string {
		from, to := group(m, 1), group(m, 2)
		if from[0] <= to[0] {
			return m.String()
		}
		return ""
	}, -1, -1)
}

func negateRange(r string) string {
	switch {
	case strings.HasPrefix(r, "!"):
		return "^" + r[1:]
	case strings.HasPrefix(r, `\^`):
		return "^" + r[2:]
	}
	return r
}

func cleanRangeBackSlash(s string) string {
	return s[:len(s)-len(s)%2]
}

// replacers is node-ignore's REPLACERS table, in order.
var replacers = []replacer{
	{mustRe("^" + string(rune(0xFEFF))), false, func(regexp2.Match, string, string) (string, error) { return "", nil }},
	{mustRe(`((?:\\\\)*?)(\\?\s+)$`), false, func(m regexp2.Match, _, _ string) (string, error) {
		if strings.HasPrefix(group(m, 2), `\`) {
			return group(m, 1) + " ", nil
		}
		return group(m, 1), nil
	}},
	{mustRe(`(\\+?)\s`), true, func(m regexp2.Match, _, _ string) (string, error) {
		s := group(m, 1)
		return s[:len(s)-len(s)%2] + " ", nil
	}},
	{mustRe(`[\\$.|*+(){^]`), true, func(m regexp2.Match, _, _ string) (string, error) { return `\` + m.String(), nil }},
	{mustRe(`(?!\\)\?`), true, func(regexp2.Match, string, string) (string, error) { return `[^/]`, nil }},
	{mustRe(`^\/`), false, func(regexp2.Match, string, string) (string, error) { return "^", nil }},
	{mustRe(`\/`), true, func(regexp2.Match, string, string) (string, error) { return `\/`, nil }},
	{mustRe(`^\^*(?:\\\*\\\*\\\/)+`), false, func(regexp2.Match, string, string) (string, error) { return `^(?:.*\/)?`, nil }},
	{mustRe(`^(?=[^^])`), false, func(_ regexp2.Match, _, pattern string) (string, error) {
		ok, err := reSlashNotLast.MatchString(pattern)
		if err != nil {
			return "", err
		}
		if !ok {
			return `(?:^|\/)`, nil
		}
		return "^", nil
	}},
	{mustRe(`\\\/\\\*\\\*(?=\\\/|$)`), true, func(m regexp2.Match, input, _ string) (string, error) {
		if m.Index+6 < len([]rune(input)) {
			return `(?:\/[^\/]+)*`, nil
		}
		return `\/.+`, nil
	}},
	{mustRe(`(^|[^\\]+)(\\\*)+(?=.+)`), true, func(m regexp2.Match, _, _ string) (string, error) {
		unescaped, err := reEscapedStar.Replace(group(m, 2), `[^\/]*`, -1, -1)
		if err != nil {
			return "", err
		}
		return group(m, 1) + unescaped, nil
	}},
	{mustRe(`\\\\\\(?=[$.|*+(){^])`), true, func(regexp2.Match, string, string) (string, error) { return `\`, nil }},
	{mustRe(`\\\\`), true, func(regexp2.Match, string, string) (string, error) { return `\`, nil }},
	{mustRe(`(\\)?\[([^\]/]*?)(\\*)($|\])`), true, func(m regexp2.Match, _, _ string) (string, error) {
		lead, rng, endEsc, closing := group(m, 1), group(m, 2), group(m, 3), group(m, 4)
		switch {
		case lead == `\`:
			return `\[` + rng + cleanRangeBackSlash(endEsc) + closing, nil
		case closing == "]" && len(endEsc)%2 == 0:
			sanitized, err := sanitizeRange(rng)
			if err != nil {
				return "", err
			}
			return "[" + negateRange(sanitized) + endEsc + "]", nil
		}
		return "[]", nil
	}},
	{mustRe(`(?:[^*])$`), false, func(m regexp2.Match, _, _ string) (string, error) {
		if strings.HasSuffix(m.String(), "/") {
			return m.String() + "$", nil
		}
		return m.String() + `(?=$|\/$)`, nil
	}},
}

// makeRegexPrefix runs pattern through every replacer in order, exactly as
// node-ignore's REPLACERS.reduce does. A replacer's own regexp2 call (the
// outer ReplaceFunc, or a nested one such as sanitizeRange) can time out;
// either is surfaced as an error rather than silently kept as the
// unreplaced input, so the caller can log the pattern and skip it.
func makeRegexPrefix(pattern string) (string, error) {
	cur := pattern
	for _, r := range replacers {
		count := 1
		if r.global {
			count = -1
		}
		input := cur
		var fnErr error
		out, err := r.re.ReplaceFunc(input, func(m regexp2.Match) string {
			s, ferr := r.fn(m, input, pattern)
			if ferr != nil {
				fnErr = ferr
			}
			return s
		}, -1, count)
		if err != nil {
			return "", err
		}
		if fnErr != nil {
			return "", fnErr
		}
		cur = out
	}
	return cur, nil
}

type rule struct {
	negative bool
	re       *regexp2.Regexp
	// pattern is the original glob, kept only to name it in a warning log
	// if a match against this rule times out later.
	pattern string
}

// Matcher tests paths against a list of gitignore-style patterns.
type Matcher struct {
	rules []rule
	cache map[string]result
}

type result struct{ ignored, unignored bool }

// New builds a matcher from patterns, skipping blank lines, comments and
// patterns with an invalid trailing backslash, exactly as node-ignore does.
// Patterns that still fail to compile are skipped.
func New(patterns ...string) *Matcher {
	m := &Matcher{cache: map[string]result{}}
	for _, p := range patterns {
		m.add(p)
	}
	return m
}

// skipPattern logs why pattern was not added as a rule. A pattern that
// fails to compile or rewrite is not a matcher bug, so it is logged and
// skipped rather than propagated as an error from New, keeping New's
// signature exactly as the brief specifies; the skipped pattern simply
// never matches anything.
func skipPattern(pattern string, err error, msg string) {
	logging.L().Warn().Str("pattern", pattern).Err(err).Msg("pathmatch: " + msg)
}

func (m *Matcher) add(pattern string) {
	if pattern == "" || strings.HasPrefix(pattern, "#") {
		return
	}
	blank, err := reBlankLine.MatchString(pattern)
	if err != nil {
		skipPattern(pattern, err, "blank-line check timed out, skipping pattern")
		return
	}
	if blank {
		return
	}
	invalidTrailing, err := reInvalidTrailing.MatchString(pattern)
	if err != nil {
		skipPattern(pattern, err, "trailing-backslash check timed out, skipping pattern")
		return
	}
	if invalidTrailing {
		return
	}
	body := pattern
	negative := false
	if strings.HasPrefix(body, "!") {
		negative = true
		body = body[1:]
	}
	body, err = reLeadingEscapedBang.Replace(body, "!", -1, 1)
	if err != nil {
		skipPattern(pattern, err, "leading-bang rewrite timed out, skipping pattern")
		return
	}
	body, err = reLeadingEscapedHash.Replace(body, "#", -1, 1)
	if err != nil {
		skipPattern(pattern, err, "leading-hash rewrite timed out, skipping pattern")
		return
	}
	prefix, err := makeRegexPrefix(body)
	if err != nil {
		skipPattern(pattern, err, "pattern rewrite timed out, skipping pattern")
		return
	}
	src, err := reTrailingWildcard.ReplaceFunc(prefix, func(mm regexp2.Match) string {
		if p1 := group(mm, 1); p1 != "" {
			return p1 + `[^/]+(?=$|\/$)`
		}
		return `[^/]*(?=$|\/$)`
	}, -1, 1)
	if err != nil {
		skipPattern(pattern, err, "trailing-wildcard rewrite timed out, skipping pattern")
		return
	}
	re, err := regexp2.Compile(src, regexp2.ECMAScript|regexp2.IgnoreCase)
	if err != nil {
		skipPattern(pattern, err, "pattern did not compile, skipping pattern")
		return
	}
	re.MatchTimeout = regex.DefaultTimeout
	m.rules = append(m.rules, rule{negative: negative, re: re, pattern: pattern})
	logging.L().Debug().Str("pattern", pattern).Msg("pathmatch: added rule")
}

// Match reports whether path is matched ("ignored", in node-ignore terms).
// A path under a matched directory is matched too, and a negated pattern
// cannot re-include it.
func (m *Matcher) Match(path string) bool {
	if path == "" || len(m.rules) == 0 {
		return false
	}
	return m.t(path, nil).ignored
}

func (m *Matcher) t(path string, slices []string) result {
	if r, ok := m.cache[path]; ok {
		return r
	}
	if slices == nil {
		for _, s := range strings.Split(path, "/") {
			if s != "" {
				slices = append(slices, s)
			}
		}
	}
	slices = slices[:len(slices)-1]
	if len(slices) == 0 {
		r := m.test(path)
		m.cache[path] = r
		return r
	}
	parent := m.t(strings.Join(slices, "/")+"/", slices)
	if parent.ignored {
		m.cache[path] = parent
		return parent
	}
	r := m.test(path)
	m.cache[path] = r
	return r
}

func (m *Matcher) test(path string) result {
	ignored, unignored := false, false
	for _, r := range m.rules {
		if (unignored == r.negative && ignored != unignored) || (r.negative && !ignored && !unignored) {
			continue
		}
		ok, err := r.re.MatchString(path)
		if err != nil {
			logging.L().Warn().Str("pattern", r.pattern).Str("path", path).Err(err).
				Msg("pathmatch: match timed out, treating as no match")
			continue
		}
		if !ok {
			continue
		}
		ignored = !r.negative
		unignored = r.negative
	}
	return result{ignored: ignored, unignored: unignored}
}

// MatchAny reports whether any of paths is matched.
func (m *Matcher) MatchAny(paths []string) bool {
	for _, p := range paths {
		if m.Match(p) {
			return true
		}
	}
	return false
}
