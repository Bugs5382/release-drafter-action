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
	"regexp"

	"github.com/Bugs5382/release-drafter-action/internal/config"
	"github.com/Bugs5382/release-drafter-action/internal/logging"
)

// Vars maps "$NAME" to its value. A missing key leaves the variable in the
// text untouched, exactly like a value v7 holds as undefined or null.
type Vars map[string]string

var varRe = regexp.MustCompile(`\$[A-Z_]+`)

// Render replaces every $UPPER_CASE token with its value. It makes one pass,
// so values are never re-scanned.
func Render(template string, vars Vars) string {
	return varRe.ReplaceAllStringFunc(template, func(tok string) string {
		if v, ok := vars[tok]; ok {
			return v
		}
		return tok
	})
}

// ApplyReplacers runs every replacer over input, in order. Neither input nor
// the replaced text is ever logged: only the replacer count, index and
// search pattern, and whether a given replacer changed the text.
func ApplyReplacers(input string, replacers []config.CompiledReplacer) (string, error) {
	logging.L().Debug().Int("replacers", len(replacers)).Msg("render: applying replacers")
	for i, r := range replacers {
		pattern := ParseReplacePattern(r.Replace)
		out, err := r.Search.Replace(input, pattern.Build)
		if err != nil {
			logging.L().Error().Int("index", i).Str("pattern", r.Search.Raw).Err(err).
				Msg("render: replacer failed")
			return "", err
		}
		logging.L().Trace().Int("index", i).Str("pattern", r.Search.Raw).Bool("changed", out != input).
			Msg("render: replacer applied")
		input = out
	}
	logging.L().Debug().Msg("render: replacers applied")
	return input, nil
}
