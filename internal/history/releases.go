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
	"sort"
	"strings"
	"time"

	"github.com/Bugs5382/release-drafter-action/internal/logging"
	"github.com/Bugs5382/release-drafter-action/internal/model"
	"github.com/Bugs5382/release-drafter-action/internal/semver"
)

// ReleaseFilter is the subset of the merged config FindPreviousReleases
// needs, mirroring the fields v7's findPreviousReleases takes.
type ReleaseFilter struct {
	Commitish          string
	FilterByCommitish  bool
	TagPrefix          string
	Prerelease         bool
	IncludePreReleases bool
	Range              *semver.Range
}

func stripHeadsPrefix(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

// FindPreviousReleases ports v7's findPreviousReleases: filters releases by
// commitish, filter-by-range and tag-prefix, then splits what is left into
// a draft (the first match, in the order the API returned them) and the
// highest published release (by sortReleases). Either return value is nil
// when nothing matches.
func FindPreviousReleases(releases []model.Release, f ReleaseFilter) (last, draft *model.Release) {
	logging.L().Debug().Int("releases", len(releases)).Str("commitish", f.Commitish).Str("tag_prefix", f.TagPrefix).
		Msg("history: finding the last release and any existing draft")

	target := stripHeadsPrefix(f.Commitish)
	filtered := make([]model.Release, 0, len(releases))
	for _, r := range releases {
		if f.FilterByCommitish && target != stripHeadsPrefix(r.TargetCommitish) {
			continue
		}
		if f.Range != nil {
			v, ok := semver.Coerce(r.TagName)
			if !ok {
				logging.L().Warn().Str("tag_name", r.TagName).Msg("history: failed to coerce a semver version, excluding it from filter-by-range")
				continue
			}
			if !f.Range.Satisfies(v) {
				continue
			}
		}
		if f.TagPrefix != "" && !strings.HasPrefix(r.TagName, f.TagPrefix) {
			continue
		}
		filtered = append(filtered, r)
	}

	var published, drafts []model.Release
	for _, r := range filtered {
		if r.Draft {
			drafts = append(drafts, r)
		} else {
			published = append(published, r)
		}
	}

	published = filterSlice(published, func(r model.Release) bool {
		if f.Prerelease || f.IncludePreReleases {
			return true
		}
		return !r.Prerelease
	})
	drafts = filterSlice(drafts, func(r model.Release) bool {
		if f.Prerelease {
			return r.Prerelease
		}
		return !r.Prerelease
	})

	if len(drafts) > 0 {
		draft = &drafts[0]
		if len(drafts) > 1 {
			tags := make([]string, len(drafts))
			for i, r := range drafts {
				tags[i] = r.TagName
			}
			logging.L().Warn().Strs("tag_names", tags).Msg("history: multiple draft releases found, using the first one the API returned")
		}
	}

	sorted := sortReleases(published, f.TagPrefix)
	if len(sorted) > 0 {
		last = &sorted[len(sorted)-1]
	}

	logging.L().Info().
		Str("last_release", releaseTag(last)).
		Str("draft_release", releaseTag(draft)).
		Msg("history: found the last release and any existing draft")
	return last, draft
}

func releaseTag(r *model.Release) string {
	if r == nil {
		return ""
	}
	return r.TagName
}

func filterSlice(in []model.Release, keep func(model.Release) bool) []model.Release {
	out := make([]model.Release, 0, len(in))
	for _, r := range in {
		if keep(r) {
			out = append(out, r)
		}
	}
	return out
}

// sortReleases ports v7's sortReleases: ascending by version (tagPrefix
// stripped first), falling back to created_at when either tag does not
// compare as a version. The sort is stable, matching Array.prototype.sort.
func sortReleases(releases []model.Release, tagPrefix string) []model.Release {
	out := make([]model.Release, len(releases))
	copy(out, releases)
	sort.SliceStable(out, func(i, j int) bool {
		a := strings.TrimPrefix(out[i].TagName, tagPrefix)
		b := strings.TrimPrefix(out[j].TagName, tagPrefix)
		if cmp, err := semver.CompareVersions(a, b); err == nil {
			return cmp < 0
		}
		ta, errA := time.Parse(time.RFC3339, out[i].CreatedAt)
		tb, errB := time.Parse(time.RFC3339, out[j].CreatedAt)
		if errA != nil || errB != nil {
			return false
		}
		return ta.Before(tb)
	})
	return out
}
