// Package render turns a set of pull requests into release notes.
//
// It holds pure functions only: categories (including multi-label categories and
// duplicate listing, as v7 does), change-template, change-title-escapes,
// replacers, collapse-after, no-changes-template, category-template, header and
// footer, every $VARIABLE substitution, version-resolver and version-template.
// Output must match release-drafter v7 byte for byte, except on a first release,
// which gets no warning banner and $RESOLVED_VERSION pinned to first-version
// (default 1.0.0) instead of v7's 0.0.0-plus-increment baseline.
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
