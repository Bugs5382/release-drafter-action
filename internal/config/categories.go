package config

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

	"github.com/Bugs5382/release-drafter-action/internal/pathmatch"
)

const migrationURL = "https://github.com/release-drafter/release-drafter/pull/1558"

func withMigrationLink(msg string) string {
	return msg + " Migration documentation: " + migrationURL
}

// ParsedConventional is a normalized conventional predicate.
type ParsedConventional struct {
	Types    []string
	Scopes   []string
	Breaking *bool
}

// ParsedCondition is a normalized when entry.
type ParsedCondition struct {
	Labels       []string
	LabelsMode   string
	Paths        []string
	PathsMode    string
	Conventional *ParsedConventional
	// Matchers holds one matcher per distinct path, in first-seen order.
	Matchers []*pathmatch.Matcher
}

// ParsedCategory is a category after v7's parseCategories.
type ParsedCategory struct {
	Type            string
	Title           string
	When            []ParsedCondition
	CollapseAfter   int
	SemverIncrement string
	Exclusive       bool
}

func unique(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func newCondition(labels []string, labelsMode string, paths []string, pathsMode string, conv *ParsedConventional) ParsedCondition {
	c := ParsedCondition{Labels: labels, LabelsMode: labelsMode, Paths: paths, PathsMode: pathsMode, Conventional: conv}
	for _, p := range unique(paths) {
		c.Matchers = append(c.Matchers, pathmatch.New(p))
	}
	return c
}

func normalizeConventional(c *Conventional) *ParsedConventional {
	if c == nil {
		return nil
	}
	if c.Any {
		return &ParsedConventional{Types: []string{}, Scopes: []string{}}
	}
	types := append(append([]string{}, c.Types...), nonEmpty(c.Type)...)
	scopes := append(append([]string{}, c.Scopes...), nonEmpty(c.Scope)...)
	return &ParsedConventional{Types: types, Scopes: scopes, Breaking: c.Breaking}
}

// invalid is a single-field ValidationError for the checks that run after
// Parse, where the config has no line numbers. It has no origin: the caller
// knows the source and the key path already names what to change.
func invalid(path, msg string) *ValidationError {
	return &ValidationError{Errors: []FieldError{{Path: path, Msg: msg}}}
}

// deprecatedKey names the deprecated labels key when it is set, else its
// paths sibling, so a migration error points at a key the user wrote.
func deprecatedKey(labels []string, labelsKey, pathsKey string) string {
	if len(labels) > 0 {
		return labelsKey
	}
	return pathsKey
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

// NormalizeCategories ports v7's parseCategories: it folds the deprecated
// category label/labels shorthands into every when condition, drops empty
// conditions, and turns exclude-labels, exclude-paths, include-labels,
// include-paths and version-resolver into pre-exclude, pre-include and
// version-resolver categories. It returns v7's warnings, word for word.
func NormalizeCategories(cfg *Config) ([]ParsedCategory, []string, error) {
	var warnings []string
	warn := func(format string, args ...any) { warnings = append(warnings, fmt.Sprintf(format, args...)) }
	out := []ParsedCategory{}

	for i, cat := range cfg.Categories {
		deprecated := append(append([]string{}, cat.Labels...), nonEmpty(cat.Label)...)
		if len(deprecated) > 0 {
			on := ""
			if cat.Title != "" {
				on = fmt.Sprintf(" on category \"%s\"", cat.Title)
			}
			warn("%s", withMigrationLink(fmt.Sprintf("Use of deprecated 'categories[*].label' or 'categories[*].labels' field detected%s. Please migrate. This field will be removed in a future release. To migrate, move the labels into the category's 'when' condition.", on)))
		}
		when := cat.When
		if len(when) == 0 && len(deprecated) > 0 {
			when = []Condition{{LabelsMode: "any", PathsMode: "any"}}
		}
		conds := []ParsedCondition{}
		for _, c := range when {
			paths := append(append([]string{}, c.Paths...), nonEmpty(c.Path)...)
			labels := append(append(append([]string{}, deprecated...), c.Labels...), nonEmpty(c.Label)...)
			conv := normalizeConventional(c.Conventional)
			if len(paths) == 0 && len(labels) == 0 && conv == nil {
				continue
			}
			conds = append(conds, newCondition(labels, c.LabelsMode, paths, c.PathsMode, conv))
		}

		switch cat.Type {
		case "changelog":
			out = append(out, ParsedCategory{Type: "changelog", Title: cat.Title, When: conds, CollapseAfter: cat.CollapseAfter, SemverIncrement: cat.SemverIncrement, Exclusive: cat.Exclusive})
		case "version-resolver":
			if cat.Title != "" {
				warn("Title \"%s\" ignored for category of type \"%s\"", cat.Title, cat.Type)
			}
			if cat.CollapseAfter != -1 {
				warn("\"collapse-after\" \"%d\" ignored for category of type \"%s\"", cat.CollapseAfter, cat.Type)
			}
			out = append(out, ParsedCategory{Type: "version-resolver", When: conds, CollapseAfter: -1, SemverIncrement: cat.SemverIncrement, Exclusive: cat.Exclusive})
		case "pre-include", "pre-exclude":
			if cat.Title != "" {
				warn("Title \"%s\" ignored for category of type \"%s\"", cat.Title, cat.Type)
			}
			if cat.CollapseAfter != -1 {
				warn("\"collapse-after\" \"%d\" ignored for category of type \"%s\"", cat.CollapseAfter, cat.Type)
			}
			if cat.Exclusive {
				return nil, warnings, invalid(fmt.Sprintf("categories[%d].exclusive", i), fmt.Sprintf("\"exclusive\" can only be set on categories of type \"changelog\" or \"version-resolver\"; it cannot be used on category of type \"%s\"", cat.Type))
			}
			if cat.SemverIncrement != "patch" {
				warn("\"semver-increment\" \"%s\" ignored for category of type \"%s\"", cat.SemverIncrement, cat.Type)
			}
			out = append(out, ParsedCategory{Type: cat.Type, When: conds, CollapseAfter: -1, SemverIncrement: "patch"})
		default:
			return nil, warnings, invalid(fmt.Sprintf("categories[%d].type", i), "unsupported category type: "+cat.Type)
		}
	}

	has := func(pred func(ParsedCategory) bool) bool {
		for _, c := range out {
			if pred(c) {
				return true
			}
		}
		return false
	}

	if len(cfg.ExcludeLabels) > 0 || len(cfg.ExcludePaths) > 0 {
		warn("%s", withMigrationLink("Use of deprecated 'exclude-labels' or 'exclude-paths' field detected. Please migrate. This field will be removed in a future release. To migrate, add the correspoding labels or paths to a 'type: \"pre-exclude\"' category."))
		if has(func(c ParsedCategory) bool { return c.Type == "pre-exclude" }) {
			return nil, warnings, invalid(deprecatedKey(cfg.ExcludeLabels, "exclude-labels", "exclude-paths"), "a 'pre-exclude' category already exists, so the deprecated exclude-labels and exclude-paths cannot be migrated: remove the deprecated fields or the existing 'pre-exclude' category")
		}
		out = append(out, ParsedCategory{Type: "pre-exclude", CollapseAfter: -1, SemverIncrement: "patch",
			When: []ParsedCondition{newCondition(cfg.ExcludeLabels, "any", cfg.ExcludePaths, "any", nil)}})
	}
	if len(cfg.IncludeLabels) > 0 || len(cfg.IncludePaths) > 0 {
		warn("%s", withMigrationLink("Use of deprecated 'include-labels' or 'include-paths' field detected. Please migrate. This field will be removed in a future release. To migrate, add the correspoding labels or paths to a 'type: \"pre-include\"' category."))
		if has(func(c ParsedCategory) bool { return c.Type == "pre-include" }) {
			return nil, warnings, invalid(deprecatedKey(cfg.IncludeLabels, "include-labels", "include-paths"), "a 'pre-include' category already exists, so the deprecated include-labels and include-paths cannot be migrated: remove the deprecated fields or the existing 'pre-include' category")
		}
		out = append(out, ParsedCategory{Type: "pre-include", CollapseAfter: -1, SemverIncrement: "patch",
			When: []ParsedCondition{newCondition(cfg.IncludeLabels, "any", cfg.IncludePaths, "any", nil)}})
	}
	vr := cfg.VersionResolver
	if vr.Default != "patch" {
		warn("%s", withMigrationLink(fmt.Sprintf("Use of deprecated 'version-resolver.default' field detected. Please migrate. This field will be removed in a future release. To migrate, either add 'semver-increment: \"%s\"' to 'type: changelog' category with no 'when' condition (uncategorized changes), or move the default resolver to a new category with type 'version-resolver' and 'semver-increment' set to \"%s\" - also without 'when' conditions.", vr.Default, vr.Default)))
		if has(func(c ParsedCategory) bool { return c.Type == "version-resolver" && len(c.When) == 0 }) {
			return nil, warnings, invalid("version-resolver.default", "a 'version-resolver' category with no 'when' condition already exists, so the deprecated version-resolver.default cannot be migrated: remove the deprecated field or that category")
		}
		out = append(out, ParsedCategory{Type: "version-resolver", SemverIncrement: vr.Default, CollapseAfter: -1, When: []ParsedCondition{}})
	}
	for _, level := range []struct {
		name   string
		labels []string
	}{{"major", vr.Major}, {"minor", vr.Minor}, {"patch", vr.Patch}} {
		if len(level.labels) == 0 {
			continue
		}
		warn("%s", withMigrationLink(fmt.Sprintf("Use of deprecated 'version-resolver.%s.labels' field detected. Please migrate. This field will be removed in a future release. To migrate, either add 'semver-increment: \"%s\"' to a pre-existing 'type: changelog' category, or move the labels from 'version-resolver.%s.labels' to a new category with type 'version-resolver' and 'semver-increment' set to '%s'.", level.name, level.name, level.name, level.name)))
		out = append(out, ParsedCategory{Type: "version-resolver", SemverIncrement: level.name, CollapseAfter: -1,
			When: []ParsedCondition{newCondition(level.labels, "any", []string{}, "any", nil)}})
	}
	return out, warnings, nil
}
