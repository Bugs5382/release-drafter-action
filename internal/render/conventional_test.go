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

// Expected values come from conventional-commits-parser 6.4.0 with the
// header patterns v7.7.0 passes to it.
func TestParseConventionalTitleMatchesV7(t *testing.T) {
	cases := []struct {
		title      string
		ok         bool
		typ, scope string
		hasScope   bool
		breaking   bool
	}{
		{"feat: add x", true, "feat", "", false, false},
		{"feat(api)!: drop y", true, "feat", "api", true, true},
		{"fix(ui): z", true, "fix", "ui", true, false},
		{"Add thing", false, "", "", false, false},
		{": empty type", false, "", "", false, false},
		{"feat!: big", true, "feat", "", false, true},
		{"feat(a)(b): odd", true, "feat", "a)(b", true, false},
		{"FEAT: upper", true, "FEAT", "", false, false},
		{"feat:nospace", false, "", "", false, false},
		{"Revert \"feat: x\"", false, "", "", false, false},
		{"feat: x\n\nBREAKING CHANGE: y", true, "feat", "", false, true},
		{"feat: x\nBREAKING-CHANGE: y", true, "feat", "", false, true},
		{"feat: x\nBREAKING CHANGES: y", true, "feat", "", false, false},
		{"  feat: lead", false, "", "", false, false},
		{"feat(): x", true, "feat", "", false, false},
		{"\n\nfeat: after blank", true, "feat", "", false, false},
		{"fëat: x", false, "", "", false, false},
		{"   ", false, "", "", false, false},
		// conventional-commits-parser 6.4.0's parseHeader tries
		// breakingHeaderPattern first and sources type and scope from
		// that match; headerPattern is only a fallback for a header the
		// breaking pattern does not match. Here breakingHeaderRe matches
		// ("a" is a well-formed scope followed by "!: "), so its fields
		// win: type feat, scope "a" (not headerRe's greedy "a)!: x").
		{"feat(a)!: x): y", true, "feat", "a", true, true},
	}
	for _, c := range cases {
		got, ok := ParseConventionalTitle(c.title)
		if ok != c.ok || got.Type != c.typ || got.Scope != c.scope || got.HasScope != c.hasScope || got.Breaking != c.breaking {
			t.Errorf("ParseConventionalTitle(%q) = %+v, %v", c.title, got, ok)
		}
	}
}
