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
	"github.com/Bugs5382/release-drafter-action/internal/model"
)

// Expected [$RESOLVED_VERSION, $NEXT_MAJOR_VERSION, $NEXT_MINOR_VERSION,
// $NEXT_PATCH_VERSION, $NEXT_PRERELEASE_VERSION, $RESOLVED_VERSION_PRERELEASE]
// from v7.7.0's getVersionInfo.
func TestGetVersionInfoMatchesV7(t *testing.T) {
	want := map[string][6]string{
		"v1.2.3 minor":         {"1.3.0", "2.0.0", "1.3.0", "1.2.4", "1.2.4-0", ""},
		"app-1.2.3 prefix":     {"1.2.4", "2.0.0", "1.3.0", "1.2.4", "1.2.4-0", ""},
		"name fallback":        {"5.0.0", "5.0.0", "4.6.0", "4.5.7", "4.5.7-0", ""},
		"prepatch from stable": {"1.2.4-beta.0", "2.0.0", "1.3.0", "1.2.4", "1.2.4-beta.0", "-beta.0"},
		"prerelease bump":      {"1.2.4-beta.1", "2.0.0", "1.3.0", "1.2.4", "1.2.4-beta.1", "-beta.1"},
		"no release":           {"0.1.0", "1.0.0", "0.1.0", "0.0.1", "0.0.1-0", ""},
		"input version wins":   {"9.9.9", "10.0.0", "9.10.0", "9.9.10", "9.9.10-0", ""},
		"unparsable input":     {"1.2.4", "2.0.0", "1.3.0", "1.2.4", "1.2.4-0", ""},
		"custom template":      {"1.2", "2.0", "1.3", "1.2", "1.2", ""},
		"first release pinned": {"1.0.0", "2.0.0", "1.1.0", "1.0.1", "1.0.1-0", ""},
		"first release input wins over first-version": {"9.9.9", "10.0.0", "9.10.0", "9.9.10", "9.9.10-0", ""},
	}
	str := func(s string) *string { return &s }
	base := config.Defaults()
	with := func(f func(*config.Config)) config.Config {
		c := base
		f(&c)
		return c
	}
	cases := []struct {
		name      string
		last      *model.Release
		cfg       config.Config
		in        VersionInput
		increment string
	}{
		{"v1.2.3 minor", &model.Release{TagName: "v1.2.3"}, base, VersionInput{}, "minor"},
		{"app-1.2.3 prefix", &model.Release{TagName: "app-1.2.3"}, with(func(c *config.Config) { c.TagPrefix = "app-" }), VersionInput{}, "patch"},
		{"name fallback", &model.Release{TagName: "latest", Name: "Release 4.5.6"}, base, VersionInput{}, "major"},
		{"prepatch from stable", &model.Release{TagName: "v1.2.3"}, with(func(c *config.Config) { c.PrereleaseIdentifier = "beta" }), VersionInput{}, "prepatch"},
		{"prerelease bump", &model.Release{TagName: "v1.2.4-beta.0"}, with(func(c *config.Config) { c.PrereleaseIdentifier = "beta" }), VersionInput{}, "preminor"},
		{"no release", nil, base, VersionInput{}, "minor"},
		{"input version wins", &model.Release{TagName: "v1.2.3"}, base, VersionInput{Version: str("v9.9.9")}, "major"},
		{"unparsable input", &model.Release{TagName: "v1.2.3"}, base, VersionInput{Version: str("next")}, "patch"},
		{"custom template", &model.Release{TagName: "v1.2.3"}, with(func(c *config.Config) { c.VersionTemplate = "$MAJOR.$MINOR" }), VersionInput{}, "patch"},
		// A first release (no last release) with a minor-labeled pull request
		// in range used to resolve to 0.1.0 (v7's 0.0.0-plus-increment
		// fallback); first-version pins it to a sensible default instead,
		// the way the action's own job-release-drafter.yaml expects.
		{"first release pinned", nil, base, VersionInput{FirstVersion: "1.0.0"}, "minor"},
		{"first release input wins over first-version", nil, base, VersionInput{Version: str("v9.9.9"), FirstVersion: "1.0.0"}, "major"},
	}
	for _, c := range cases {
		vars, err := GetVersionInfo(c.last, c.cfg, c.in, c.increment)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got := [6]string{vars["$RESOLVED_VERSION"], vars["$NEXT_MAJOR_VERSION"], vars["$NEXT_MINOR_VERSION"], vars["$NEXT_PATCH_VERSION"], vars["$NEXT_PRERELEASE_VERSION"], vars["$RESOLVED_VERSION_PRERELEASE"]}
		if got != want[c.name] {
			t.Errorf("%s: got %q, want %q", c.name, got, want[c.name])
		}
	}
}

// TestGetVersionInfoRejectsAnUnparsableFirstVersion pins that, unlike the
// version/tag/name overrides (which fall back to the next reference when
// they do not parse), an unparsable first-version is a configuration
// mistake and fails loudly rather than silently reverting to 0.0.0.
func TestGetVersionInfoRejectsAnUnparsableFirstVersion(t *testing.T) {
	_, err := GetVersionInfo(nil, config.Defaults(), VersionInput{FirstVersion: "not-a-version"}, "minor")
	if err == nil {
		t.Fatal("expected an error for an unparsable first-version")
	}
}

