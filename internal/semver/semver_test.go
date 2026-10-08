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
	"strings"
	"testing"
)

// Expected values in this file were produced by running node-semver 7.8.5
// and compare-versions 6.1.1, the versions release-drafter v7.7.0 locks.

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"v1.2.3", "1.2.3", true},
		{"1.2.3-beta.4", "1.2.3-beta.4", true},
		{" v1.2.3-rc.1+build ", "1.2.3-rc.1", true},
		{"v2", "", false},
		{"1.2.3.4", "", false},
		{"01.2.3", "", false},
		{"foo", "", false},
	}
	for _, c := range cases {
		got, ok := Parse(c.in)
		if ok != c.ok || (ok && got.String() != c.want) {
			t.Errorf("Parse(%q) = %q, %v; want %q, %v", c.in, got.String(), ok, c.want, c.ok)
		}
	}
}

func TestCoerce(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"v1.2.3", "1.2.3", true},
		{"1.2.3-beta.4", "1.2.3", true},
		{"v2", "2.0.0", true},
		{"release-3.4", "3.4.0", true},
		{"1.2.3.4", "1.2.3", true},
		{"v1.2.3-rc.1+build", "1.2.3", true},
		{"10.20", "10.20.0", true},
		{"foo", "", false},
	}
	for _, c := range cases {
		got, ok := Coerce(c.in)
		if ok != c.ok || (ok && got.String() != c.want) {
			t.Errorf("Coerce(%q) = %q, %v; want %q, %v", c.in, got.String(), ok, c.want, c.ok)
		}
	}
}

func TestInc(t *testing.T) {
	cases := []struct{ from, release, id, want string }{
		{"1.2.3", "major", "beta", "2.0.0"},
		{"1.2.3", "minor", "beta", "1.3.0"},
		{"1.2.3", "patch", "beta", "1.2.4"},
		{"1.2.3", "premajor", "", "2.0.0-0"},
		{"1.2.3", "premajor", "beta", "2.0.0-beta.0"},
		{"1.2.3", "premajor", "rc", "2.0.0-rc.0"},
		{"1.2.3", "preminor", "", "1.3.0-0"},
		{"1.2.3", "preminor", "beta", "1.3.0-beta.0"},
		{"1.2.3", "preminor", "rc", "1.3.0-rc.0"},
		{"1.2.3", "prepatch", "", "1.2.4-0"},
		{"1.2.3", "prepatch", "beta", "1.2.4-beta.0"},
		{"1.2.3", "prepatch", "rc", "1.2.4-rc.0"},
		{"1.2.3", "prerelease", "", "1.2.4-0"},
		{"1.2.3", "prerelease", "beta", "1.2.4-beta.0"},
		{"1.2.3", "prerelease", "rc", "1.2.4-rc.0"},
		{"1.2.3-beta.0", "major", "beta", "2.0.0"},
		{"1.2.3-beta.0", "minor", "beta", "1.3.0"},
		{"1.2.3-beta.0", "patch", "beta", "1.2.3"},
		{"1.2.3-beta.0", "premajor", "", "2.0.0-0"},
		{"1.2.3-beta.0", "premajor", "beta", "2.0.0-beta.0"},
		{"1.2.3-beta.0", "premajor", "rc", "2.0.0-rc.0"},
		{"1.2.3-beta.0", "preminor", "", "1.3.0-0"},
		{"1.2.3-beta.0", "preminor", "beta", "1.3.0-beta.0"},
		{"1.2.3-beta.0", "preminor", "rc", "1.3.0-rc.0"},
		{"1.2.3-beta.0", "prepatch", "", "1.2.4-0"},
		{"1.2.3-beta.0", "prepatch", "beta", "1.2.4-beta.0"},
		{"1.2.3-beta.0", "prepatch", "rc", "1.2.4-rc.0"},
		{"1.2.3-beta.0", "prerelease", "", "1.2.3-beta.1"},
		{"1.2.3-beta.0", "prerelease", "beta", "1.2.3-beta.1"},
		{"1.2.3-beta.0", "prerelease", "rc", "1.2.3-rc.0"},
		{"2.0.0-beta.1", "major", "beta", "2.0.0"},
		{"2.0.0-beta.1", "minor", "beta", "2.0.0"},
		{"2.0.0-beta.1", "patch", "beta", "2.0.0"},
		{"2.0.0-beta.1", "premajor", "", "3.0.0-0"},
		{"2.0.0-beta.1", "premajor", "beta", "3.0.0-beta.0"},
		{"2.0.0-beta.1", "premajor", "rc", "3.0.0-rc.0"},
		{"2.0.0-beta.1", "preminor", "", "2.1.0-0"},
		{"2.0.0-beta.1", "preminor", "beta", "2.1.0-beta.0"},
		{"2.0.0-beta.1", "preminor", "rc", "2.1.0-rc.0"},
		{"2.0.0-beta.1", "prepatch", "", "2.0.1-0"},
		{"2.0.0-beta.1", "prepatch", "beta", "2.0.1-beta.0"},
		{"2.0.0-beta.1", "prepatch", "rc", "2.0.1-rc.0"},
		{"2.0.0-beta.1", "prerelease", "", "2.0.0-beta.2"},
		{"2.0.0-beta.1", "prerelease", "beta", "2.0.0-beta.2"},
		{"2.0.0-beta.1", "prerelease", "rc", "2.0.0-rc.0"},
		{"0.0.0", "major", "beta", "1.0.0"},
		{"0.0.0", "minor", "beta", "0.1.0"},
		{"0.0.0", "patch", "beta", "0.0.1"},
		{"0.0.0", "premajor", "", "1.0.0-0"},
		{"0.0.0", "premajor", "beta", "1.0.0-beta.0"},
		{"0.0.0", "premajor", "rc", "1.0.0-rc.0"},
		{"0.0.0", "preminor", "", "0.1.0-0"},
		{"0.0.0", "preminor", "beta", "0.1.0-beta.0"},
		{"0.0.0", "preminor", "rc", "0.1.0-rc.0"},
		{"0.0.0", "prepatch", "", "0.0.1-0"},
		{"0.0.0", "prepatch", "beta", "0.0.1-beta.0"},
		{"0.0.0", "prepatch", "rc", "0.0.1-rc.0"},
		{"0.0.0", "prerelease", "", "0.0.1-0"},
		{"0.0.0", "prerelease", "beta", "0.0.1-beta.0"},
		{"0.0.0", "prerelease", "rc", "0.0.1-rc.0"},
	}
	for _, c := range cases {
		v, ok := Parse(c.from)
		if !ok {
			t.Fatalf("Parse(%q) failed", c.from)
		}
		got, err := Inc(v, c.release, c.id)
		if err != nil {
			t.Fatalf("Inc(%s, %s, %q): %v", c.from, c.release, c.id, err)
		}
		if got.String() != c.want {
			t.Errorf("Inc(%s, %s, %q) = %s; want %s", c.from, c.release, c.id, got.String(), c.want)
		}
	}
}

