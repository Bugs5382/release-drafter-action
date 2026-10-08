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
	"bytes"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDefaults(t *testing.T) {
	cases := []struct {
		name          string
		env           map[string]string
		level, format string
	}{
		{"local run", map[string]string{}, "trace", "console"},
		{"actions run", map[string]string{"GITHUB_ACTIONS": "true"}, "info", "console"},
		{"actions debug re-run", map[string]string{"GITHUB_ACTIONS": "true", "RUNNER_DEBUG": "1"}, "debug", "console"},
		{"explicit values win", map[string]string{"GITHUB_ACTIONS": "true", "LOG_LEVEL": "error", "LOG_FORMAT": "json"}, "error", "json"},
	}
	for _, c := range cases {
		level, format := Defaults(env(c.env))
		if level != c.level || format != c.format {
			t.Errorf("%s: got %s/%s, want %s/%s", c.name, level, format, c.level, c.format)
		}
	}
}

func TestAnnotationsEscape(t *testing.T) {
	var b bytes.Buffer
	a := Annotator{W: &b}
	a.Error("Invalid config: categories", "line one\nline two 100%")
	a.Warning("a, b: c\r\n")
	want := "::error title=Invalid config%3A categories::line one%0Aline two 100%25\n" +
		"::warning::a, b: c%0D%0A\n"
	if b.String() != want {
		t.Errorf("got %q\nwant %q", b.String(), want)
	}
}
