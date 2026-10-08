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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
)

// LoadOptions are the inputs that pick the config source.
type LoadOptions struct {
	// Inline is the config input (YAML text).
	Inline string
	// Extends is the extends input (owner/repo@tag).
	Extends string
	// ConfigName is the config-name input.
	ConfigName string
	// Workspace is the checked-out repository (GITHUB_WORKSPACE).
	Workspace string
	// ExcludeBots is the exclude-bots input. It only applies to the
	// built-in default.
	ExcludeBots bool
	// Fetch configures the extends download.
	Fetch ExtendsOptions
	Log   *zerolog.Logger
}

// Loaded is the parsed config and where it came from.
type Loaded struct {
	Config *Config
	Source Source
	// Warnings are for the step log as ::warning annotations.
	Warnings []string
}

// Load picks exactly one config source, the first that is set, and parses
// it. Nothing is merged:
//
//  1. the config input;
//  2. extends, a release asset at a pinned tag;
//  3. the local config-name file under .github/ in the checkout;
//  4. the built-in default.
//
// The built-in default is only used when config-name is left at its
// default. A config-name that was set but does not exist is an error, so a
// typo never silently drafts with the wrong config.
func Load(ctx context.Context, o LoadOptions) (*Loaded, error) {
	log := o.Log
	if log == nil {
		nop := zerolog.Nop()
		log = &nop
	}
	if o.Fetch.Log == nil {
		o.Fetch.Log = log
	}
	inline, extends := strings.TrimSpace(o.Inline), strings.TrimSpace(o.Extends)
	out := &Loaded{}
	var err error
	switch {
	case inline != "":
		out.Source = Source{Text: []byte(o.Inline), Origin: "input config", Kind: KindInput}
		if extends != "" {
			out.Warnings = append(out.Warnings, fmt.Sprintf("Both config and extends are set; using the config input and ignoring extends %s.", extends))
		}
	case extends != "":
		out.Source, err = FetchExtends(ctx, extends, o.Fetch)
	default:
		out.Source, err = ReadLocal(o.ConfigName, o.Workspace)
		if errors.Is(err, ErrNotFound) {
			rel, normErr := NormalizeConfigName(o.ConfigName)
			if normErr != nil {
				// ReadLocal normalizes the same name to reach ErrNotFound,
				// so this should not happen; fall back to the raw input
				// rather than reporting an empty config-name.
				rel = o.ConfigName
			}
			if rel != DefaultConfigName {
				// ReadLocal's error already starts with ErrNotFound's text,
				// so the missing file is spelled out here rather than
				// wrapping it and repeating "no config found".
				full := filepath.Join(o.Workspace, filepath.FromSlash(rel))
				return nil, fmt.Errorf("%w: checked the config input (empty), extends (empty) and config-name %s, but %s does not exist (did the workflow check out the repository?)", ErrNotFound, rel, full)
			}
			err = nil
			out.Source = Source{Text: DefaultText(), Origin: DefaultOrigin, Kind: KindDefault}
			if _, statErr := os.Stat(filepath.Join(o.Workspace, ".git")); statErr != nil {
				out.Warnings = append(out.Warnings, "No config input, extends or "+DefaultConfigName+", and the workspace is not a checkout; using the built-in default config. Add actions/checkout before this step to use the repository's own file.")
			}
		}
	}
	if err != nil {
		log.Error().Err(err).Msg("config source failed")
		return nil, err
	}
	log.Info().Str("kind", out.Source.Kind).Str("origin", out.Source.Origin).Int("bytes", len(out.Source.Text)).Msg("config source selected")

	if out.Source.Kind == KindDefault {
		out.Config, err = BuiltinDefault(o.ExcludeBots)
	} else {
		out.Config, err = Parse(out.Source.Text, out.Source.Origin)
		if o.ExcludeBots {
			out.Warnings = append(out.Warnings, fmt.Sprintf("exclude-bots only applies to the built-in default config; ignoring it for %s. Use exclude-contributors and a pre-exclude category in your config instead.", out.Source.Origin))
		}
	}
	if err != nil {
		return nil, err
	}
	// Counts only: the config text can hold private templates and is never
	// logged.
	log.Debug().
		Str("kind", out.Source.Kind).
		Str("origin", out.Source.Origin).
		Bool("exclude_bots", out.Config.ExcludeBots).
		Int("categories", len(out.Config.Categories)).
		Int("replacers", len(out.Config.Replacers)).
		Int("autolabeler", len(out.Config.Autolabeler)).
		Msg("config parsed")
	return out, nil
}
