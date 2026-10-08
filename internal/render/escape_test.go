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

import "testing"

// Expected values come from v7.7.0's escapeTitle.
func TestEscapeTitleMatchesV7(t *testing.T) {
	cases := []struct{ title, escapes, want string }{
		{"a_b *c* `d_e` @x #1", "_*@#", "a\\_b \\*c\\* `d_e` @<!---->x #<!---->1"},
		{"a_b `d_e`", "_`", "a\\_b \\`d\\_e\\`"},
		{"`unclosed _ tick", "_", "`unclosed \\_ tick"},
		{"plain", "", "plain"},
		{"back\\slash <tag> & amp", "\\<&", "back\\\\slash \\<tag> \\& amp"},
		{"`a`b`c`", "`", "\\`a\\`b\\`c\\`"},
		{"dash-and]bracket^", "-]^", "dash\\-and\\]bracket\\^"},
	}
	for _, c := range cases {
		if got := EscapeTitle(c.title, c.escapes); got != c.want {
			t.Errorf("EscapeTitle(%q, %q) = %q; want %q", c.title, c.escapes, got, c.want)
		}
	}
}
