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
)

// maxSafeInteger mirrors JavaScript's Number.MAX_SAFE_INTEGER, which
// node-semver uses as the upper bound for every numeric component.
const maxSafeInteger = 9007199254740991

// maxLength mirrors node-semver's MAX_LENGTH: a version string longer than
// this is rejected before it is even matched against the grammar.
const maxLength = 256

// PreID is one dot-separated prerelease identifier. Numeric identifiers are
// stored as numbers, the way node-semver converts them.
type PreID struct {
	IsNum bool
	Num   uint64
	Str   string
}

func (p PreID) String() string {
	if p.IsNum {
		return strconv.FormatUint(p.Num, 10)
	}
	return p.Str
}

// Version is a parsed semantic version.
type Version struct {
	Major, Minor, Patch uint64
	Pre                 []PreID
	Build               []string
}

// String formats the version the way node-semver's format() does: build
// metadata is never included.
func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if len(v.Pre) > 0 {
		s += "-" + v.PrereleaseString()
	}
	return s
}

// PrereleaseString joins the prerelease identifiers with dots, without the
// leading dash. It returns "" when there is no prerelease.
func (v Version) PrereleaseString() string {
	parts := make([]string, len(v.Pre))
	for i, p := range v.Pre {
		parts[i] = p.String()
	}
	return strings.Join(parts, ".")
}

// fullRe is node-semver's strict FULL regex (src[t.FULL]).
var fullRe = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
	`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][a-zA-Z0-9-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][a-zA-Z0-9-]*))*))?` +
	`(?:\+([a-zA-Z0-9-]+(?:\.[a-zA-Z0-9-]+)*))?$`)

var numericRe = regexp.MustCompile(`^[0-9]+$`)

// Parse is node-semver's parse(version) with default options: the input is
// trimmed and must match the strict grammar (a leading "v" is allowed).
func Parse(s string) (Version, bool) {
	if len(s) > maxLength {
		return Version{}, false
	}
	m := fullRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return Version{}, false
	}
	var v Version
	var ok bool
	if v.Major, ok = component(m[1]); !ok {
		return Version{}, false
	}
	if v.Minor, ok = component(m[2]); !ok {
		return Version{}, false
	}
	if v.Patch, ok = component(m[3]); !ok {
		return Version{}, false
	}
	if m[4] != "" {
		for _, id := range strings.Split(m[4], ".") {
			v.Pre = append(v.Pre, toPreID(id))
		}
	}
	if m[5] != "" {
		v.Build = strings.Split(m[5], ".")
	}
	return v, true
}

func component(s string) (uint64, bool) {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || n > maxSafeInteger {
		return 0, false
	}
	return n, true
}

func toPreID(id string) PreID {
	if numericRe.MatchString(id) {
		if n, err := strconv.ParseUint(id, 10, 64); err == nil && n < maxSafeInteger {
			return PreID{IsNum: true, Num: n}
		}
	}
	return PreID{Str: id}
}

// coerceRe is node-semver's COERCE regex with MAX_SAFE_COMPONENT_LENGTH 16.
var coerceRe = regexp.MustCompile(`(^|[^\d])(\d{1,16})(?:\.(\d{1,16}))?(?:\.(\d{1,16}))?(?:$|[^\d])`)

// Coerce is node-semver's coerce(version) without includePrerelease or rtl:
// it finds the first MAJOR[.MINOR[.PATCH]] run and drops any prerelease.
func Coerce(s string) (Version, bool) {
	m := coerceRe.FindStringSubmatch(s)
	if m == nil {
		return Version{}, false
	}
	minor, patch := m[3], m[4]
	if minor == "" {
		minor = "0"
	}
	if patch == "" {
		patch = "0"
	}
	return Parse(m[2] + "." + minor + "." + patch)
}

