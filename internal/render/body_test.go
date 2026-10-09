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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bugs5382/release-drafter-action/internal/config"
	"github.com/Bugs5382/release-drafter-action/internal/model"
)

// The expected bodies were produced by release-drafter v7.7.0's own
// buildReleasePayload steps run over the same fixture (the first-release
// case without v7's banner, and with the first commit as $PREVIOUS_TAG).
func TestBuildMatchesV7(t *testing.T) {
	str := func(s string) *string { return &s }
	yes := true
	cases := []struct {
		name, config   string
		last           *model.Release
		inputs         config.Inputs
		previousTag    string
		newLogins      map[string]bool
		wantBody       string
		wantName       string
		wantTag        string
		wantVersion    [5]string
		wantPrerelease bool
		wantMakeLatest bool
		wantDraft      bool
	}{
		{"hub", "hub-plain.yml", &model.Release{TagName: "v1.2.3", Name: "v1.2.3"}, config.Inputs{}, "v1.2.3", nil,
			"# What Changed\n\n- Uncategorized thing @ghost (#7)\n\n## Breaking Changes\n\n- refactor!: drop old @carol (#6)\n\n## Features\n\n- feat(api): add `x_y` endpoint \\<b> \\& \\*stars\\* @alice (#1)\n- docs: readme @alice (#4)\n\n## Bug Fixes\n\n- fix: crash @mention #12 @Bob (#2)\n\n## Changes\n\n- refactor!: drop old @carol (#6)\n\n## Documentation\n\n- docs: readme @alice (#4)\n\n## Dependency Updates\n\n<details>\n<summary>7 changes</summary>\n\n- chore(deps): bump dep13 @[renovate[bot]](https://github.com/apps/renovate) (#13)\n- chore(deps): bump dep12 @[renovate[bot]](https://github.com/apps/renovate) (#12)\n- chore(deps): bump dep11 @[renovate[bot]](https://github.com/apps/renovate) (#11)\n- chore(deps): bump dep10 @[renovate[bot]](https://github.com/apps/renovate) (#10)\n- chore(deps): bump lib @[dependabot[bot]](https://github.com/apps/dependabot) (#3)\n- chore(deps): bump dep9 @[renovate[bot]](https://github.com/apps/renovate) (#9)\n- chore(deps): bump dep8 @[renovate[bot]](https://github.com/apps/renovate) (#8)\n</details>\n\n# Extra\n\n**Full Changelog**: https://github.com/o/r/compare/v1.2.3...v2.0.0\n",
			"v2.0.0", "v2.0.0", [5]string{"2.0.0", "2", "0", "0", ""}, false, true, true},
		{"rich", "rich.yml", &model.Release{TagName: "v2.0.0-rc.1", Name: "v2.0.0-rc.1"}, config.Inputs{Prerelease: &yes}, "v2.0.0-rc.1", map[string]bool{"Bob": true},
			"Top o/r\n## Changes since v2.0.0-rc.1\n## Breaking\n\n* [Breaking] refactor!: drop old (#6) by @carol & @eve (carol)\n\n## Features\n\n* [Features] docs: readme (#4) by @alice (alice)\n* [Features] feat(api): add \\`x\\_y\\` endpoint \\<b> \\& \\*stars\\* (#1) by @alice & Dave NoUser (alice)\n\n## Fixes\n\n* [Fixes] fix: crash @<!---->mention #<!---->12 (#2) by @Bob (Bob)\n\n## Deps\n\n<details>\n<summary>7 changes</summary>\n\n* [Deps] chore(deps): bump dep10 (#10) by [@bot:RENOVATE](https://github.com/apps/renovate) & Some Bot ([bot:RENOVATE](https://github.com/apps/renovate))\n* [Deps] chore(deps): bump dep11 (#11) by [@bot:RENOVATE](https://github.com/apps/renovate) & Some Bot ([bot:RENOVATE](https://github.com/apps/renovate))\n* [Deps] chore(deps): bump dep12 (#12) by [@bot:RENOVATE](https://github.com/apps/renovate) & Some Bot ([bot:RENOVATE](https://github.com/apps/renovate))\n* [Deps] chore(deps): bump dep13 (#13) by [@bot:RENOVATE](https://github.com/apps/renovate) & Some Bot ([bot:RENOVATE](https://github.com/apps/renovate))\n* [Deps] chore(deps): bump dep8 (#8) by [@bot:RENOVATE](https://github.com/apps/renovate) & Some Bot ([bot:RENOVATE](https://github.com/apps/renovate))\n* [Deps] chore(deps): bump dep9 (#9) by [@bot:RENOVATE](https://github.com/apps/renovate) & Some Bot ([bot:RENOVATE](https://github.com/apps/renovate))\n* [Deps] chore(deps): bump lib (#3) by [@bot:DEPENDABOT](https://github.com/apps/dependabot) & Some Bot ([bot:DEPENDABOT](https://github.com/apps/dependabot))\n</details>\n\n## Other\n\n* [Other] Uncategorized thing (#7) by Some Bot (ghost)\n\nContributors: @alice, @Bob, [@bot:DEPENDABOT](https://github.com/apps/dependabot), [@bot:RENOVATE](https://github.com/apps/renovate), Dave NoUser, @eve and Some Bot\n\nNew:\n* @Bob made their first contribution in #2\nNext: 2.0.0 / 2.0.0 / 2.0.0 / 2.0.0-rc.2\n\nEnd 2.0.0-rc.2",
			"Release 2.0.0-rc.2", "v2.0.0-rc.2", [5]string{"2.0.0-rc.2", "2", "0", "0", "-rc.2"}, true, true, true},
		{"paths", "paths.yml", &model.Release{TagName: "v0.1.0"}, config.Inputs{}, "v0.1.0", nil,
			"## Code\n\n- feat(api): add `x_y` endpoint <b> & *stars* @alice (#1)\n\n## Anything\n\n- fix: crash @mention #12 @Bob (#2)",
			"", "", [5]string{"0.1.1", "0", "1", "1", ""}, false, true, true},
		{"first", "hub-plain.yml", nil, config.Inputs{}, "0000000000000000000000000000000000000001", nil,
			"# What Changed\n\n- Uncategorized thing @ghost (#7)\n\n## Breaking Changes\n\n- refactor!: drop old @carol (#6)\n\n## Features\n\n- feat(api): add `x_y` endpoint \\<b> \\& \\*stars\\* @alice (#1)\n- docs: readme @alice (#4)\n\n## Bug Fixes\n\n- fix: crash @mention #12 @Bob (#2)\n\n## Changes\n\n- refactor!: drop old @carol (#6)\n\n## Documentation\n\n- docs: readme @alice (#4)\n\n## Dependency Updates\n\n<details>\n<summary>7 changes</summary>\n\n- chore(deps): bump dep13 @[renovate[bot]](https://github.com/apps/renovate) (#13)\n- chore(deps): bump dep12 @[renovate[bot]](https://github.com/apps/renovate) (#12)\n- chore(deps): bump dep11 @[renovate[bot]](https://github.com/apps/renovate) (#11)\n- chore(deps): bump dep10 @[renovate[bot]](https://github.com/apps/renovate) (#10)\n- chore(deps): bump lib @[dependabot[bot]](https://github.com/apps/dependabot) (#3)\n- chore(deps): bump dep9 @[renovate[bot]](https://github.com/apps/renovate) (#9)\n- chore(deps): bump dep8 @[renovate[bot]](https://github.com/apps/renovate) (#8)\n</details>\n\n# Extra\n\n**Full Changelog**: https://github.com/o/r/compare/0000000000000000000000000000000000000001...v1.0.0\n",
			"v1.0.0", "v1.0.0", [5]string{"1.0.0", "1", "0", "0", ""}, false, true, true},
		{"input", "rich.yml", &model.Release{TagName: "v2.0.0", Name: "v2.0.0"}, config.Inputs{Version: str("3.0.0"), Name: str("Custom $RESOLVED_VERSION")}, "v2.0.0", nil,
			"Top o/r\n## Changes since v2.0.0\n## Breaking\n\n* [Breaking] refactor!: drop old (#6) by @carol & @eve (carol)\n\n## Features\n\n* [Features] docs: readme (#4) by @alice (alice)\n* [Features] feat(api): add \\`x\\_y\\` endpoint \\<b> \\& \\*stars\\* (#1) by @alice & Dave NoUser (alice)\n\n## Fixes\n\n* [Fixes] fix: crash @<!---->mention #<!---->12 (#2) by @Bob (Bob)\n\n## Deps\n\n<details>\n<summary>7 changes</summary>\n\n* [Deps] chore(deps): bump dep10 (#10) by [@bot:RENOVATE](https://github.com/apps/renovate) & Some Bot ([bot:RENOVATE](https://github.com/apps/renovate))\n* [Deps] chore(deps): bump dep11 (#11) by [@bot:RENOVATE](https://github.com/apps/renovate) & Some Bot ([bot:RENOVATE](https://github.com/apps/renovate))\n* [Deps] chore(deps): bump dep12 (#12) by [@bot:RENOVATE](https://github.com/apps/renovate) & Some Bot ([bot:RENOVATE](https://github.com/apps/renovate))\n* [Deps] chore(deps): bump dep13 (#13) by [@bot:RENOVATE](https://github.com/apps/renovate) & Some Bot ([bot:RENOVATE](https://github.com/apps/renovate))\n* [Deps] chore(deps): bump dep8 (#8) by [@bot:RENOVATE](https://github.com/apps/renovate) & Some Bot ([bot:RENOVATE](https://github.com/apps/renovate))\n* [Deps] chore(deps): bump dep9 (#9) by [@bot:RENOVATE](https://github.com/apps/renovate) & Some Bot ([bot:RENOVATE](https://github.com/apps/renovate))\n* [Deps] chore(deps): bump lib (#3) by [@bot:DEPENDABOT](https://github.com/apps/dependabot) & Some Bot ([bot:DEPENDABOT](https://github.com/apps/dependabot))\n</details>\n\n## Other\n\n* [Other] Uncategorized thing (#7) by Some Bot (ghost)\n\nContributors: @alice, @Bob, [@bot:DEPENDABOT](https://github.com/apps/dependabot), [@bot:RENOVATE](https://github.com/apps/renovate), Dave NoUser, @eve and Some Bot\n\nNew:\n* No new contributors\nNext: 4.0.0 / 3.1.0 / 3.0.1 / 3.0.1-rc.0\n\nEnd 3.0.0",
			"Custom 3.0.0", "v3.0.0", [5]string{"3.0.0", "3", "0", "0", ""}, true, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", c.config))
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse(data, c.config)
			if err != nil {
				t.Fatal(err)
			}
			p, err := config.Merge(cfg, c.inputs, "refs/heads/main")
			if err != nil {
				t.Fatal(err)
			}
			f := newFixture()
			prs := append([]model.PullRequest(nil), f.prs...)
			if NeedsChangedFiles(p.Categories) {
				for i := range prs {
					prs[i].ChangedFiles = f.files[prs[i].Number]
				}
			}
			got, err := Build(BuildInput{Parsed: p, Inputs: c.inputs, LastRelease: c.last, PreviousTag: c.previousTag,
				Commits: f.commits, PullRequests: prs, NewContributors: c.newLogins, Owner: "o", Repo: "r", ServerURL: "https://github.com"})
			if err != nil {
				t.Fatal(err)
			}
			if got.Body != c.wantBody {
				t.Errorf("body:\n--- got ---\n%s\n--- want ---\n%s", got.Body, c.wantBody)
			}
			if got.Name != c.wantName || got.Tag != c.wantTag {
				t.Errorf("name/tag = %q/%q; want %q/%q", got.Name, got.Tag, c.wantName, c.wantTag)
			}
			v := [5]string{got.ResolvedVersion, got.MajorVersion, got.MinorVersion, got.PatchVersion, got.PrereleaseVersion}
			if v != c.wantVersion {
				t.Errorf("versions = %q; want %q", v, c.wantVersion)
			}
			if got.Prerelease != c.wantPrerelease || got.MakeLatest != c.wantMakeLatest || got.Draft != c.wantDraft {
				t.Errorf("prerelease/latest/draft = %v/%v/%v; want %v/%v/%v",
					got.Prerelease, got.MakeLatest, got.Draft, c.wantPrerelease, c.wantMakeLatest, c.wantDraft)
			}
		})
	}
}

