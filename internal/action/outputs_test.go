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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bugs5382/release-drafter-action/internal/model"
	"github.com/Bugs5382/release-drafter-action/internal/render"
)

func TestWriteOutputsNoOpWithoutAPath(t *testing.T) {
	if err := writeOutputs("", [][2]string{{"a", "b"}}); err != nil {
		t.Fatalf("writeOutputs with no path: %v", err)
	}
}

func TestWriteOutputsHandlesMultilineValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output")
	body := "line one\nline two\n"
	if err := writeOutputs(path, [][2]string{{"body", body}, {"name", "v1.0.0"}}); err != nil {
		t.Fatalf("writeOutputs: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, "body<<ghadelimiter_") {
		t.Errorf("output missing the body delimiter line: %s", out)
	}
	if !strings.Contains(out, body) {
		t.Errorf("output missing the multiline body: %s", out)
	}
	if !strings.Contains(out, "name<<ghadelimiter_") || !strings.Contains(out, "v1.0.0") {
		t.Errorf("output missing name: %s", out)
	}
}

func TestDraftOutputsFallsBackToThePayloadWhenThereIsNoResult(t *testing.T) {
	payload := render.Payload{Name: "v1.0.0", Tag: "v1.0.0", Body: "notes", ResolvedVersion: "1.0.0"}
	pairs := draftOutputs(payload, nil)
	got := map[string]string{}
	for _, p := range pairs {
		got[p[0]] = p[1]
	}
	if got["name"] != "v1.0.0" || got["tag_name"] != "v1.0.0" || got["body"] != "notes" {
		t.Errorf("pairs = %+v", got)
	}
	if got["id"] != "" || got["html_url"] != "" || got["upload_url"] != "" {
		t.Errorf("pairs should have no release-only fields without a result: %+v", got)
	}
}

func TestDraftOutputsPrefersTheResultWhenThereIsOne(t *testing.T) {
	payload := render.Payload{Name: "v1.0.0", Tag: "v1.0.0", Body: "notes", ResolvedVersion: "1.0.0"}
	result := &model.Release{ID: 9, Name: "v1.0.0", TagName: "v1.0.0", HTMLURL: "https://example/release", UploadURL: "https://example/upload"}
	pairs := draftOutputs(payload, result)
	got := map[string]string{}
	for _, p := range pairs {
		got[p[0]] = p[1]
	}
	if got["id"] != "9" || got["html_url"] != "https://example/release" || got["upload_url"] != "https://example/upload" {
		t.Errorf("pairs = %+v", got)
	}
}
