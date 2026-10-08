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

// Config is the release-drafter config as written, with v7's schema
// defaults applied. Optional values v7 treats by truthiness are plain
// strings where "" means unset; values whose presence matters are pointers.
type Config struct {
	ChangeTemplate              string
	ChangeAuthorTemplate        string
	ChangeAuthorsSeparator      string
	ChangeAuthorsFinalSeparator *string
	ChangeTitleEscapes          string
	NoChangesTemplate           string
	VersionTemplate             string
	NameTemplate                string
	TagPrefix                   string
	TagTemplate                 string
	ExcludeLabels               []string
	IncludeLabels               []string
	IncludePaths                []string
	ExcludePaths                []string
	ExcludeContributors         []string
	NewContributorTemplate      string
	NoNewContributorTemplate    string
	NoContributorsTemplate      string
	SortBy                      string
	SortDirection               string
	FilterByCommitish           bool
	PullRequestLimit            int
	HistoryLimit                int
	Replacers                   []Replacer
	Categories                  []Category
	VersionResolver             VersionResolver
	CategoryTemplate            string
	Template                    string

	// Keys that the config and the action inputs share.
	Latest               *bool
	Prerelease           *bool
	PrereleaseIdentifier string
	IncludePreReleases   *bool
	Commitish            string
	Header               string
	Footer               string
	FilterByRange        string

	// Autolabeler rules. HasAutolabeler is false when the key is absent.
	Autolabeler    []Autolabel
	HasAutolabeler bool

	// ExcludeBots drops pull requests opened by bots before rendering. No
	// config key sets it: only BuiltinDefault does, for the exclude-bots
	// input.
	ExcludeBots bool
}

// Replacer is one search and replace applied to the rendered body.
type Replacer struct {
	Search  string
	Replace string
}

// Category is one entry of categories, before normalization.
type Category struct {
	Title           string
	Type            string
	Exclusive       bool
	CollapseAfter   int
	SemverIncrement string
	Labels          []string
	Label           string
	When            []Condition
}

// Condition is one when entry: all set predicates must hold.
type Condition struct {
	Conventional *Conventional
	Label        string
	Labels       []string
	LabelsMode   string
	Path         string
	Paths        []string
	PathsMode    string
}

// Conventional is the conventional predicate. Any is true for
// `conventional: true`.
type Conventional struct {
	Any      bool
	Type     string
	Types    []string
	Scope    string
	Scopes   []string
	Breaking *bool
}

// VersionResolver is the deprecated version-resolver block.
type VersionResolver struct {
	Major   []string
	Minor   []string
	Patch   []string
	Default string
}

// Autolabel is one autolabeler rule.
type Autolabel struct {
	Label  string
	Files  []string
	Branch []string
	Title  []string
	Body   []string
}

// Defaults returns a Config holding v7's schema defaults.
func Defaults() Config {
	return Config{
		ChangeTemplate:           "* $TITLE (#$NUMBER) $AUTHORS",
		ChangeAuthorTemplate:     "$AUTHOR_MENTION",
		ChangeAuthorsSeparator:   ", ",
		NoChangesTemplate:        "* No changes",
		VersionTemplate:          "$MAJOR.$MINOR.$PATCH$PRERELEASE",
		ExcludeLabels:            []string{},
		IncludeLabels:            []string{},
		IncludePaths:             []string{},
		ExcludePaths:             []string{},
		ExcludeContributors:      []string{},
		NewContributorTemplate:   "* $AUTHOR_MENTION made their first contribution in #$NUMBER",
		NoNewContributorTemplate: "* No new contributors",
		NoContributorsTemplate:   "No contributors",
		SortBy:                   "merged_at",
		SortDirection:            "descending",
		PullRequestLimit:         5,
		HistoryLimit:             15,
		Replacers:                []Replacer{},
		Categories:               []Category{},
		VersionResolver:          VersionResolver{Major: []string{}, Minor: []string{}, Patch: []string{}, Default: "patch"},
		CategoryTemplate:         "## $TITLE",
		Template:                 "",
	}
}
