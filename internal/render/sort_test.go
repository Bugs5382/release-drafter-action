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

	"github.com/Bugs5382/release-drafter-action/internal/model"
)

func TestSortPullRequests(t *testing.T) {
	s := func(v string) *string { return &v }
	prs := []model.PullRequest{
		{Number: 1, Title: "b", MergedAt: s("2026-01-02")},
		{Number: 2, Title: "a", MergedAt: nil},
		{Number: 3, Title: "c", MergedAt: s("2026-01-02")},
		{Number: 4, Title: "B", MergedAt: s("2026-01-01")},
	}
	order := func(ps []model.PullRequest) []int {
		var out []int
		for _, p := range ps {
			out = append(out, p.Number)
		}
		return out
	}
	cases := []struct {
		by, dir string
		want    []int
	}{
		{"merged_at", "descending", []int{2, 1, 3, 4}},
		{"merged_at", "ascending", []int{4, 1, 3, 2}},
		{"title", "ascending", []int{4, 2, 1, 3}},
		{"title", "descending", []int{3, 1, 2, 4}},
	}
	for _, c := range cases {
		got := order(SortPullRequests(prs, c.by, c.dir))
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s %s = %v; want %v", c.by, c.dir, got, c.want)
				break
			}
		}
	}
	if prs[0].Number != 1 {
		t.Error("SortPullRequests must not reorder its input")
	}
}

// TestSortPullRequestsTitleUTF16 pins that title sorting compares UTF-16
// code units the way JavaScript's default string comparison does, not Go's
// UTF-8 bytes. rune(0x1F680) (a supplementary-plane character) encodes as
// the UTF-16 surrogate pair 0xD83D 0xDE80; rune(0xFF21) is the single BMP
// code unit 0xFF21. Since 0xD83D < 0xFF21, JS sorts the 0x1F680 title
// first ascending, even though its UTF-8 bytes (starting 0xF0) sort after
// the 0xFF21 title's UTF-8 bytes (starting 0xEF).
func TestSortPullRequestsTitleUTF16(t *testing.T) {
	supplementary := string(rune(0x1F680))
	bmp := string(rune(0xFF21))
	prs := []model.PullRequest{
		{Number: 1, Title: bmp},
		{Number: 2, Title: supplementary},
	}
	got := SortPullRequests(prs, "title", "ascending")
	if got[0].Number != 2 || got[1].Number != 1 {
		t.Errorf("ascending title order = [%d %d]; want [2 1]", got[0].Number, got[1].Number)
	}
	gotDesc := SortPullRequests(prs, "title", "descending")
	if gotDesc[0].Number != 1 || gotDesc[1].Number != 2 {
		t.Errorf("descending title order = [%d %d]; want [1 2]", gotDesc[0].Number, gotDesc[1].Number)
	}
}
