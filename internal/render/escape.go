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

import "strings"

// JavaScript treats these as line terminators too.
const (
	lineSeparator      = rune(0x2028)
	paragraphSeparator = rune(0x2029)
	byteOrderMark      = rune(0xFEFF)
)

func isLineTerminator(r rune) bool {
	return r == '\n' || r == '\r' || r == lineSeparator || r == paragraphSeparator
}

// EscapeTitle escapes the characters listed in change-title-escapes so they
// are not read as Markdown. An @ or # gets an HTML comment after it instead
// of a backslash, so GitHub does not turn it into a mention or a reference.
// A backtick span is copied as is, unless the backtick itself is listed.
func EscapeTitle(title, escapes string) string {
	set := map[rune]bool{}
	for _, r := range escapes {
		set[r] = true
	}
	runes := []rune(title)
	var b strings.Builder
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if set[r] {
			if r == '@' || r == '#' {
				b.WriteString(string(r) + "<!---->")
			} else {
				b.WriteString(`\` + string(r))
			}
			continue
		}
		if r == '`' {
			end := -1
			for j := i + 1; j < len(runes); j++ {
				if isLineTerminator(runes[j]) {
					break
				}
				if runes[j] == '`' {
					end = j
					break
				}
			}
			if end > 0 {
				b.WriteString(string(runes[i : end+1]))
				i = end
				continue
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}
