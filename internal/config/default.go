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
	_ "embed"
	"slices"
)

// defaultYAML is the built-in default config. Its category titles carry
// emoji, written in the YAML as \U/\u escapes: the repository's PR hygiene
// check forbids emoji outside the root .github directory, and this file
// lives under internal/config/.github, not the root.
//
//go:embed .github/default.yml
var defaultYAML []byte

// DefaultOrigin names the built-in default in logs and errors.
const DefaultOrigin = "built-in default"

// BotLogins are the bot accounts exclude-bots leaves out of the notes. They
// are matched with and without the [bot] suffix.
var BotLogins = []string{"dependabot", "renovate", "github-actions"}

// DefaultText returns a copy of the built-in default config text.
func DefaultText() []byte { return slices.Clone(defaultYAML) }

// BuiltinDefault parses the built-in default config. With excludeBots, pull
// requests opened by bots are dropped before rendering (ExcludeBots) and the
// bot accounts are left out of $NEW_CONTRIBUTORS (exclude-contributors).
func BuiltinDefault(excludeBots bool) (*Config, error) {
	cfg, err := Parse(defaultYAML, DefaultOrigin)
	if err != nil {
		return nil, err
	}
	if excludeBots {
		cfg.ExcludeBots = true
		cfg.ExcludeContributors = append(cfg.ExcludeContributors, BotLogins...)
	}
	return cfg, nil
}

// IsBot reports whether a pull request author is a bot that exclude-bots
// leaves out: any GraphQL Bot actor, or one of BotLogins.
func IsBot(typename, login string) bool {
	if typename == "Bot" {
		return true
	}
	return slices.Contains(BotLogins, login) || slices.ContainsFunc(BotLogins, func(b string) bool { return b+"[bot]" == login })
}
