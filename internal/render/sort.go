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
	"sort"
	"unicode/utf16"

	"github.com/Bugs5382/release-drafter-action/internal/model"
)

// compareUTF16 compares a and b the way JavaScript's default string
// comparison does: lexicographically by UTF-16 code unit, not by Go's
// UTF-8 bytes. They differ when one string has a rune above U+FFFF
// (encoded in UTF-16 as a surrogate pair starting at 0xD800-0xDBFF, which
// sorts before the BMP range 0xE000-0xFFFF) but their UTF-8 byte order
// disagrees. Returns -1, 0 or 1.
func compareUTF16(a, b string) int {
	au := utf16.Encode([]rune(a))
	bu := utf16.Encode([]rune(b))
	for i := 0; i < len(au) && i < len(bu); i++ {
		switch {
		case au[i] < bu[i]:
			return -1
		case au[i] > bu[i]:
			return 1
		}
	}
	switch {
	case len(au) < len(bu):
		return -1
	case len(au) > len(bu):
		return 1
	}
	return 0
}

// SortPullRequests returns a sorted copy, by merged_at or title, ascending
// or descending. Like v7, a missing value sorts last when ascending and
// first when descending, and ties keep their order. Title comparisons use
// UTF-16 code units, matching JavaScript's default string comparison
// (merged_at values are ASCII timestamps, so byte and code-unit order
// agree there).
func SortPullRequests(prs []model.PullRequest, sortBy, direction string) []model.PullRequest {
	out := append([]model.PullRequest(nil), prs...)
	byTitle := sortBy == "title"
	field := func(p model.PullRequest) *string {
		if byTitle {
			t := p.Title
			return &t
		}
		return p.MergedAt
	}
	descending := direction != "ascending"
	less := func(a, b *string) bool {
		switch {
		case a == nil && b == nil:
			return false
		case a == nil:
			return descending
		case b == nil:
			return !descending
		}
		c := 0
		if byTitle {
			c = compareUTF16(*a, *b)
		} else if *a < *b {
			c = -1
		} else if *a > *b {
			c = 1
		}
		if descending {
			return c > 0
		}
		return c < 0
	}
	sort.SliceStable(out, func(i, j int) bool { return less(field(out[i]), field(out[j])) })
	return out
}
