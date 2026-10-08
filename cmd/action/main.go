// Command action is the entry point of the release-drafter-action Docker
// image.
//
// It reads the action inputs from flags, builds internal/action.Env from
// the process environment and hands both to internal/action.Run, which
// does the actual work: draft mode (push, workflow_dispatch and release)
// creates or updates the draft release, and autolabel mode (pull_request
// and pull_request_target) adds labels. Everything testable lives in
// internal/action and the packages it wires together; this file only
// touches the process (flags, environment, stdio) and os.Exit.
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
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"

	"github.com/Bugs5382/release-drafter-action/internal/action"
	"github.com/Bugs5382/release-drafter-action/internal/logging"
)

// Set at build time with -ldflags "-X main.Version=... -X main.Gitsha=...".
var (
	Version = "local"
	Gitsha  = "unknown"
)

func parseFlags(args []string, stderr io.Writer) (action.Flags, error) {
	var in action.Flags
	fs := flag.NewFlagSet("release-drafter-action", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&in.ConfigName, "config-name", "release-drafter.yml", "config file under .github on the checked-out ref")
	fs.StringVar(&in.Config, "config", "", "config as YAML text; takes precedence over config-name")
	fs.StringVar(&in.Extends, "extends", "", "shared config: a release asset at a pinned tag, owner/repo@tag")
	fs.StringVar(&in.ExtendsAsset, "extends-asset", "", "release asset name extends downloads")
	fs.StringVar(&in.ExcludeBots, "exclude-bots", "", "drop pull requests opened by a bot account (built-in default config only)")
	fs.StringVar(&in.Name, "name", "", "release name, overrides name-template")
	fs.StringVar(&in.Tag, "tag", "", "release tag, overrides tag-template")
	fs.StringVar(&in.Version, "version", "", "release version, overrides the resolved version")
	fs.StringVar(&in.Publish, "publish", "", "publish the release immediately")
	fs.StringVar(&in.Latest, "latest", "", "mark the release as latest")
	fs.StringVar(&in.Prerelease, "prerelease", "", "draft a prerelease")
	fs.StringVar(&in.PrereleaseIdentifier, "prerelease-identifier", "", "prerelease identifier (alpha, beta, rc)")
	fs.StringVar(&in.IncludePreReleases, "include-pre-releases", "", "include prereleases when finding the last release")
	fs.StringVar(&in.Commitish, "commitish", "", "release target branch, SHA or ref")
	fs.StringVar(&in.Header, "header", "", "text added before the body")
	fs.StringVar(&in.Footer, "footer", "", "text added after the body")
	fs.StringVar(&in.DryRun, "dry-run", "", "print the body and outputs, write nothing")
	fs.StringVar(&in.FilterByRange, "filter-by-range", "", "semver range the last release must satisfy")
	fs.StringVar(&in.Since, "since", "", "first-release start point (tag, SHA or date)")
	fs.StringVar(&in.FirstVersion, "first-version", "1.0.0", "resolved version for a first release, instead of incrementing from 0.0.0")
	err := fs.Parse(args)
	return in, err
}

// extendsCacheDir picks where extends caches downloaded config assets:
// RUNNER_TEMP inside Actions (cleaned up with the runner), the system temp
// directory otherwise (a local task run).
func extendsCacheDir(getenv func(string) string) string {
	dir := getenv("RUNNER_TEMP")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "release-drafter-action-extends")
}

func run(args []string, stdout, stderr io.Writer) int {
	flags, err := parseFlags(args, stderr)
	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	logger := logging.Init()
	logger.Info().Str("version", Version).Str("gitsha", Gitsha).Msg("release-drafter-action: starting")

	return action.Run(context.Background(), action.Options{
		Flags:           flags,
		Env:             action.LoadEnv(os.Getenv),
		Stdout:          stdout,
		Stderr:          stderr,
		ExtendsCacheDir: extendsCacheDir(os.Getenv),
	})
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
