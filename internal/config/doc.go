// Package config loads and validates the release-drafter configuration.
//
// Exactly one source is used. The first one that is set wins, and nothing is
// merged between them:
//
//  1. the config input (YAML text);
//  2. extends: a release asset at a pinned tag of another repository,
//     checked against the sha256 digest GitHub reports and cached per tag;
//  3. the local .github/<config-name> file from the checkout (.yml, .yaml or
//     .json);
//  4. the built-in default.
//
// The built-in default applies only when config-name is left at its default.
// A config-name that was set but does not exist is an error that lists the
// sources checked, so a typo never drafts with the wrong config.
//
// Every release-drafter v7 key is validated. An unknown key is an error naming
// its path, and _extends inside a config is rejected in favour of the extends
// input. The v7 defaults are applied, and the action inputs override the config
// the way v7 does (commitish, header, footer, prerelease and the rest).
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
