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
	"regexp"
	"strings"
)

// The header patterns v7 hands to conventional-commits-parser (taken from
// the conventionalcommits preset).
var (
	headerRe         = regexp.MustCompile(`^(\w*)(?:\((.*)\))?!?: (.*)$`)
	breakingHeaderRe = regexp.MustCompile(`^(\w*)(?:\((.*)\))?!: (.*)$`)
	notesRe          = regexp.MustCompile(`(?i)^[\s|*]*(BREAKING CHANGE|BREAKING-CHANGE)[:\s]+(.*)`)
	lineSplitRe      = regexp.MustCompile(`\r?\n`)
)

// ConventionalTitle is what v7 reads from a conventional commit title.
type ConventionalTitle struct {
	Type     string
	Scope    string
	HasScope bool
	Breaking bool
}

// ParseConventionalTitle parses a pull request title the way v7's
// category matching does. ok is false when the title is not conventional.
//
// This is a pure hot predicate: no logging here by design (see the render
// package logging ruling), since it runs once per pull request per
// condition and would drown out everything else.
func ParseConventionalTitle(title string) (ConventionalTitle, bool) {
	if strings.TrimSpace(title) == "" {
		return ConventionalTitle{}, false
	}
	lines := lineSplitRe.Split(strings.Trim(title, "\r\n"), -1)
	header := lines[0]
	// conventional-commits-parser 6.4.0's parseHeader matches
	// breakingHeaderPattern first and sources type and scope from that
	// match; only when it does not match does it fall back to
	// headerPattern (dist/CommitParser.js, parseHeader).
	m := breakingHeaderRe.FindStringSubmatch(header)
	if m == nil {
		m = headerRe.FindStringSubmatch(header)
	}
	if m == nil || m[1] == "" {
		return ConventionalTitle{}, false
	}
	ct := ConventionalTitle{Type: m[1], Scope: m[2], HasScope: m[2] != ""}
	for _, line := range lines[1:] {
		if notesRe.MatchString(line) {
			ct.Breaking = true
		}
	}
	if !ct.Breaking && breakingHeaderRe.MatchString(header) {
		ct.Breaking = true
	}
	return ct, true
}
