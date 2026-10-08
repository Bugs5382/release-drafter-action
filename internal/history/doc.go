// Package history finds the releases a draft cares about, works out the
// commit range between them, and collects the pull requests merged in that
// range.
//
// FindPreviousReleases filters and sorts a repository's releases by
// tag-prefix, filter-by-commitish, prerelease, include-pre-releases and
// filter-by-range, the way v7 does, and returns both the last published
// release (the range's start) and any existing draft release that matches
// (the one to update instead of creating a new one).
//
// The range itself is the commits GitHub compares between the last
// release's tag and the target commitish. On a first release, with no
// earlier published release to compare against, it instead walks the whole
// history reachable from the target commit, trimmed by the since input
// (tag, SHA or date) when set; this is this action's own addition; v7 has
// no first-release support at all (release-drafter/release-drafter#1630).
//
// Pull requests are collected from the commits' associated pull requests,
// plus a recovery pass for very recently merged pull requests GitHub's
// index has not caught up with yet. They are not filtered by label or path
// here: deprecated exclude-labels/include-labels/exclude-paths/include-paths
// are migrated to pre-exclude/pre-include categories by internal/config,
// and internal/render drops non-matching pull requests before rendering.
package history

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
