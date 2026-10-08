package action

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
	"testing"
)

// hubFixtureSpec is one pull request (and its one commit) in the fixture
// internal/render/fixture_test.go's newFixture() builds for the "hub" golden
// case in body_test.go. It is reproduced here, as GraphQL wire data, so
// this package's own test can drive the whole action (config load, the
// GitHub client, history and render) and still land on the exact body v7.7.0
// produced for it, not just render.Build in isolation.
type hubFixtureSpec struct {
	number   int
	title    string
	labels   []string
	author   string
	typename string
	merged   string
}

func hubFixtureSpecs() []hubFixtureSpec {
	specs := []hubFixtureSpec{
		{1, "feat(api): add `x_y` endpoint <b> & *stars*", []string{"enhancement"}, "alice", "User", "2026-01-03T10:00:00Z"},
		{2, "fix: crash @mention #12", []string{"bug"}, "Bob", "User", "2026-01-02T10:00:00Z"},
		{3, "chore(deps): bump lib", []string{"dependencies"}, "dependabot", "Bot", "2026-01-04T10:00:00Z"},
		{4, "docs: readme", []string{"documentation", "enhancement"}, "alice", "User", "2026-01-01T10:00:00Z"},
		{5, "ci: pin", []string{"skip-changelog"}, "alice", "User", "2026-01-05T10:00:00Z"},
		{6, "refactor!: drop old", []string{"breaking", "refactor"}, "carol", "User", "2026-01-02T10:00:00Z"},
		{7, "Uncategorized thing", []string{}, "", "", "2026-01-06T10:00:00Z"},
	}
	for i := 8; i < 14; i++ {
		specs = append(specs, hubFixtureSpec{i, fmt.Sprintf("chore(deps): bump dep%d", i), []string{"dependencies"}, "renovate", "Bot", fmt.Sprintf("2026-01-0%dT12:00:00Z", i-6)})
	}
	return specs
}

// hubCompareResponse builds the findCommitsInComparison response: one
// commit per pull request, each with that pull request as its only
// associated pull request, the way newFixture() pairs them 1:1.
func hubCompareResponse(t *testing.T, owner, repo string) []byte {
	nameWithOwner := owner + "/" + repo
	var nodes []map[string]any
	for _, s := range hubFixtureSpecs() {
		var author map[string]any
		if s.author != "" {
			url := "https://github.com/" + s.author
			if s.typename == "Bot" {
				url = "https://github.com/apps/" + s.author
			}
			author = map[string]any{"__typename": s.typename, "login": s.author, "url": url}
		}
		labelNodes := []map[string]any{}
		for _, l := range s.labels {
			labelNodes = append(labelNodes, map[string]any{"__typename": "Label", "name": l})
		}
		pr := map[string]any{
			"__typename":     "PullRequest",
			"title":          s.title,
			"number":         s.number,
			"author":         author,
			"baseRepository": map[string]any{"__typename": "Repository", "nameWithOwner": nameWithOwner},
			"mergedAt":       s.merged,
			"labels":         map[string]any{"__typename": "LabelConnection", "nodes": labelNodes},
			"merged":         true,
		}
		nodes = append(nodes, map[string]any{
			"oid":                    fmt.Sprintf("%040x", s.number),
			"committedDate":          s.merged,
			"message":                s.title,
			"author":                 map[string]any{"name": "Some Bot", "user": nil},
			"authors":                map[string]any{"nodes": []any{}},
			"associatedPullRequests": map[string]any{"nodes": []any{pr}},
		})
	}
	return graphQLEnvelope(t, map[string]any{
		"repository": map[string]any{
			"ref": map[string]any{
				"compare": map[string]any{
					"commits": map[string]any{
						"pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""},
						"nodes":    nodes,
					},
				},
			},
		},
	})
}

// hubFixtureWantBody is v7.7.0's own buildReleasePayload output for the
// fixture above, against last release v1.2.3 and no input overrides. Copied
// from internal/render/body_test.go's TestBuildMatchesV7 "hub" case, which
// documents its provenance: release-drafter v7.7.0 run directly over this
// same fixture.
const hubFixtureWantBody = "# What Changed\n\n- Uncategorized thing @ghost (#7)\n\n## Breaking Changes\n\n- refactor!: drop old @carol (#6)\n\n## Features\n\n- feat(api): add `x_y` endpoint \\<b> \\& \\*stars\\* @alice (#1)\n- docs: readme @alice (#4)\n\n## Bug Fixes\n\n- fix: crash @mention #12 @Bob (#2)\n\n## Changes\n\n- refactor!: drop old @carol (#6)\n\n## Documentation\n\n- docs: readme @alice (#4)\n\n## Dependency Updates\n\n<details>\n<summary>7 changes</summary>\n\n- chore(deps): bump dep13 @[renovate[bot]](https://github.com/apps/renovate) (#13)\n- chore(deps): bump dep12 @[renovate[bot]](https://github.com/apps/renovate) (#12)\n- chore(deps): bump dep11 @[renovate[bot]](https://github.com/apps/renovate) (#11)\n- chore(deps): bump dep10 @[renovate[bot]](https://github.com/apps/renovate) (#10)\n- chore(deps): bump lib @[dependabot[bot]](https://github.com/apps/dependabot) (#3)\n- chore(deps): bump dep9 @[renovate[bot]](https://github.com/apps/renovate) (#9)\n- chore(deps): bump dep8 @[renovate[bot]](https://github.com/apps/renovate) (#8)\n</details>\n\n# Extra\n\n**Full Changelog**: https://github.com/o/r/compare/v1.2.3...v2.0.0\n"