// TestBuildPreviousTagFromLastRelease pins that $PREVIOUS_TAG comes from
// LastRelease.TagName when LastRelease is set, the way v7 fills
// lastRelease.tag_name, even when the caller's PreviousTag disagrees (the
// input is only a first-release fallback, e.g. the SHA of the first
// commit).
func TestBuildPreviousTagFromLastRelease(t *testing.T) {
	cfg, err := config.Parse([]byte("template: \"$PREVIOUS_TAG\"\n"), "test")
	if err != nil {
		t.Fatal(err)
	}
	p, err := config.Merge(cfg, config.Inputs{}, "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Build(BuildInput{
		Parsed:      p,
		LastRelease: &model.Release{TagName: "v1.2.3"},
		PreviousTag: "0000000000000000000000000000000000000001",
		Owner:       "o", Repo: "r",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "v1.2.3" {
		t.Errorf("body = %q; want %q", got.Body, "v1.2.3")
	}

	// On a first release (LastRelease nil), PreviousTag is used as-is.
	got, err = Build(BuildInput{
		Parsed:      p,
		LastRelease: nil,
		PreviousTag: "0000000000000000000000000000000000000001",
		Owner:       "o", Repo: "r",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "0000000000000000000000000000000000000001" {
		t.Errorf("body = %q; want the first-release fallback", got.Body)
	}
}

// Review focus: v7 renders the version variables over the finished body, so
// a pull request title that mentions one gets it filled in, and an unknown
// $VARIABLE stays as written.
func TestTitlesWithVariables(t *testing.T) {
	cfg, err := config.Parse([]byte("template: \"$CHANGES\"\nchange-template: '- $TITLE'\n"), "test")
	if err != nil {
		t.Fatal(err)
	}
	p, err := config.Merge(cfg, config.Inputs{}, "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	prs := []model.PullRequest{{Number: 1, Title: "docs: mention $RESOLVED_VERSION and $UNKNOWN", Merged: true, BaseRepository: "o/r"}}
	got, err := Build(BuildInput{Parsed: p, LastRelease: &model.Release{TagName: "v1.2.3"}, PreviousTag: "v1.2.3", PullRequests: prs, Owner: "o", Repo: "r"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "- docs: mention 1.2.4 and $UNKNOWN" {
		t.Errorf("body = %q", got.Body)
	}
}

// Review focus: exclude-bots is never set by a config key, only by
// config.BuiltinDefault for the exclude-bots input; Build drops the bot's
// pull request before anything downstream (sorting, the changelog, and the
// version increment) ever sees it.
func TestBuildExcludesBotsWhenConfigured(t *testing.T) {
	cfg, err := config.Parse([]byte("template: \"$CHANGES\"\nchange-template: '- $TITLE'\ncategories:\n  - title: Deps\n    labels: [dependencies]\n    semver-increment: major\n"), "test")
	if err != nil {
		t.Fatal(err)
	}
	cfg.ExcludeBots = true
	p, err := config.Merge(cfg, config.Inputs{}, "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	prs := []model.PullRequest{
		{Number: 1, Title: "feat: human change", Merged: true, BaseRepository: "o/r", Author: &model.Actor{Typename: "User", Login: "alice"}, Labels: []string{"enhancement"}},
		{Number: 2, Title: "chore(deps): bump lib", Merged: true, BaseRepository: "o/r", Author: &model.Actor{Typename: "Bot", Login: "dependabot"}, Labels: []string{"dependencies"}},
	}
	got, err := Build(BuildInput{Parsed: p, LastRelease: &model.Release{TagName: "v1.0.0"}, PullRequests: prs, Owner: "o", Repo: "r"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.Body, "bump lib") {
		t.Errorf("body still holds the bot's pull request: %q", got.Body)
	}
	if !strings.Contains(got.Body, "human change") {
		t.Errorf("body dropped the human pull request: %q", got.Body)
	}
	if got.ResolvedVersion != "1.0.1" {
		t.Errorf("resolved_version = %q, want 1.0.1 (the bot's major-bump dependency change never reaches the Deps category)", got.ResolvedVersion)
	}
}

// TestBuildBuiltinDefaultListsOnlyNewContributors pins issue #7: the
// built-in default's "## New Contributors" section calls out only the
// authors making their first contribution, the same as upstream
// release-drafter's own default, not a roster of every author with a
// merged pull request in the release.
func TestBuildBuiltinDefaultListsOnlyNewContributors(t *testing.T) {
	cfg, err := config.BuiltinDefault(false)
	if err != nil {
		t.Fatal(err)
	}
	p, err := config.Merge(cfg, config.Inputs{}, "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	prs := []model.PullRequest{
		{Number: 1, Title: "feat: add the first endpoint", Merged: true, BaseRepository: "o/r",
			Author: &model.Actor{Typename: "User", Login: "alice"}, Labels: []string{"enhancement"}},
		{Number: 2, Title: "fix: handle empty input", Merged: true, BaseRepository: "o/r",
			Author: &model.Actor{Typename: "User", Login: "bob"}, Labels: []string{"bug"}},
	}
	got, err := Build(BuildInput{
		Parsed: p, LastRelease: &model.Release{TagName: "v1.0.0"}, PullRequests: prs,
		NewContributors: map[string]bool{"alice": true}, Owner: "o", Repo: "r", ServerURL: "https://github.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Body, "## New Contributors") {
		t.Errorf("body missing the New Contributors heading: %q", got.Body)
	}
	if !strings.Contains(got.Body, "@alice made their first contribution in #1") {
		t.Errorf("body missing alice's first-contribution line: %q", got.Body)
	}
	newSection := got.Body[strings.Index(got.Body, "## New Contributors"):]
	if strings.Contains(newSection, "@bob") {
		t.Errorf("New Contributors section calls out bob, who is not a new contributor: %q", newSection)
	}
	if strings.Contains(got.Body, "## Contributors\n") {
		t.Errorf("body still carries the full-roster Contributors heading: %q", got.Body)
	}
}
