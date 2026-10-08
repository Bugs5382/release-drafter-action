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
	"testing"

	"github.com/Bugs5382/release-drafter-action/internal/config"
	"github.com/Bugs5382/release-drafter-action/internal/regex"
)

func TestRenderLeavesUnknownAndLowercaseTokens(t *testing.T) {
	got := Render("$A $AB $A_B $a $B $C$A. $UNSET ${A}", Vars{"$A": "x", "$AB": "y", "$B": "", "$C": "0"})
	if want := "x y $A_B $a  0x. $UNSET ${A}"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderIsSinglePass(t *testing.T) {
	if got := Render("$A", Vars{"$A": "$B", "$B": "no"}); got != "$B" {
		t.Errorf("values must not be re-scanned, got %q", got)
	}
}

// Expected output from v7.7.0's renderTemplate with the same replacers.
func TestApplyReplacersMatchesV7(t *testing.T) {
	cases := []struct{ search, replace, input, want string }{
		{"/@github-actions\\[bot\\]/g", "[@github-actions](https://github.com/apps/github-actions)", "by @github-actions[bot] and @github-actions[bot]", "by [@github-actions](https://github.com/apps/github-actions) and [@github-actions](https://github.com/apps/github-actions)"},
		{"/(\\w+)@(\\w+)/", "$2 at $1", "a@b c@d", "b at a c@d"},
		{"/(\\w+)@(\\w+)/g", "\\u$1-\\U$2\\E!", "ab@cd", "Ab-CD!"},
		{"/(a)/g", "$$ $& $0 $1 $2 $10 $01", "xa", "x$ a a a $2 a0 a1"},
		{"/x/g", "line\\nnext\\ttab\\\\slash\\q", "x", "line\nnext\ttab\\slash\\q"},
		{"/(h)(e)(l)(l)(o)(w)(o)(r)(l)(d)(!)/", "[$11][$1$0]", "helloworld!", "[!][hhelloworld!]"},
		{"/b/", "trailing\\", "abc", "atrailing\\c"},
		{"/b/", "dollar$", "abc", "adollar$c"},
		{"/(\\w)(\\w)/g", "\\L$1\\l$2", "ABCD", "abcd"},
	}
	for _, c := range cases {
		re, err := regex.Parse(c.search, 0)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ApplyReplacers(c.input, []config.CompiledReplacer{{Search: re, Replace: c.replace}})
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("%s -> %q on %q = %q; want %q", c.search, c.replace, c.input, got, c.want)
		}
	}
}
