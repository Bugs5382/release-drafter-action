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

import (
	"testing"

	"github.com/Bugs5382/release-drafter-action/internal/model"
	"github.com/Bugs5382/release-drafter-action/internal/semver"
)

func rel(tag string, draft, prerelease bool, targetCommitish, createdAt string) model.Release {
	return model.Release{TagName: tag, Name: tag, Draft: draft, Prerelease: prerelease, TargetCommitish: targetCommitish, CreatedAt: createdAt}
}

func TestFindPreviousReleasesPicksTheHighestPublishedVersion(t *testing.T) {
	releases := []model.Release{
		rel("v1.0.0", false, false, "main", "2026-01-01T00:00:00Z"),
		rel("v1.2.0", false, false, "main", "2026-03-01T00:00:00Z"),
		rel("v1.1.0", false, false, "main", "2026-02-01T00:00:00Z"),
	}
	last, draft := FindPreviousReleases(releases, ReleaseFilter{Commitish: "main"})
	if last == nil || last.TagName != "v1.2.0" {
		t.Fatalf("last = %+v, want v1.2.0", last)
	}
	if draft != nil {
		t.Fatalf("draft = %+v, want nil", draft)
	}
}

func TestFindPreviousReleasesReturnsTheFirstMatchingDraft(t *testing.T) {
	releases := []model.Release{
		rel("v1.3.0", true, false, "main", "2026-04-01T00:00:00Z"),
		rel("v1.0.0", false, false, "main", "2026-01-01T00:00:00Z"),
		rel("v1.4.0", true, false, "main", "2026-05-01T00:00:00Z"),
	}
	last, draft := FindPreviousReleases(releases, ReleaseFilter{Commitish: "main"})
	if last == nil || last.TagName != "v1.0.0" {
		t.Fatalf("last = %+v, want v1.0.0", last)
	}
	if draft == nil || draft.TagName != "v1.3.0" {
		t.Fatalf("draft = %+v, want the first draft (v1.3.0)", draft)
	}
}

func TestFindPreviousReleasesFiltersByCommitishWhenEnabled(t *testing.T) {
	releases := []model.Release{
		rel("v1.0.0", false, false, "develop", "2026-01-01T00:00:00Z"),
		rel("v1.1.0", false, false, "main", "2026-02-01T00:00:00Z"),
	}
	last, _ := FindPreviousReleases(releases, ReleaseFilter{Commitish: "refs/heads/main", FilterByCommitish: true})
	if last == nil || last.TagName != "v1.1.0" {
		t.Fatalf("last = %+v, want v1.1.0 (develop excluded)", last)
	}
}

func TestFindPreviousReleasesFiltersByTagPrefix(t *testing.T) {
	releases := []model.Release{
		rel("web-v1.0.0", false, false, "main", "2026-01-01T00:00:00Z"),
		rel("api-v2.0.0", false, false, "main", "2026-02-01T00:00:00Z"),
	}
	last, _ := FindPreviousReleases(releases, ReleaseFilter{Commitish: "main", TagPrefix: "api-"})
	if last == nil || last.TagName != "api-v2.0.0" {
		t.Fatalf("last = %+v, want api-v2.0.0", last)
	}
}

func TestFindPreviousReleasesExcludesPrereleasesByDefault(t *testing.T) {
	releases := []model.Release{
		rel("v1.0.0", false, false, "main", "2026-01-01T00:00:00Z"),
		rel("v2.0.0-beta.1", false, true, "main", "2026-02-01T00:00:00Z"),
	}
	last, _ := FindPreviousReleases(releases, ReleaseFilter{Commitish: "main"})
	if last == nil || last.TagName != "v1.0.0" {
		t.Fatalf("last = %+v, want v1.0.0 (prerelease excluded)", last)
	}
}

func TestFindPreviousReleasesIncludesPrereleasesWhenAsked(t *testing.T) {
	releases := []model.Release{
		rel("v1.0.0", false, false, "main", "2026-01-01T00:00:00Z"),
		rel("v2.0.0-beta.1", false, true, "main", "2026-02-01T00:00:00Z"),
	}
	last, _ := FindPreviousReleases(releases, ReleaseFilter{Commitish: "main", IncludePreReleases: true})
	if last == nil || last.TagName != "v2.0.0-beta.1" {
		t.Fatalf("last = %+v, want v2.0.0-beta.1", last)
	}
}

func TestFindPreviousReleasesFiltersByRange(t *testing.T) {
	releases := []model.Release{
		rel("v1.9.0", false, false, "main", "2026-01-01T00:00:00Z"),
		rel("v2.0.0", false, false, "main", "2026-02-01T00:00:00Z"),
	}
	rng, err := semver.ParseRange("1.x")
	if err != nil {
		t.Fatalf("ParseRange: %v", err)
	}
	last, _ := FindPreviousReleases(releases, ReleaseFilter{Commitish: "main", Range: rng})
	if last == nil || last.TagName != "v1.9.0" {
		t.Fatalf("last = %+v, want v1.9.0 (v2.0.0 out of range)", last)
	}
}

func TestFindPreviousReleasesFallsBackToCreatedAtForNonSemverTags(t *testing.T) {
	releases := []model.Release{
		rel("nightly-2026-01-01", false, false, "main", "2026-01-01T00:00:00Z"),
		rel("nightly-2026-03-01", false, false, "main", "2026-03-01T00:00:00Z"),
		rel("nightly-2026-02-01", false, false, "main", "2026-02-01T00:00:00Z"),
	}
	last, _ := FindPreviousReleases(releases, ReleaseFilter{Commitish: "main"})
	if last == nil || last.TagName != "nightly-2026-03-01" {
		t.Fatalf("last = %+v, want the most recently created tag", last)
	}
}

func TestFindPreviousReleasesReturnsNilsWhenNothingMatches(t *testing.T) {
	last, draft := FindPreviousReleases(nil, ReleaseFilter{Commitish: "main"})
	if last != nil || draft != nil {
		t.Fatalf("last = %+v, draft = %+v, want both nil", last, draft)
	}
}
