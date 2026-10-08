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
	"strconv"
	"strings"
)

// ReplacePattern is a parsed replacer replace string. v7 ported this parser
// from VS Code: it understands \n, \t, \\, the case operators \u \U \l \L \E,
// $$, $& and $0, and $1 to $99. Everything else is literal.
type ReplacePattern struct {
	pieces []piece
}

type piece struct {
	static  string
	isMatch bool
	index   int
	caseOps []byte
}

// ParseReplacePattern parses a replace string.
func ParseReplacePattern(s string) ReplacePattern {
	var (
		pieces  []piece
		current strings.Builder
		caseOps []byte
		last    int
	)
	emitUnchanged := func(to int) {
		current.WriteString(s[last:to])
		last = to
	}
	emitStatic := func(v string, to int) {
		current.WriteString(v)
		last = to
	}
	emitMatch := func(index, to int) {
		if current.Len() > 0 {
			pieces = append(pieces, piece{static: current.String()})
			current.Reset()
		}
		pieces = append(pieces, piece{isMatch: true, index: index, caseOps: append([]byte(nil), caseOps...)})
		caseOps = caseOps[:0]
		last = to
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
			if i >= len(s) {
				break
			}
			switch s[i] {
			case '\\':
				emitUnchanged(i - 1)
				emitStatic(`\`, i+1)
			case 'n':
				emitUnchanged(i - 1)
				emitStatic("\n", i+1)
			case 't':
				emitUnchanged(i - 1)
				emitStatic("\t", i+1)
			case 'u', 'U', 'l', 'L', 'E':
				emitUnchanged(i - 1)
				emitStatic("", i+1)
				caseOps = append(caseOps, s[i])
			}
		case '$':
			i++
			if i >= len(s) {
				break
			}
			next := s[i]
			switch {
			case next == '$':
				emitUnchanged(i - 1)
				emitStatic("$", i+1)
			case next == '0' || next == '&':
				emitUnchanged(i - 1)
				emitMatch(0, i+1)
			case next >= '1' && next <= '9':
				index := int(next - '0')
				if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
					i++
					index = index*10 + int(s[i]-'0')
					emitUnchanged(i - 2)
					emitMatch(index, i+1)
					continue
				}
				emitUnchanged(i - 1)
				emitMatch(index, i+1)
			}
		}
	}
	emitUnchanged(len(s))
	if current.Len() > 0 {
		pieces = append(pieces, piece{static: current.String()})
	}
	return ReplacePattern{pieces: pieces}
}

// Build renders the pattern for one match. matches[0] is the whole match,
// then one entry per numbered group.
func (p ReplacePattern) Build(matches []string) string {
	var b strings.Builder
	for _, pc := range p.pieces {
		if !pc.isMatch {
			b.WriteString(pc.static)
			continue
		}
		b.WriteString(applyCaseOps(substitute(pc.index, matches), pc.caseOps))
	}
	return b.String()
}

func substitute(index int, matches []string) string {
	if index == 0 {
		return matches[0]
	}
	remainder := ""
	for index > 0 {
		if index < len(matches) {
			return matches[index] + remainder
		}
		remainder = strconv.Itoa(index%10) + remainder
		index /= 10
	}
	return "$" + remainder
}

func applyCaseOps(match string, ops []byte) string {
	if len(ops) == 0 {
		return match
	}
	runes := []rune(match)
	var out strings.Builder
	op := 0
	for i := 0; i < len(runes); i++ {
		if op >= len(ops) {
			out.WriteString(string(runes[i:]))
			break
		}
		switch ops[op] {
		case 'U':
			out.WriteString(strings.ToUpper(string(runes[i])))
		case 'u':
			out.WriteString(strings.ToUpper(string(runes[i])))
			op++
		case 'L':
			out.WriteString(strings.ToLower(string(runes[i])))
		case 'l':
			out.WriteString(strings.ToLower(string(runes[i])))
			op++
		case 'E':
			out.WriteString(string(runes[i:]))
			i = len(runes)
		default:
			out.WriteRune(runes[i])
		}
	}
	return out.String()
}
