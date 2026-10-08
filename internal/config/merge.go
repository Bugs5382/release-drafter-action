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
	"errors"
	"fmt"

	"github.com/Bugs5382/release-drafter-action/internal/pathmatch"
	"github.com/Bugs5382/release-drafter-action/internal/regex"
	"github.com/Bugs5382/release-drafter-action/internal/semver"
)

// Inputs are the action inputs that feed draft mode. Pointers are nil when
// the input was left empty, which is how v7 tells "unset" from "false".
type Inputs struct {
	Name                 *string
	Tag                  *string
	Version              *string
	Publish              bool
	Latest               *bool
	Prerelease           *bool
	IncludePreReleases   *bool
	PrereleaseIdentifier string
	Commitish            string
	Header               string
	Footer               string
	FilterByRange        string
	DryRun               bool
}

// CompiledReplacer is a replacer whose search is compiled.
type CompiledReplacer struct {
	Search  *regex.Regex
	Replace string
}

// Parsed is the config with the inputs merged in, the way v7's
// mergeInputAndConfig leaves it.
type Parsed struct {
	// Config holds the templates and scalar settings after input overrides.
	Config Config
	// Commitish is the release target: the config or input value, else the
	// workflow ref.
	Commitish          string
	Latest             bool
	Prerelease         bool
	IncludePreReleases bool
	Range              *semver.Range
	Replacers          []CompiledReplacer
	Categories         []ParsedCategory
	// Warnings are v7's warnings for this config, word for word.
	Warnings []string
	// Notes are informational messages (input overrides).
	Notes []string
}

// Merge applies the inputs to the config, normalizes the categories,
// compiles the replacers and validates the result. defaultRef is the
// workflow ref (GITHUB_REF, or the event payload's ref).
func Merge(cfg *Config, in Inputs, defaultRef string) (*Parsed, error) {
	c := *cfg
	p := &Parsed{}
	note := func(key, input, config string) {
		p.Notes = append(p.Notes, fmt.Sprintf("Input's %s \"%s\" overrides config's %s \"%s\"", key, input, key, config))
	}
	str := func(key, input string, dst *string) {
		if input == "" {
			return
		}
		if *dst != "" && *dst != input {
			note(key, input, *dst)
		}
		*dst = input
	}
	boolean := func(key string, input *bool, dst **bool) {
		if input == nil {
			return
		}
		if *dst != nil && **dst != *input {
			note(key, fmt.Sprint(*input), fmt.Sprint(**dst))
		}
		v := *input
		*dst = &v
	}
	str("commitish", in.Commitish, &c.Commitish)
	str("header", in.Header, &c.Header)
	str("footer", in.Footer, &c.Footer)
	str("prerelease-identifier", in.PrereleaseIdentifier, &c.PrereleaseIdentifier)
	boolean("prerelease", in.Prerelease, &c.Prerelease)
	boolean("include-pre-releases", in.IncludePreReleases, &c.IncludePreReleases)
	boolean("latest", in.Latest, &c.Latest)
	str("filter-by-range", in.FilterByRange, &c.FilterByRange)

	if c.Latest != nil && *c.Latest && c.Prerelease != nil && *c.Prerelease {
		p.Warnings = append(p.Warnings, "'prerelease' and 'latest' cannot be both true. Switch 'latest' to false - release will be a pre-release.")
		f := false
		c.Latest = &f
	}
	prereleaseOn := c.Prerelease != nil && *c.Prerelease
	if c.PrereleaseIdentifier != "" && !prereleaseOn && (in.Prerelease == nil || in.PrereleaseIdentifier != "") {
		p.Warnings = append(p.Warnings, fmt.Sprintf("You specified a 'prerelease-identifier' (%s), but 'prerelease' is set to false. Switching to true.", c.PrereleaseIdentifier))
		t := true
		c.Prerelease = &t
	}

	p.Commitish = c.Commitish
	if p.Commitish == "" {
		p.Commitish = defaultRef
	}
	p.Latest = c.Latest == nil || *c.Latest
	p.Prerelease = c.Prerelease != nil && *c.Prerelease
	p.IncludePreReleases = c.IncludePreReleases != nil && *c.IncludePreReleases

	for i, r := range c.Replacers {
		re, err := regex.Parse(r.Search, regex.DefaultTimeout)
		if err != nil {
			return nil, fmt.Errorf("replacers[%d].search: %w", i, err)
		}
		p.Replacers = append(p.Replacers, CompiledReplacer{Search: re, Replace: r.Replace})
	}

	cats, warnings, err := NormalizeCategories(&c)
	p.Warnings = append(p.Warnings, warnings...)
	if err != nil {
		return nil, err
	}
	p.Categories = cats
	p.Config = c

	if p.Commitish == "" {
		return nil, invalid("commitish", "'commitish' is required: set the commitish input or config key (it defaults to the workflow ref, which is empty here)")
	}
	// NormalizeCategories emits exactly one entry per configured category,
	// in order, before any migrated one, and migrated entries are never
	// changelog categories. So a changelog entry's index in cats is its
	// index in the config's categories list.
	uncategorized, second := 0, -1
	for i, cat := range cats {
		if cat.Type != "changelog" {
			continue
		}
		if cat.Title == "" {
			return nil, invalid(fmt.Sprintf("categories[%d].title", i), "every 'type: \"changelog\"' category must define a non-empty 'title'")
		}
		if len(cat.When) == 0 {
			uncategorized++
			if uncategorized == 2 {
				second = i
			}
		}
	}
	if uncategorized > 1 {
		return nil, invalid(fmt.Sprintf("categories[%d]", second), "more than one 'type: \"changelog\"' category has no 'when' condition; only one such category can collect the uncategorized changes")
	}
	rng, err := semver.ParseRange(c.FilterByRange)
	if err != nil {
		return nil, invalid("filter-by-range", fmt.Sprintf("'filter-by-range' value %v", err))
	}
	p.Range = rng
	return p, nil
}

// CompiledAutolabel is one autolabeler rule, ready to match.
type CompiledAutolabel struct {
	Label  string
	Files  *pathmatch.Matcher
	Branch []*regex.Regex
	Title  []*regex.Regex
	Body   []*regex.Regex
}

// CompileAutolabeler compiles the autolabeler rules. v7's autolabeler
// requires at least one rule.
func CompileAutolabeler(cfg *Config) ([]CompiledAutolabel, error) {
	if !cfg.HasAutolabeler || len(cfg.Autolabeler) == 0 {
		return nil, errors.New("autolabeler: the config needs at least one autolabeler rule to run on pull_request events")
	}
	out := make([]CompiledAutolabel, 0, len(cfg.Autolabeler))
	for i, a := range cfg.Autolabeler {
		c := CompiledAutolabel{Label: a.Label}
		if len(a.Files) > 0 {
			c.Files = pathmatch.New(a.Files...)
		}
		for _, f := range []struct {
			name string
			src  []string
			dst  *[]*regex.Regex
		}{{"branch", a.Branch, &c.Branch}, {"title", a.Title, &c.Title}, {"body", a.Body, &c.Body}} {
			for j, s := range f.src {
				re, err := regex.Parse(s, regex.DefaultTimeout)
				if err != nil {
					return nil, fmt.Errorf("autolabeler[%d].%s[%d]: %w", i, f.name, j, err)
				}
				*f.dst = append(*f.dst, re)
			}
		}
		out = append(out, c)
	}
	return out, nil
}
