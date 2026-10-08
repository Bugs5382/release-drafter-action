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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// workspace makes a checkout-like directory. withFile writes
// .github/release-drafter.yml; withGit adds the .git directory a checkout has.
func workspace(t *testing.T, withFile, withGit bool) string {
	t.Helper()
	ws := t.TempDir()
	if withFile {
		if err := os.MkdirAll(filepath.Join(ws, ".github"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, ".github", "release-drafter.yml"), []byte("template: file\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if withGit {
		if err := os.Mkdir(filepath.Join(ws, ".git"), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	return ws
}

func TestLoadFirstSourceWins(t *testing.T) {
	f := newFakeGitHub(t)
	ctx := context.Background()
	ws := workspace(t, true, true)

	got, err := Load(ctx, LoadOptions{Inline: "template: inline\n", Extends: "cfg/shared@v1.0.0", ConfigName: "release-drafter.yml", Workspace: ws, Fetch: f.opts("")})
	if err != nil || got.Source.Kind != KindInput || got.Config.Template != "inline" {
		t.Fatalf("input: %+v, %v", got, err)
	}
	if len(f.requests) != 0 || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "ignoring extends cfg/shared@v1.0.0") {
		t.Errorf("config input must win without fetching: requests %q, warnings %q", f.requests, got.Warnings)
	}

	got, err = Load(ctx, LoadOptions{Extends: "cfg/shared@v1.0.0", ConfigName: "release-drafter.yml", Workspace: ws, Fetch: f.opts("")})
	if err != nil || got.Source.Kind != KindExtends || got.Config.Template != "shared $CHANGES" || len(got.Warnings) != 0 {
		t.Fatalf("extends: %+v, %v", got, err)
	}

	got, err = Load(ctx, LoadOptions{Inline: "  \n", ConfigName: "release-drafter.yml", Workspace: ws})
	if err != nil || got.Source.Kind != KindFile || got.Config.Template != "file" {
		t.Fatalf("file: %+v, %v", got, err)
	}

	got, err = Load(ctx, LoadOptions{ConfigName: "release-drafter.yml", Workspace: workspace(t, false, true)})
	if err != nil || got.Source.Kind != KindDefault || got.Source.Origin != DefaultOrigin || len(got.Config.Categories) != 9 || len(got.Warnings) != 0 {
		t.Fatalf("default: %+v, %v", got, err)
	}
}

func TestLoadDefaultWarnsWithoutCheckout(t *testing.T) {
	got, err := Load(context.Background(), LoadOptions{Workspace: workspace(t, false, false)})
	if err != nil || got.Source.Kind != KindDefault {
		t.Fatalf("%+v, %v", got, err)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "Add actions/checkout") {
		t.Errorf("warnings = %q", got.Warnings)
	}
}

func TestLoadExplicitConfigNameMustExist(t *testing.T) {
	_, err := Load(context.Background(), LoadOptions{ConfigName: "drafter.yml", Workspace: workspace(t, false, true)})
	if !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "config input (empty), extends (empty) and config-name .github/drafter.yml") {
		t.Errorf("err = %v", err)
	}
	msg := err.Error()
	if n := strings.Count(msg, "no config found"); n != 1 {
		t.Errorf("%q says \"no config found\" %d times; want once", msg, n)
	}
	if !strings.Contains(msg, filepath.Join(".github", "drafter.yml")+" does not exist") || !strings.Contains(msg, "check out") {
		t.Errorf("%q should name the missing file and the checkout hint", msg)
	}
}

func TestLoadLogsTheParsedConfigAtDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	inline := "categories:\n  - title: A\n  - title: B\n    labels: [b]\nreplacers:\n  - search: x\n    replace: y\nautolabeler:\n  - label: docs\n    files: ['*.md']\n"
	if _, err := Load(context.Background(), LoadOptions{Inline: inline, Log: &logger}); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		if entry["message"] != "config parsed" {
			continue
		}
		found = true
		if entry["level"] != "debug" || entry["kind"] != KindInput || entry["origin"] != "input config" ||
			entry["categories"] != float64(2) || entry["replacers"] != float64(1) || entry["autolabeler"] != float64(1) {
			t.Errorf("config parsed entry = %v", entry)
		}
	}
	if !found {
		t.Fatalf("no config parsed entry in %s", buf.String())
	}
	if strings.Contains(buf.String(), "title: A") || strings.Contains(buf.String(), "*.md") {
		t.Errorf("the config text leaked into the log: %s", buf.String())
	}
}

func TestLoadExcludeBots(t *testing.T) {
	got, err := Load(context.Background(), LoadOptions{ExcludeBots: true, Workspace: workspace(t, false, true)})
	if err != nil || !got.Config.ExcludeBots || len(got.Warnings) != 0 {
		t.Fatalf("default with exclude-bots: %+v, %v", got, err)
	}
	got, err = Load(context.Background(), LoadOptions{ExcludeBots: false, Workspace: workspace(t, false, true)})
	if err != nil || got.Config.ExcludeBots {
		t.Fatalf("default without exclude-bots: %+v, %v", got, err)
	}
	got, err = Load(context.Background(), LoadOptions{Inline: "template: x\n", ExcludeBots: true})
	if err != nil || got.Config.ExcludeBots || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "only applies to the built-in default") {
		t.Errorf("config input with exclude-bots: %+v, %v", got, err)
	}
}

func TestLoadExtendsAssetIsValidated(t *testing.T) {
	f := newFakeGitHub(t)
	f.asset = "_extends: org/.github\n"
	f.digest = "sha256:" + sha256Hex(f.asset)
	_, err := Load(context.Background(), LoadOptions{Extends: "cfg/shared@v1.0.0", Fetch: f.opts("")})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Origin != "extends cfg/shared@v1.0.0 (asset release-drafter.yml)" || !strings.Contains(err.Error(), "_extends (line 1): is not supported") {
		t.Errorf("err = %v", err)
	}
}
