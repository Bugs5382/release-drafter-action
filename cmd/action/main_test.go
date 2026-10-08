package main

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
	"path/filepath"
	"testing"
)

func TestParseFlagsAppliesTheConfigNameDefault(t *testing.T) {
	var stderr bytes.Buffer
	in, err := parseFlags(nil, &stderr)
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if in.ConfigName != "release-drafter.yml" {
		t.Errorf("ConfigName = %q, want the default", in.ConfigName)
	}
}

func TestParseFlagsAppliesTheFirstVersionDefault(t *testing.T) {
	var stderr bytes.Buffer
	in, err := parseFlags(nil, &stderr)
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if in.FirstVersion != "1.0.0" {
		t.Errorf("FirstVersion = %q, want the default 1.0.0", in.FirstVersion)
	}
}

func TestParseFlagsReadsEveryInput(t *testing.T) {
	var stderr bytes.Buffer
	in, err := parseFlags([]string{
		"--extends=acme/shared@v1.0.0", "--extends-asset=notes.yml", "--exclude-bots=true", "--dry-run=true",
	}, &stderr)
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if in.Extends != "acme/shared@v1.0.0" || in.ExtendsAsset != "notes.yml" || in.ExcludeBots != "true" || in.DryRun != "true" {
		t.Errorf("in = %+v", in)
	}
}

func TestParseFlagsHelpReturnsNoError(t *testing.T) {
	var stderr bytes.Buffer
	if _, err := parseFlags([]string{"--help"}, &stderr); err == nil {
		t.Fatal("flag.ErrHelp expected")
	}
}

func TestExtendsCacheDirPrefersRunnerTemp(t *testing.T) {
	getenv := func(key string) string {
		if key == "RUNNER_TEMP" {
			return "/tmp/runner"
		}
		return ""
	}
	want := filepath.Join("/tmp/runner", "release-drafter-action-extends")
	if got := extendsCacheDir(getenv); got != want {
		t.Errorf("extendsCacheDir = %q, want %q", got, want)
	}
}

func TestExtendsCacheDirFallsBackToTempDir(t *testing.T) {
	got := extendsCacheDir(func(string) string { return "" })
	if filepath.Base(got) != "release-drafter-action-extends" {
		t.Errorf("extendsCacheDir = %q, want it under a temp dir", got)
	}
}

func TestRunReturns2OnAnInvalidFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--unknown-flag"}, &stdout, &stderr); code != 2 {
		t.Errorf("run = %d, want 2", code)
	}
}

func TestRunReturns0OnHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Errorf("run = %d, want 0", code)
	}
}
