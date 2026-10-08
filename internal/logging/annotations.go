package logging

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
	"io"
	"os"
	"strings"
)

// Annotator writes workflow commands. GitHub reads them from stdout.
type Annotator struct{ W io.Writer }

// Stdout is the annotator the action uses.
var Stdout = Annotator{W: os.Stdout}

// escapeData matches @actions/core's escapeData.
func escapeData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

// escapeProperty matches @actions/core's escapeProperty.
func escapeProperty(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(s)
}

// Error writes an ::error annotation with a title. The step still has to
// exit non-zero to fail.
func (a Annotator) Error(title, msg string) {
	_, _ = fmt.Fprintf(a.W, "::error title=%s::%s\n", escapeProperty(title), escapeData(msg))
}

// Warning writes a ::warning annotation, the way v7's core.warning does.
func (a Annotator) Warning(msg string) {
	_, _ = fmt.Fprintf(a.W, "::warning::%s\n", escapeData(msg))
}
