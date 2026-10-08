package semver

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
	"fmt"
	"regexp"
	"strconv"
	"strings"

	msemver "github.com/Masterminds/semver/v3"
)

// compareRe is compare-versions' validation regex (case-insensitive).
var compareRe = regexp.MustCompile(`(?i)^[v^~<>=]*?(\d+)(?:\.([x*]|\d+)(?:\.([x*]|\d+)(?:\.([x*]|\d+))?(?:-([\da-z\-]+(?:\.[\da-z\-]+)*))?(?:\+[\da-z\-]+(?:\.[\da-z\-]+)*)?)?)?$`)

// CompareVersions ports compare-versions' compareVersions. It returns -1, 0
// or 1, and an error when either side is not a version string it accepts
// (release-drafter then falls back to comparing creation dates).
func CompareVersions(a, b string) (int, error) {
	na, err := splitVersion(a)
	if err != nil {
		return 0, err
	}
	nb, err := splitVersion(b)
	if err != nil {
		return 0, err
	}
	pa, pb := na[4], nb[4]
	if r := compareSegments(na[:4], nb[:4]); r != 0 {
		return r, nil
	}
	switch {
	case pa != "" && pb != "":
		return compareSegments(strings.Split(pa, "."), strings.Split(pb, ".")), nil
	case pa != "":
		return -1, nil
	case pb != "":
		return 1, nil
	}
	return 0, nil
}

func splitVersion(v string) ([]string, error) {
	m := compareRe.FindStringSubmatch(v)
	if m == nil {
		return nil, fmt.Errorf("invalid argument not valid semver ('%s' received)", v)
	}
	return m[1:], nil
}

func compareSegments(a, b []string) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		x, y := "0", "0"
		if i < len(a) && a[i] != "" {
			x = a[i]
		}
		if i < len(b) && b[i] != "" {
			y = b[i]
		}
		if r := compareStrings(x, y); r != 0 {
			return r
		}
	}
	return 0
}

func isWildcard(s string) bool { return s == "*" || s == "x" || s == "X" }

// compareStrings mirrors compare-versions: parseInt when both sides are
// numeric, otherwise a string comparison.
func compareStrings(a, b string) int {
	if isWildcard(a) || isWildcard(b) {
		return 0
	}
	an, aErr := leadingInt(a)
	bn, bErr := leadingInt(b)
	if aErr == nil && bErr == nil {
		switch {
		case an > bn:
			return 1
		case an < bn:
			return -1
		}
		return 0
	}
	if aErr == nil {
		a = strconv.FormatInt(an, 10)
	}
	if bErr == nil {
		b = strconv.FormatInt(bn, 10)
	}
	return strings.Compare(a, b)
}

// leadingInt is JavaScript's parseInt(v, 10): it reads leading digits and
// fails when there are none.
func leadingInt(s string) (int64, error) {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, fmt.Errorf("not a number: %q", s)
	}
	return strconv.ParseInt(s[:end], 10, 64)
}

// Range is a compiled filter-by-range expression.
type Range struct {
	raw string
	c   *msemver.Constraints
}

// ParseRange validates a filter-by-range value. "" and "*" mean "no filter"
// and return a nil Range with no error.
func ParseRange(r string) (*Range, error) {
	trimmed := strings.TrimSpace(r)
	if trimmed == "" || trimmed == "*" {
		return nil, nil
	}
	c, err := msemver.NewConstraint(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%q could not be parsed as a valid semver range: %w", r, err)
	}
	return &Range{raw: r, c: c}, nil
}

// Satisfies reports whether the coerced tag version is inside the range.
func (r *Range) Satisfies(v Version) bool {
	mv, err := msemver.NewVersion(fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch))
	if err != nil {
		return false
	}
	return r.c.Check(mv)
}

// String returns the range as written in the config.
func (r *Range) String() string { return r.raw }