func TestIncRejectsBadIdentifier(t *testing.T) {
	cases := []string{
		"beta!",
		"01",
		"alpha.01",
	}
	v, _ := Parse("1.2.3")
	for _, id := range cases {
		if _, err := Inc(v, "prerelease", id); err == nil {
			t.Errorf("Inc(1.2.3, prerelease, %q): expected an error for an invalid identifier", id)
		}
	}
}

func TestParseRejectsOverlongVersion(t *testing.T) {
	// node-semver rejects any version string longer than MAX_LENGTH (256
	// characters) before it is even matched against the grammar. This
	// version would otherwise parse cleanly (a valid non-numeric
	// prerelease identifier), so the only reason to reject it is length.
	overlong := "1.2.3-" + strings.Repeat("a", 251)
	if len(overlong) != 257 {
		t.Fatalf("test setup: overlong is %d characters, want 257", len(overlong))
	}
	if _, ok := Parse(overlong); ok {
		t.Errorf("Parse(<257 chars>) = ok; want false")
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
		err  bool
	}{
		{"1.2.3", "1.2.10", -1, false},
		{"v1.2.3", "1.2.3", 0, false},
		{"1.2.3-beta.1", "1.2.3", -1, false},
		{"1.2", "1.2.0", 0, false},
		{"release-1", "1.0.0", 0, true},
		{"1.2.3-alpha", "1.2.3-beta", -1, false},
		{"1.0.0-rc.10", "1.0.0-rc.9", 1, false},
		{"2", "1.9.9", 1, false},
	}
	for _, c := range cases {
		got, err := CompareVersions(c.a, c.b)
		if (err != nil) != c.err || (!c.err && got != c.want) {
			t.Errorf("CompareVersions(%q, %q) = %d, %v; want %d, err=%v", c.a, c.b, got, err, c.want, c.err)
		}
	}
}

func TestRange(t *testing.T) {
	cases := []struct {
		tag, rng string
		want     bool
	}{
		{"v1.2.3", "^1.0.0", true},
		{"v2.0.0", "^1.0.0", false},
		{"v1.5.0", ">=1.2 <2", true},
		{"1.2.3-beta.1", "1.x", true},
		{"v0.3.0", "~0.3", true},
		{"v1.9.0", "1.2.3 - 2.0.0", true},
		{"v0.4.0", ">1 || <0.5", true},
	}
	for _, c := range cases {
		r, err := ParseRange(c.rng)
		if err != nil || r == nil {
			t.Fatalf("ParseRange(%q) = %v, %v", c.rng, r, err)
		}
		v, ok := Coerce(c.tag)
		if !ok {
			t.Fatalf("Coerce(%q) failed", c.tag)
		}
		if got := r.Satisfies(v); got != c.want {
			t.Errorf("%q satisfies %q = %v; want %v", c.tag, c.rng, got, c.want)
		}
	}
	if r, err := ParseRange("*"); r != nil || err != nil {
		t.Errorf("ParseRange(*) = %v, %v; want nil, nil", r, err)
	}
	if _, err := ParseRange("not a range"); err == nil {
		t.Error("ParseRange(not a range) should fail")
	}
}
