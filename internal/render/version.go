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
	"fmt"
	"strconv"
	"strings"

	"github.com/Bugs5382/release-drafter-action/internal/config"
	"github.com/Bugs5382/release-drafter-action/internal/logging"
	"github.com/Bugs5382/release-drafter-action/internal/model"
	"github.com/Bugs5382/release-drafter-action/internal/semver"
)

var priority = map[string]int{"patch": 1, "minor": 2, "major": 3}
var byPriority = map[int]string{1: "patch", 2: "minor", 3: "major"}

func highestPriority(prs []model.PullRequest, cats []config.ParsedCategory, fallback bool) int {
	var empty *config.ParsedCategory
	for i := range cats {
		if len(cats[i].When) == 0 {
			empty = &cats[i]
			break
		}
	}
	highest := 0
	matched := map[int]bool{}
	remaining := make([]int, len(prs))
	for i := range prs {
		remaining[i] = i
	}
	for _, c := range cats {
		if len(c.When) == 0 {
			continue
		}
		var hits []int
		for _, i := range remaining {
			if MatchesCategory(c, prs[i]) {
				hits = append(hits, i)
			}
		}
		if len(hits) == 0 {
			continue
		}
		logging.L().Trace().Str("category", c.Title).Int("hits", len(hits)).Str("semver_increment", c.SemverIncrement).
			Bool("exclusive", c.Exclusive).Msg("render: category matched while resolving the increment")
		highest = max(highest, priority[c.SemverIncrement])
		hitSet := map[int]bool{}
		for _, i := range hits {
			matched[i] = true
			hitSet[i] = true
		}
		if c.Exclusive {
			var keep []int
			for _, i := range remaining {
				if !hitSet[i] {
					keep = append(keep, i)
				}
			}
			remaining = keep
		}
	}
	if empty == nil {
		return highest
	}
	if fallback {
		if highest == 0 {
			logging.L().Trace().Str("category", empty.Title).Str("semver_increment", empty.SemverIncrement).
				Msg("render: falling back to the when-less category's increment")
			return priority[empty.SemverIncrement]
		}
		logging.L().Trace().Msg("render: no fallback needed, a category already matched")
		return highest
	}
	if len(matched) == len(prs) {
		logging.L().Trace().Msg("render: every pull request matched, no when-less category contributes")
		return highest
	}
	logging.L().Trace().Str("category", empty.Title).Str("semver_increment", empty.SemverIncrement).
		Msg("render: unmatched pull requests fall to the when-less category")
	return max(highest, priority[empty.SemverIncrement])
}

// ResolveIncrement ports v7's resolveVersionKeyIncrement: the most severe
// semver-increment contributed by changelog categories (for the pull
// requests they end up holding) and version-resolver categories (for their
// own matches, with a when-less one as the fallback). The default is patch.
// With prerelease and a prerelease-identifier it becomes premajor,
// preminor or prepatch.
func ResolveIncrement(prs []model.PullRequest, p *config.Parsed) string {
	filtered := FilterPre(prs, p.Categories)
	changelog := highestPriority(filtered, OfType(p.Categories, "changelog"), false)
	resolver := highestPriority(filtered, OfType(p.Categories, "version-resolver"), true)
	if resolver == 0 {
		resolver = priority["patch"]
	}
	key := byPriority[max(changelog, resolver)]
	prerelease := p.Prerelease && p.Config.PrereleaseIdentifier != ""
	result := key
	if prerelease {
		result = "pre" + key
	}
	logging.L().Debug().Int("changelog_priority", changelog).Int("resolver_priority", resolver).
		Bool("prerelease", prerelease).Str("increment", result).Msg("render: resolved the version increment")
	return result
}

// descriptor is v7's VersionDescriptor.
type descriptor struct {
	v          *semver.Version
	identifier string
	prefix     string
}

func toSemver(s string) *semver.Version {
	if v, ok := semver.Parse(s); ok {
		return &v
	}
	if v, ok := semver.Coerce(s); ok {
		return &v
	}
	return nil
}

func (d descriptor) strip(s string) string {
	if d.prefix != "" && strings.HasPrefix(s, d.prefix) {
		return s[len(d.prefix):]
	}
	return s
}

func (d descriptor) incremented(release string) (descriptor, error) {
	if d.v == nil || release == "no_increment" {
		return d, nil
	}
	next, err := semver.Inc(*d.v, release, d.identifier)
	if err != nil {
		return d, fmt.Errorf("failed to increment version %s with increment %s: %w", d.v, release, err)
	}
	return descriptor{v: &next, identifier: d.identifier, prefix: d.prefix}, nil
}

func (d descriptor) parts() (major, minor, patch, pre string, ok bool) {
	if d.v == nil {
		return "", "", "", "", false
	}
	pre = ""
	if len(d.v.Pre) > 0 {
		pre = "-" + d.v.PrereleaseString()
	}
	return strconv.FormatUint(d.v.Major, 10), strconv.FormatUint(d.v.Minor, 10), strconv.FormatUint(d.v.Patch, 10), pre, true
}

func (d descriptor) rendered(template string) string {
	major, minor, patch, pre, ok := d.parts()
	if !ok {
		return template
	}
	return Render(template, Vars{"$MAJOR": major, "$MINOR": minor, "$PATCH": patch, "$PRERELEASE": pre})
}

