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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/Bugs5382/release-drafter-action/internal/model"
	"github.com/Bugs5382/release-drafter-action/internal/render"
)

// writeOutput appends one $GITHUB_OUTPUT entry with a random delimiter, the
// way actions/toolkit's core.setOutput writes a multi-line value (the
// rendered body can hold anything, including a line that looks like a
// delimiter) since GITHUB_OUTPUT replaced the deprecated ::set-output
// command.
func writeOutput(w io.Writer, name, value string) error {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Errorf("action: writing output %s: %w", name, err)
	}
	delim := "ghadelimiter_" + hex.EncodeToString(buf[:])
	_, err := fmt.Fprintf(w, "%s<<%s\n%s\n%s\n", name, delim, value, delim)
	return err
}

// writeOutputs appends every pair, in order, to the file at path. path ==
// "" (no GITHUB_OUTPUT, e.g. a local task run) is a no-op, not an error.
func writeOutputs(path string, pairs [][2]string) error {
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- path is GITHUB_OUTPUT, set by the Actions runner itself
	if err != nil {
		return fmt.Errorf("action: opening %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	for _, p := range pairs {
		if err := writeOutput(f, p[0], p[1]); err != nil {
			return err
		}
	}
	return nil
}

// draftOutputs builds the draft mode outputs in action.yml's order. The
// version outputs always come from the rendered payload. The release
// outputs (id, name, tag_name, body, html_url, upload_url) come from the
// created or updated release when there is one, falling back to the
// rendered payload for name, tag_name and body so a dry run or a skipped
// empty draft still reports what would have been set; id, html_url and
// upload_url only ever exist on a real release, so they stay empty then.
func draftOutputs(payload render.Payload, result *model.Release) [][2]string {
	id, name, tag, body, htmlURL, uploadURL := "", payload.Name, payload.Tag, payload.Body, "", ""
	if result != nil {
		id = fmt.Sprintf("%d", result.ID)
		if result.Name != "" {
			name = result.Name
		}
		if result.TagName != "" {
			tag = result.TagName
		}
		htmlURL = result.HTMLURL
		uploadURL = result.UploadURL
	}
	return [][2]string{
		{"id", id},
		{"name", name},
		{"tag_name", tag},
		{"body", body},
		{"html_url", htmlURL},
		{"upload_url", uploadURL},
		{"resolved_version", payload.ResolvedVersion},
		{"major_version", payload.MajorVersion},
		{"minor_version", payload.MinorVersion},
		{"patch_version", payload.PatchVersion},
	}
}
