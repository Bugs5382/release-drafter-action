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

	"github.com/Bugs5382/release-drafter-action/internal/config"
)

// Flags are the action inputs as given on the command line, mirroring
// action.yml: every value stays a string, the way cmd/action receives it
// (the action passes --name=value even when value is empty). parseFlags
// does the typed parsing release-drafter v7 itself does for booleans.
type Flags struct {
	ConfigName           string
	Config               string
	Extends              string
	ExtendsAsset         string
	ExcludeBots          string
	Name                 string
	Tag                  string
	Version              string
	Publish              string
	Latest               string
	Prerelease           string
	PrereleaseIdentifier string
	IncludePreReleases   string
	Commitish            string
	Header               string
	Footer               string
	DryRun               string
	FilterByRange        string
	Since                string
	FirstVersion         string
}

// parsedInputs is Flags after typed parsing: config.Inputs for config.Merge,
// plus the fields config.Inputs has no room for (the config source
// selectors and the first-release since/first-version inputs).
type parsedInputs struct {
	ConfigName   string
	Config       string
	Extends      string
	ExtendsAsset string
	ExcludeBots  bool
	Since        string
	FirstVersion string
	Inputs       config.Inputs
}

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// optBool parses an optional boolean input: "" stays nil (unset), the way
// config.Inputs tells "not given" from "given as false".
func optBool(key, s string) (*bool, error) {
	if s == "" {
		return nil, nil
	}
	b, err := config.ParseBool(s)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	return &b, nil
}

// reqBool parses a required boolean input: "" defaults to false.
func reqBool(key, s string) (bool, error) {
	if s == "" {
		return false, nil
	}
	b, err := config.ParseBool(s)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}
	return b, nil
}

// parseFlags validates and types Flags. The error already names the input
// that failed, so the caller can report it as-is.
func parseFlags(f Flags) (parsedInputs, error) {
	out := parsedInputs{
		ConfigName:   f.ConfigName,
		Config:       f.Config,
		Extends:      f.Extends,
		ExtendsAsset: f.ExtendsAsset,
		Since:        f.Since,
		FirstVersion: f.FirstVersion,
	}

	excludeBots, err := reqBool("exclude-bots", f.ExcludeBots)
	if err != nil {
		return out, err
	}
	out.ExcludeBots = excludeBots

	publish, err := reqBool("publish", f.Publish)
	if err != nil {
		return out, err
	}
	dryRun, err := reqBool("dry-run", f.DryRun)
	if err != nil {
		return out, err
	}
	latest, err := optBool("latest", f.Latest)
	if err != nil {
		return out, err
	}
	prerelease, err := optBool("prerelease", f.Prerelease)
	if err != nil {
		return out, err
	}
	includePreReleases, err := optBool("include-pre-releases", f.IncludePreReleases)
	if err != nil {
		return out, err
	}

	out.Inputs = config.Inputs{
		Name:                 optStr(f.Name),
		Tag:                  optStr(f.Tag),
		Version:              optStr(f.Version),
		Publish:              publish,
		Latest:               latest,
		Prerelease:           prerelease,
		IncludePreReleases:   includePreReleases,
		PrereleaseIdentifier: f.PrereleaseIdentifier,
		Commitish:            f.Commitish,
		Header:               f.Header,
		Footer:               f.Footer,
		FilterByRange:        f.FilterByRange,
		DryRun:               dryRun,
	}
	return out, nil
}