// identifierRe is node-semver's PRERELEASE grammar (the part after the
// leading "-"): each dot-separated segment is either a NUMERICIDENTIFIER
// (0, or a run of digits with no leading zero) or a NONNUMERICIDENTIFIER
// (at least one letter or dash). A digit run with a leading zero, such as
// "01", matches neither and is rejected, matching semver.inc's real
// validation of the identifier argument against `-${identifier}`.
var identifierRe = regexp.MustCompile(
	`^(?:0|[1-9]\d*|\d*[a-zA-Z-][a-zA-Z0-9-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][a-zA-Z0-9-]*))*$`)

// Inc is node-semver's inc(version, release, loose, identifier) with the
// default identifierBase. An empty identifier means "no identifier".
func Inc(v Version, release, identifier string) (Version, error) {
	if strings.HasPrefix(release, "pre") && identifier != "" && !identifierRe.MatchString(identifier) {
		return Version{}, fmt.Errorf("invalid identifier: %s", identifier)
	}
	out := v
	out.Pre = append([]PreID(nil), v.Pre...)
	out.Build = append([]string(nil), v.Build...)
	if err := out.inc(release, identifier); err != nil {
		return Version{}, err
	}
	return out, nil
}

func (v *Version) inc(release, identifier string) error {
	switch release {
	case "premajor":
		v.Pre = nil
		v.Patch = 0
		v.Minor = 0
		v.Major++
		return v.inc("pre", identifier)
	case "preminor":
		v.Pre = nil
		v.Patch = 0
		v.Minor++
		return v.inc("pre", identifier)
	case "prepatch":
		v.Pre = nil
		if err := v.inc("patch", identifier); err != nil {
			return err
		}
		return v.inc("pre", identifier)
	case "prerelease":
		if len(v.Pre) == 0 {
			if err := v.inc("patch", identifier); err != nil {
				return err
			}
		}
		return v.inc("pre", identifier)
	case "major":
		if v.Minor != 0 || v.Patch != 0 || len(v.Pre) == 0 {
			v.Major++
		}
		v.Minor = 0
		v.Patch = 0
		v.Pre = nil
	case "minor":
		if v.Patch != 0 || len(v.Pre) == 0 {
			v.Minor++
		}
		v.Patch = 0
		v.Pre = nil
	case "patch":
		if len(v.Pre) == 0 {
			v.Patch++
		}
		v.Pre = nil
	case "pre":
		if len(v.Pre) == 0 {
			v.Pre = []PreID{{IsNum: true, Num: 0}}
		} else {
			incremented := false
			for i := len(v.Pre) - 1; i >= 0; i-- {
				if v.Pre[i].IsNum {
					v.Pre[i].Num++
					incremented = true
					break
				}
			}
			if !incremented {
				v.Pre = append(v.Pre, PreID{IsNum: true, Num: 0})
			}
		}
		if identifier != "" {
			fresh := []PreID{{Str: identifier}, {IsNum: true, Num: 0}}
			ids := strings.Split(identifier, ".")
			if isPrereleaseIdentifier(v.Pre, ids) {
				if len(ids) >= len(v.Pre) || !v.Pre[len(ids)].IsNum {
					v.Pre = fresh
				}
			} else {
				v.Pre = fresh
			}
		}
	default:
		return fmt.Errorf("invalid increment argument: %s", release)
	}
	return nil
}

func isPrereleaseIdentifier(pre []PreID, ids []string) bool {
	if len(ids) > len(pre) {
		return false
	}
	for i, id := range ids {
		if compareIdentifier(pre[i], id) != 0 {
			return false
		}
	}
	return true
}

// compareIdentifier is node-semver's compareIdentifiers for a stored
// identifier and a raw string identifier.
func compareIdentifier(a PreID, b string) int {
	bNum := numericRe.MatchString(b)
	switch {
	case a.IsNum && bNum:
		n, _ := strconv.ParseUint(b, 10, 64)
		switch {
		case a.Num < n:
			return -1
		case a.Num > n:
			return 1
		}
		return 0
	case a.IsNum:
		return -1
	case bNum:
		return 1
	}
	return strings.Compare(a.Str, b)
}