// TestGetVersionInfoFirstReleaseRCFlow covers the rc flow a brand-new
// repo runs through: a first release drafted with prerelease on seeds
// "<first-version>-<identifier>.1" instead of dropping the prerelease
// (issue #4); a later draft computed against a published rc release keeps
// bumping the prerelease count; and promoting to stable (prerelease off,
// whether or not include-pre-releases finds that rc as the last release)
// drops the prerelease suffix without a spurious major/minor/patch bump,
// the same way node-semver resolves "patch"/"minor" against a version
// that already carries a prerelease.
func TestGetVersionInfoFirstReleaseRCFlow(t *testing.T) {
	rcCfg := config.Defaults()
	rcCfg.PrereleaseIdentifier = "rc"

	cases := []struct {
		name      string
		last      *model.Release
		in        VersionInput
		increment string
		want      string
		wantPre   string
	}{
		{
			name:      "first release seeds rc.1",
			last:      nil,
			in:        VersionInput{FirstVersion: "0.1.0"},
			increment: "preminor",
			want:      "0.1.0-rc.1",
			wantPre:   "-rc.1",
		},
		{
			name:      "next draft bumps the published rc",
			last:      &model.Release{TagName: "v0.1.0-rc.1"},
			in:        VersionInput{},
			increment: "preminor",
			want:      "0.1.0-rc.2",
			wantPre:   "-rc.2",
		},
		{
			name:      "promoting drops the prerelease without bumping",
			last:      &model.Release{TagName: "v0.1.0-rc.1"},
			in:        VersionInput{},
			increment: "minor",
			want:      "0.1.0",
			wantPre:   "",
		},
	}
	for _, c := range cases {
		vars, err := GetVersionInfo(c.last, rcCfg, c.in, c.increment)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := vars["$RESOLVED_VERSION"]; got != c.want {
			t.Errorf("%s: $RESOLVED_VERSION = %q, want %q", c.name, got, c.want)
		}
		if got := vars["$RESOLVED_VERSION_PRERELEASE"]; got != c.wantPre {
			t.Errorf("%s: $RESOLVED_VERSION_PRERELEASE = %q, want %q", c.name, got, c.wantPre)
		}
	}
}

// TestGetVersionInfoFirstReleaseWithoutIdentifierKeepsPlainFirstVersion
// pins that prerelease alone, with no prerelease-identifier, never seeds a
// prerelease on a first release: ResolveIncrement never asks for one
// (p.Config.PrereleaseIdentifier == "" keeps the plain key), so the
// existing "pinned to first-version exactly" behavior is unchanged.
func TestGetVersionInfoFirstReleaseWithoutIdentifierKeepsPlainFirstVersion(t *testing.T) {
	vars, err := GetVersionInfo(nil, config.Defaults(), VersionInput{FirstVersion: "0.1.0"}, "minor")
	if err != nil {
		t.Fatal(err)
	}
	if got := vars["$RESOLVED_VERSION"]; got != "0.1.0" {
		t.Errorf("$RESOLVED_VERSION = %q, want %q", got, "0.1.0")
	}
}

// TestRenderNameOrTagKeepsTheRCSuffix pins that a custom tag-template (or
// name-template) built around $RESOLVED_VERSION carries the prerelease
// suffix through untouched, the same as any other resolved version.
func TestRenderNameOrTagKeepsTheRCSuffix(t *testing.T) {
	rcCfg := config.Defaults()
	rcCfg.PrereleaseIdentifier = "rc"
	vars, err := GetVersionInfo(nil, rcCfg, VersionInput{FirstVersion: "0.1.0"}, "preminor")
	if err != nil {
		t.Fatal(err)
	}
	if got := RenderNameOrTag(nil, "v$RESOLVED_VERSION", vars); got != "v0.1.0-rc.1" {
		t.Errorf("tag-template = %q, want %q", got, "v0.1.0-rc.1")
	}
}

func TestResolveIncrement(t *testing.T) {
	data := "categories:\n  - title: Features\n    semver-increment: minor\n    labels: [feature]\n  - title: Other\n    semver-increment: patch\n" +
		"version-resolver:\n  major:\n    labels: [breaking]\n"
	cfg, err := config.Parse([]byte(data), "test")
	if err != nil {
		t.Fatal(err)
	}
	p, err := config.Merge(cfg, config.Inputs{}, "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		labels [][]string
		want   string
	}{
		{nil, "patch"},
		{[][]string{{"docs"}}, "patch"},
		{[][]string{{"feature"}, {"docs"}}, "minor"},
		{[][]string{{"feature"}, {"breaking"}}, "major"},
	}
	for _, c := range cases {
		var prs []model.PullRequest
		for i, l := range c.labels {
			prs = append(prs, model.PullRequest{Number: i + 1, Labels: l})
		}
		if got := ResolveIncrement(prs, p); got != c.want {
			t.Errorf("labels %v: got %s, want %s", c.labels, got, c.want)
		}
	}
	yes := true
	pre, _ := config.Merge(cfg, config.Inputs{Prerelease: &yes, PrereleaseIdentifier: "rc"}, "refs/heads/main")
	if got := ResolveIncrement([]model.PullRequest{{Labels: []string{"feature"}}}, pre); got != "preminor" {
		t.Errorf("prerelease increment = %s, want preminor", got)
	}
}