// VersionInput is what GetVersionInfo needs from the action inputs.
type VersionInput struct {
	Version, Tag, Name *string
	// FirstVersion is the $RESOLVED_VERSION baseline on a first release
	// (no last release and no version/tag/name override), instead of v7's
	// 0.0.0 fallback: this action's own fix for release-drafter's
	// first-release gap (release-drafter/release-drafter#1630), which left
	// a first release drifting to whatever 0.0.0 plus the highest matched
	// category happens to resolve to. Empty keeps the 0.0.0 fallback. When
	// the resolved increment also asks for a prerelease (prerelease on,
	// with a prerelease-identifier), the first draft is
	// "<first-version>-<identifier>.1" rather than FirstVersion unchanged,
	// so a brand-new repo can run an rc flow from its very first release.
	FirstVersion string
}

func firstSet(values ...*string) string {
	for _, v := range values {
		if v != nil && *v != "" {
			return *v
		}
	}
	return ""
}

// GetVersionInfo ports v7's getVersionInfo. The reference version is the
// version/tag/name input when it parses (and is then used as is), else the
// last release (tag name, then release name, prefix stripped), else 0.0.0.
func GetVersionInfo(last *model.Release, cfg config.Config, in VersionInput, increment string) (Vars, error) {
	base := descriptor{identifier: cfg.PrereleaseIdentifier, prefix: cfg.TagPrefix}
	fromInput := base
	if s := firstSet(in.Version, in.Tag, in.Name); s != "" {
		fromInput.v = toSemver(fromInput.strip(s))
	}
	fromLast := base
	fromLastName := false
	if last != nil {
		if v := toSemver(fromLast.strip(last.TagName)); v != nil {
			fromLast.v = v
		} else {
			fromLast.v = toSemver(fromLast.strip(last.Name))
			fromLastName = true
		}
	}
	incoming := increment
	var ref descriptor
	var reference string
	// firstPrerelease is set only on a first release (no input override, no
	// last release) whose resolved increment asks for a prerelease: this
	// action's own first-release design (see FirstVersion) seeds the
	// prerelease count at 1 (v0.1.0-rc.1), not node-semver's usual -0, so
	// the first draft reads as "release candidate one" and the next
	// published prerelease's "prerelease" increment lands on rc.2.
	firstPrerelease := false
	switch {
	case fromInput.v != nil:
		increment = "no_increment"
		ref = fromInput
		reference = "input"
	case fromLast.v != nil:
		ref = fromLast
		if fromLastName {
			reference = "last name"
		} else {
			reference = "last tag"
		}
		if strings.HasPrefix(increment, "pre") && len(ref.v.Pre) > 0 {
			increment = "prerelease"
		}
	case in.FirstVersion != "":
		v := toSemver(base.strip(in.FirstVersion))
		if v == nil {
			return nil, fmt.Errorf("render: first-version %q does not parse as a version", in.FirstVersion)
		}
		firstPrerelease = strings.HasPrefix(increment, "pre") && base.identifier != ""
		increment = "no_increment"
		ref = descriptor{v: v, identifier: base.identifier, prefix: base.prefix}
		reference = "first release"
	default:
		zero, _ := semver.Parse("0.0.0")
		ref = descriptor{v: &zero, identifier: base.identifier, prefix: base.prefix}
		reference = "0.0.0"
	}
	vars := Vars{}
	for _, next := range []struct{ name, release string }{
		{"$NEXT_MAJOR_VERSION", "major"}, {"$NEXT_MINOR_VERSION", "minor"}, {"$NEXT_PATCH_VERSION", "patch"},
	} {
		d, err := ref.incremented(next.release)
		if err != nil {
			logging.L().Error().Str("version", d.v.String()).Str("increment", next.release).Err(err).
				Msg("render: failed to increment the version")
			return nil, err
		}
		major, minor, patch, _, _ := d.parts()
		vars[next.name] = d.rendered(cfg.VersionTemplate)
		vars[next.name+"_MAJOR"] = major
		vars[next.name+"_MINOR"] = minor
		vars[next.name+"_PATCH"] = patch
	}
	pre, err := ref.incremented("prerelease")
	if err != nil {
		logging.L().Error().Str("version", pre.v.String()).Str("increment", "prerelease").Err(err).
			Msg("render: failed to increment the version")
		return nil, err
	}
	_, _, _, prePart, _ := pre.parts()
	vars["$NEXT_PRERELEASE_VERSION"] = pre.rendered(cfg.VersionTemplate)
	vars["$NEXT_PRERELEASE_VERSION_PRERELEASE"] = prePart
	resolved, err := ref.incremented(increment)
	if err != nil {
		logging.L().Error().Str("version", resolved.v.String()).Str("increment", increment).Err(err).
			Msg("render: failed to increment the version")
		return nil, err
	}
	if firstPrerelease {
		seeded := *resolved.v
		seeded.Pre = []semver.PreID{{Str: base.identifier}, {IsNum: true, Num: 1}}
		resolved = descriptor{v: &seeded, identifier: resolved.identifier, prefix: resolved.prefix}
	}
	major, minor, patch, rpre, _ := resolved.parts()
	vars["$RESOLVED_VERSION"] = resolved.rendered(cfg.VersionTemplate)
	vars["$RESOLVED_VERSION_MAJOR"] = major
	vars["$RESOLVED_VERSION_MINOR"] = minor
	vars["$RESOLVED_VERSION_PATCH"] = patch
	vars["$RESOLVED_VERSION_PRERELEASE"] = rpre
	logging.L().Debug().Str("reference", reference).Str("incoming_increment", incoming).Str("effective_increment", increment).
		Str("resolved_version", vars["$RESOLVED_VERSION"]).Msg("render: resolved the version info")
	return vars, nil
}

// RenderNameOrTag renders the name or tag: the input when given, else the
// template, with the version variables filled in.
func RenderNameOrTag(input *string, template string, vars Vars) string {
	if input != nil {
		return Render(*input, vars)
	}
	return Render(template, vars)
}
