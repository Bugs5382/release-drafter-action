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
	"regexp"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
)

// DefaultTimeout bounds every match so a pathological pattern cannot hang
// the action.
const DefaultTimeout = time.Second

// literalCheck is v7's test for "this string is a /pattern/flags literal".
// Note that it does not accept the y flag, so /x/y is a plain string.
var literalCheck = regexp.MustCompile(`^/.+/[AJUXgimsux]*$`)

// Regex is a compiled release-drafter pattern.
type Regex struct {
	// Raw is the string exactly as it appeared in the config.
	Raw string
	// Source is the pattern handed to the engine.
	Source string
	// Flags holds the JavaScript flags kept by regex-parser, in first-seen
	// order, filtered to g, i, m, s, u and y.
	Flags string
	// Global is true when the g flag is set; replacers then replace every
	// match instead of only the first.
	Global bool
	// Literal is true when Raw was not a /pattern/flags literal and is
	// matched as an escaped plain string (always global).
	Literal bool
	re      *regexp2.Regexp
}

// ErrTimeout wraps a match that ran past the timeout.
var ErrTimeout = errors.New("regex match timed out")

// CompileError is a pattern the engine rejected.
type CompileError struct {
	Pattern string
	Err     error
}

func (e *CompileError) Error() string {
	return fmt.Sprintf("pattern %q does not compile: %v", e.Pattern, e.Err)
}

func (e *CompileError) Unwrap() error { return e.Err }

// Parse compiles s. A /pattern/flags literal becomes a regex; anything else
// is escaped and matched literally with the g flag, as v7 does.
func Parse(s string, timeout time.Duration) (*Regex, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	r := &Regex{Raw: s}
	if literalCheck.MatchString(s) {
		last := strings.LastIndex(s, "/")
		r.Source = s[1:last]
		seen := map[rune]bool{}
		for _, f := range s[last+1:] {
			if strings.ContainsRune("gimsuy", f) && !seen[f] {
				seen[f] = true
				r.Flags += string(f)
			}
		}
	} else {
		r.Literal = true
		r.Source = regexp2.Escape(s)
		r.Flags = "g"
	}
	opts := regexp2.RegexOptions(regexp2.ECMAScript)
	for _, f := range r.Flags {
		switch f {
		case 'g':
			r.Global = true
		case 'i':
			opts |= regexp2.IgnoreCase
		case 'm':
			opts |= regexp2.Multiline
		case 's':
			opts |= regexp2.Singleline
		case 'u':
			opts |= regexp2.Unicode
		}
	}
	re, err := regexp2.Compile(r.Source, opts)
	if err != nil {
		return nil, &CompileError{Pattern: s, Err: err}
	}
	re.MatchTimeout = timeout
	r.re = re
	return r, nil
}

// timeoutErr reports a match timeout by pattern and limit. regexp2's own
// error quotes the whole input (a release or pull request body), which
// would land in the ::error annotation, so it is left out.
func (r *Regex) timeoutErr() error {
	return fmt.Errorf("%w: pattern %q timed out after %s", ErrTimeout, r.Raw, r.re.MatchTimeout)
}

// MatchString reports whether the pattern matches anywhere in s, like
// RegExp.prototype.test. A timeout is returned as an error wrapping
// ErrTimeout.
func (r *Regex) MatchString(s string) (bool, error) {
	ok, err := r.re.MatchString(s)
	if err != nil {
		return false, r.timeoutErr()
	}
	return ok, nil
}

// Replace replaces the first match, or every match when the pattern is
// global, with the string fn returns. fn receives the whole match followed
// by every numbered group; a group that did not take part is "".
//
// The loop is driven by hand, rather than delegating to regexp2's
// ReplaceFunc: regexp2 v1.12.0's internal replace() discards both the
// buffer and the error when FindNextMatch fails on the second or later
// match, returning ("", nil) as if the replace had succeeded on an empty
// string. Checking the error ourselves at every step keeps a timeout past
// the first match reachable through ErrTimeout instead of swallowed.
func (r *Regex) Replace(input string, fn func(groups []string) string) (string, error) {
	m, err := r.re.FindStringMatch(input)
	if err != nil {
		return "", r.timeoutErr()
	}
	if m == nil {
		return input, nil
	}
	runes := []rune(input)
	out := make([]rune, 0, len(runes))
	prev := 0
	for m != nil {
		if m.Index != prev {
			out = append(out, runes[prev:m.Index]...)
		}
		prev = m.Index + m.Length
		out = append(out, []rune(fn(matchGroups(m)))...)
		if !r.Global {
			break
		}
		m, err = r.re.FindNextMatch(m)
		if err != nil {
			return "", r.timeoutErr()
		}
	}
	if prev < len(runes) {
		out = append(out, runes[prev:]...)
	}
	return string(out), nil
}

// matchGroups returns the whole match followed by every numbered group, in
// the shape Regex.Replace's callback expects: a group that did not take
// part in the match comes back as "".
func matchGroups(m *regexp2.Match) []string {
	gs := m.Groups()
	groups := make([]string, len(gs))
	for i, g := range gs {
		if len(g.Captures) > 0 {
			groups[i] = g.String()
		}
	}
	return groups
}
